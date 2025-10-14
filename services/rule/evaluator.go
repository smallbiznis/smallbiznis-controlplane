package rule

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

// Evaluator evaluates CEL expressions using a dynamic set of variables.
type Evaluator struct{}

// NewEvaluator creates a new Evaluator instance.
func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

// Evaluate evaluates a CEL expression against the provided context map.
// The context map entries are exposed as top-level variables in the CEL program.
func (e *Evaluator) Evaluate(expression string, context map[string]any) (bool, error) {
	if expression == "" {
		return false, fmt.Errorf("expression must not be empty")
	}

	if context == nil {
		context = map[string]any{}
	}

	declarations := make([]*exprpb.Decl, 0, len(context))
	for key := range context {
		declarations = append(declarations, decls.NewVar(key, decls.Dyn))
	}

	envOpts := []cel.EnvOption{cel.Declarations(declarations...)}
	env, err := cel.NewEnv(envOpts...)
	if err != nil {
		return false, fmt.Errorf("failed to create CEL env: %w", err)
	}

	ast, issues := env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return false, fmt.Errorf("failed to compile expression: %w", issues.Err())
	}

	program, err := env.Program(ast)
	if err != nil {
		return false, fmt.Errorf("failed to create CEL program: %w", err)
	}

	result, _, err := program.Eval(context)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate expression: %w", err)
	}

	boolean, ok := result.Value().(bool)
	if !ok {
		return false, fmt.Errorf("expression must return a boolean, got %T", result.Value())
	}

	return boolean, nil
}

// ExampleEvaluate demonstrates how to evaluate a rule expression.
func ExampleEvaluate() {
	evaluator := NewEvaluator()
	rule := Rule{Expression: "total_spent > 100000 && user_tier == 'gold'"}
	context := map[string]any{"total_spent": 120000, "user_tier": "gold"}
	result, _ := evaluator.Evaluate(rule.Expression, context)
	fmt.Println(result)
	// Output: true
}
