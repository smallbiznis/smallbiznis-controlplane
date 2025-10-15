package rule

import (
	"fmt"
	"math"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

// Codex directive:
// Provide DSL evaluator using github.com/google/cel-go/cel.
// Implement Evaluate(expression string, ctx map[string]interface{}) (bool, error)
// Return true if expression evaluates to true.
// Used by RuleService for condition evaluation.

// Evaluator compiles and evaluates CEL expressions for rule matching.
type Evaluator struct{}

// NewEvaluator constructs a new Evaluator instance.
func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

// Evaluate executes the provided CEL expression against the supplied context map.
// The expression must resolve to a boolean value.
func (Evaluator) Evaluate(expression string, ctx map[string]any) (bool, error) {
	if expression == "" {
		return false, fmt.Errorf("expression must not be empty")
	}

	env, err := cel.NewEnv(cel.Declarations(declarationsFromContext(ctx)...))
	if err != nil {
		return false, fmt.Errorf("build cel env: %w", err)
	}

	ast, issues := env.Parse(expression)
	if issues != nil && issues.Err() != nil {
		return false, fmt.Errorf("parse cel expression: %w", issues.Err())
	}

	checked, issues := env.Check(ast)
	if issues != nil && issues.Err() != nil {
		return false, fmt.Errorf("check cel expression: %w", issues.Err())
	}

	prog, err := env.Program(checked)
	if err != nil {
		return false, fmt.Errorf("build cel program: %w", err)
	}

	result, _, err := prog.Eval(ctx)
	if err != nil {
		return false, fmt.Errorf("evaluate cel expression: %w", err)
	}

	boolVal, ok := result.Value().(bool)
	if !ok {
		return false, fmt.Errorf("cel expression did not evaluate to boolean")
	}

	return boolVal, nil
}

func declarationsFromContext(ctx map[string]any) []*exprpb.Decl {
	if len(ctx) == 0 {
		return nil
	}

	declsOut := make([]*exprpb.Decl, 0, len(ctx))
	for key, value := range ctx {
		declsOut = append(declsOut, decls.NewVar(key, typeForValue(value)))
	}
	return declsOut
}

func typeForValue(v any) *exprpb.Type {
	switch t := v.(type) {
	case bool:
		return decls.Bool
	case int, int32, int64:
		return decls.Int
	case uint, uint32, uint64:
		return decls.Uint
	case float32, float64:
		return decls.Double
	case string:
		return decls.String
	case map[string]any:
		return decls.NewMapType(decls.String, decls.Dyn)
	case []any:
		return decls.NewListType(decls.Dyn)
	case nil:
		return decls.Null
	default:
		// CEL uses double precision floats; promote NaN types accordingly.
		if f, ok := convertNumeric(t); ok {
			if math.IsNaN(f) {
				return decls.Double
			}
		}
		return decls.Dyn
	}
}

func convertNumeric(v any) (float64, bool) {
	switch n := v.(type) {
	case float32:
		return float64(n), true
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	default:
		return 0, false
	}
}
