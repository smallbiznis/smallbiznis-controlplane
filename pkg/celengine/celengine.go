package celengine

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/google/cel-go/cel"
)

func BuildCelEnvFromAttributes(attrs map[string]interface{}) (*cel.Env, error) {
	var variables []cel.EnvOption

	for key, val := range attrs {

		switch v := val.(type) {
		case string:
			variables = append(variables, cel.Variable(key, cel.StringType))

		case int, int64, float64, float32:
			variables = append(variables, cel.Variable(key, cel.IntType))

		case bool:
			variables = append(variables, cel.Variable(key, cel.BoolType))

		case []interface{}:
			// Try to inspect the first item
			if len(v) > 0 {
				elem := v[0]
				elemType := reflect.TypeOf(elem)
				switch elem.(type) {
				case map[string]interface{}:
					// List of maps, e.g. items
					variables = append(variables, cel.Variable(key, cel.ListType(cel.MapType(cel.StringType, cel.DynType))))
				default:
					// Generic list
					fmt.Printf("Generic list with element type: %v\n", elemType)
					variables = append(variables, cel.Variable(key, cel.ListType(cel.DynType)))
				}
			} else {
				variables = append(variables, cel.Variable(key, cel.ListType(cel.DynType)))
			}

		case []map[string]interface{}:
			variables = append(variables, cel.Variable(key, cel.ListType(cel.MapType(cel.StringType, cel.DynType))))

		case map[string]interface{}:
			// Generic map
			variables = append(variables, cel.Variable(key, cel.MapType(cel.StringType, cel.DynType)))

		default:
			fmt.Printf("Unhandled type for key: %s -> %T\n", key, val)
			variables = append(variables, cel.Variable(key, cel.DynType))
		}
	}

	env, err := cel.NewEnv(variables...)
	if err != nil {
		return nil, err
	}

	return env, nil
}

func StructToMap(s *interface{}) map[string]interface{} {
	if s == nil {
		return map[string]interface{}{}
	}

	b, err := json.Marshal(s)
	if err != nil {
		return map[string]interface{}{}
	}

	var result map[string]interface{}
	if err := json.Unmarshal(b, &result); err != nil {
		return map[string]interface{}{}
	}

	return result
}

func ValidateExpression(env *cel.Env, expr string) error {
	_, issues := env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return issues.Err()
	}
	return nil
}

func Evaluate(env *cel.Env, expr string, attrs map[string]interface{}) (bool, error) {
	ast, issues := env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return false, issues.Err()
	}

	prg, err := env.Program(ast)
	if err != nil {
		return false, err
	}

	out, _, err := prg.Eval(attrs)
	if err != nil {
		return false, err
	}

	val := out.Value()

	b, ok := val.(bool)
	if !ok {
		return false, fmt.Errorf("expected bool from expression, got %T (%v)", val, val)
	}

	return b, nil
}
