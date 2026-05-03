package status

import "fmt"

type RuleContext map[string]any

func GetParam[T any](ctx RuleContext, key string) (T, error) {
	val, ok := ctx[key]
	if !ok {
		var zero T
		return zero, fmt.Errorf("param %q not found in context", key)
	}
	typed, ok := val.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("param %q has wrong type", key)
	}
	return typed, nil
}
