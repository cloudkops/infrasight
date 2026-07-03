package evaluator

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

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
	if s, ok := asString(actual); ok {
		return strings.Contains(s, cond.Contains)
	}
	// A []string field (e.g. container.security.capabilities_add) makes
	// "contains" a membership check instead of a substring check — this is what
	// lets a rule ask "was this capability added" without a dedicated operator.
	if items, ok := asStringSlice(actual); ok {
		return slices.Contains(items, cond.Contains)
	}
	return false
}

type existsOperator struct{}

func (existsOperator) Name() string { return "exists" }
func (existsOperator) Match(actual any, cond parser.Condition) bool {
	return isPresent(actual) == cond.Exists
}

type notContainsOperator struct{}

func (notContainsOperator) Name() string { return "not_contains" }
func (notContainsOperator) Match(actual any, cond parser.Condition) bool {
	s, ok := asString(actual)
	return ok && !strings.Contains(s, cond.NotContains)
}

type startsWithOperator struct{}

func (startsWithOperator) Name() string { return "starts_with" }
func (startsWithOperator) Match(actual any, cond parser.Condition) bool {
	s, ok := asString(actual)
	return ok && strings.HasPrefix(s, cond.StartsWith)
}

type endsWithOperator struct{}

func (endsWithOperator) Name() string { return "ends_with" }
func (endsWithOperator) Match(actual any, cond parser.Condition) bool {
	s, ok := asString(actual)
	return ok && strings.HasSuffix(s, cond.EndsWith)
}

type regexOperator struct{}

func (regexOperator) Name() string { return "regex" }
func (regexOperator) Match(actual any, cond parser.Condition) bool {
	s, ok := asString(actual)
	if !ok {
		return false
	}
	re, err := compileRegex(cond.Regex)
	if err != nil {
		return false
	}
	return re.MatchString(s)
}

// regexCache avoids recompiling the same pattern on every (resource x container x
// rule) evaluation — ValidateRule already rejects malformed patterns at load time,
// so a compile failure here in practice only means an empty/zero-value Condition.
var (
	regexCacheMu sync.RWMutex
	regexCache   = map[string]*regexp.Regexp{}
)

func compileRegex(pattern string) (*regexp.Regexp, error) {
	regexCacheMu.RLock()
	re, ok := regexCache[pattern]
	regexCacheMu.RUnlock()
	if ok {
		return re, nil
	}

	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	regexCacheMu.Lock()
	regexCache[pattern] = compiled
	regexCacheMu.Unlock()
	return compiled, nil
}

type inOperator struct{}

func (inOperator) Name() string { return "in" }
func (inOperator) Match(actual any, cond parser.Condition) bool {
	got, ok := asComparable(actual)
	if !ok {
		return false
	}
	for _, v := range cond.In {
		if want, ok := asComparable(v); ok && want == got {
			return true
		}
	}
	return false
}

type notInOperator struct{}

func (notInOperator) Name() string { return "not_in" }
func (notInOperator) Match(actual any, cond parser.Condition) bool {
	got, ok := asComparable(actual)
	if !ok {
		return false
	}
	for _, v := range cond.NotIn {
		if want, ok := asComparable(v); ok && want == got {
			return false
		}
	}
	return true
}

type greaterThanOperator struct{}

func (greaterThanOperator) Name() string { return "greater_than" }
func (greaterThanOperator) Match(actual any, cond parser.Condition) bool {
	a, aok := asFloat64(actual)
	e, eok := asFloat64(cond.GreaterThan)
	return aok && eok && a > e
}

type lessThanOperator struct{}

func (lessThanOperator) Name() string { return "less_than" }
func (lessThanOperator) Match(actual any, cond parser.Condition) bool {
	a, aok := asFloat64(actual)
	e, eok := asFloat64(cond.LessThan)
	return aok && eok && a < e
}

type greaterThanOrEqualOperator struct{}

func (greaterThanOrEqualOperator) Name() string { return "greater_than_or_equal" }
func (greaterThanOrEqualOperator) Match(actual any, cond parser.Condition) bool {
	a, aok := asFloat64(actual)
	e, eok := asFloat64(cond.GreaterThanOrEqual)
	return aok && eok && a >= e
}

type lessThanOrEqualOperator struct{}

func (lessThanOrEqualOperator) Name() string { return "less_than_or_equal" }
func (lessThanOrEqualOperator) Match(actual any, cond parser.Condition) bool {
	a, aok := asFloat64(actual)
	e, eok := asFloat64(cond.LessThanOrEqual)
	return aok && eok && a <= e
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

// asStringSlice normalizes []string (the typed representation, e.g.
// SecurityContext.Flatten's capabilities_add/drop) and []interface{} (what YAML
// list values decode as) into a plain []string for membership checks.
func asStringSlice(v any) ([]string, bool) {
	switch t := v.(type) {
	case []string:
		return t, true
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out, true
	default:
		return nil, false
	}
}

// asFloat64 normalizes ints/floats, and numeric-looking strings, into a plain
// float64 for the four numeric comparison operators. It deliberately does NOT
// understand Kubernetes resource.Quantity suffixes ("500m" CPU, "2Gi" memory) —
// container.resources.cpu_limit/memory_limit are quantity strings, and comparing
// them numerically requires a provider-aware unit conversion this package (generic
// across every provider) does not do. A quantity string like "500m" fails to
// parse here and the operator simply returns false, same as any other type
// mismatch — see docs/rulesets_manual.md for the workaround.
func asFloat64(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case string:
		f, err := strconv.ParseFloat(t, 64)
		return f, err == nil
	default:
		return 0, false
	}
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
