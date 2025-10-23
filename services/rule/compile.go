package rule

import (
	"fmt"

	"github.com/google/cel-go/cel"
)

type CompiledRule struct {
	ID      string
	Rule    Rule
	Program cel.Program
}

func (r *CompiledRule) evaluate(ctx map[string]interface{}) (bool, error) {
	if r.Program == nil {
		return false, fmt.Errorf("compiled program is nil for rule %s", r.ID)
	}

	// Eval di cel-go mengembalikan tiga nilai: Value, Details, Error
	val, _, err := r.Program.Eval(ctx)
	if err != nil {
		return false, fmt.Errorf("eval failed for rule %s: %w", r.ID, err)
	}

	// hasil expression di CEL adalah bool
	matched, ok := val.Value().(bool)
	if !ok {
		return false, fmt.Errorf("rule %s did not return boolean", r.ID)
	}

	return matched, nil
}
