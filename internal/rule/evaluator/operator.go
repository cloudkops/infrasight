package evaluator

import (
	"fmt"
	"strings"

	"github.com/cloudkops/infrasight/internal/rule/parser"
)

// Operator matches a resolved field value against a Condition's expected value.
// Adding an operator means implementing this interface and registering it (see
// registry.go) — Evaluate never grows a new switch case.
type Operator interface {
	Name() string
	Match(actual any, cond parser.Condition) bool
}

type equalsOperator struct{}

func (equalsOperator) Name() string { return "equals" }
func (equalsOperator) Match(actual any, cond parser.Condition) bool {
	expected, ok := asComparable(cond.Equals)
	if !ok {
		return false
	}
	got, ok := asComparable(actual)
	return ok && got == expected
}

type notEqualsOperator struct{}

func (notEqualsOperator) Name() string { return "not_equals" }
func (notEqualsOperator) Match(actual any, cond parser.Condition) bool {
	expected, ok := asComparable(cond.NotEquals)
	if !ok {
		return false
	}
	got, ok := asComparable(actual)
	return ok && got != expected
}

type containsOperator struct{}

func (containsOperator) Name() string { return "contains" }
func (containsOperator) Match(actual any, cond parser.Condition) bool {
	s, ok := asString(actual)
	return ok && strings.Contains(s, cond.Contains)
}

type existsOperator struct{}

func (existsOperator) Name() string { return "exists" }
func (existsOperator) Match(actual any, cond parser.Condition) bool {
	return isPresent(actual) == cond.Exists
}

// asComparable normalizes ints/floats/strings/bools into a single comparable
// representation so equals/not_equals work across the YAML-decoded interface{}
// types and the typed Go values pulled from resource.Resource.
func asComparable(v any) (string, bool) {
	if v == nil {
		return "", false
	}
	switch t := v.(type) {
	case string:
		return t, true
	case bool:
		return fmt.Sprintf("%v", t), true
	case int:
		return fmt.Sprintf("%d", t), true
	case int64:
		return fmt.Sprintf("%d", t), true
	case float64:
		return fmt.Sprintf("%v", t), true
	default:
		return "", false
	}
}

func asString(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok
}

// isPresent reports whether a resolved field value counts as "set" for the exists
// operator: non-empty strings, non-zero numbers, true bools, and non-nil in general.
func isPresent(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case string:
		return t != ""
	case bool:
		return t
	case int:
		return t != 0
	case int64:
		return t != 0
	default:
		return true
	}
}
