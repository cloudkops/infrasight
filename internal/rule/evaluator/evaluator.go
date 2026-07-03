package evaluator

import (
	"strings"

	"github.com/cloudkops/infrasight/internal/resource"
	"github.com/cloudkops/infrasight/internal/rule/parser"
)

// Match evaluates one condition against a resource/container pair. It resolves the
// dotted field path to a value (typed fast path first, then Resource/Container
// Attributes as a fallback for provider-specific fields), determines which
// operator the condition expresses, and delegates the comparison to the Operator
// Registry.
func Match(r resource.Resource, c resource.Container, cond parser.Condition) bool {
	actual, ok := resolveField(r, c, cond.Field)

	opName := operatorNameFor(cond)
	op, found := Get(opName)
	if !found {
		return false
	}

	// A field that fails to resolve is "not present" — which is exactly what the
	// exists operator needs to check for absence. Every other operator requires a
	// resolved value to compare against.
	if !ok && opName != "exists" {
		return false
	}
	return op.Match(actual, cond)
}

// conditionOperators lists every operator key a Condition can set, in the
// precedence order used when more than one is populated on the same condition
// (only one operator ever runs — see docs/rulesets_manual.md's pitfalls section).
// Adding an operator means appending one entry here and to registry.go's init() —
// operatorNameFor itself never grows a new branch.
var conditionOperators = []struct {
	name string
	set  func(parser.Condition) bool
}{
	{"not_equals", func(c parser.Condition) bool { return c.NotEquals != nil }},
	{"not_in", func(c parser.Condition) bool { return c.NotIn != nil }},
	{"not_contains", func(c parser.Condition) bool { return c.NotContains != "" }},
	{"in", func(c parser.Condition) bool { return c.In != nil }},
	{"contains", func(c parser.Condition) bool { return c.Contains != "" }},
	{"starts_with", func(c parser.Condition) bool { return c.StartsWith != "" }},
	{"ends_with", func(c parser.Condition) bool { return c.EndsWith != "" }},
	{"regex", func(c parser.Condition) bool { return c.Regex != "" }},
	{"greater_than_or_equal", func(c parser.Condition) bool { return c.GreaterThanOrEqual != nil }},
	{"less_than_or_equal", func(c parser.Condition) bool { return c.LessThanOrEqual != nil }},
	{"greater_than", func(c parser.Condition) bool { return c.GreaterThan != nil }},
	{"less_than", func(c parser.Condition) bool { return c.LessThan != nil }},
	{"equals", func(c parser.Condition) bool { return c.Equals != nil }},
}

// operatorNameFor infers which operator a Condition expresses from whichever field
// is populated — this keeps the YAML schema's ergonomic shorthand (equals/
// not_equals/contains/exists/... as direct keys) while still resolving through the
// Operator Registry rather than a hardcoded switch on comparison logic. A
// Condition with none of conditionOperators set is an exists check.
func operatorNameFor(cond parser.Condition) string {
	for _, o := range conditionOperators {
		if o.set(cond) {
			return o.name
		}
	}
	return "exists"
}

// resolveField resolves a dotted-path field against a Resource/Container pair.
// Only the simple, universally-needed identity/resource fields get a typed fast
// path here. Every Security/Networking signal — however many a provider ends up
// collecting — is resolved through the Attributes fallback instead, populated by
// internal/scan/pipeline's EnrichAttributes stage from SecurityContext.Flatten /
// Networking.Flatten. That is the one place to touch when a new security or
// networking signal is added; this function does not need a new case for it.
func resolveField(r resource.Resource, c resource.Container, field string) (any, bool) {
	switch {
	case field == "resource.name":
		return r.Name, true
	case field == "resource.namespace":
		return r.Namespace, true
	case field == "resource.kind":
		return r.Kind, true
	case field == "resource.provider":
		return r.Provider, true
	case strings.HasPrefix(field, "resource.labels."):
		key := strings.TrimPrefix(field, "resource.labels.")
		v, ok := r.Metadata.Labels[key]
		return v, ok
	case strings.HasPrefix(field, "resource.annotations."):
		key := strings.TrimPrefix(field, "resource.annotations.")
		v, ok := r.Metadata.Annotations[key]
		return v, ok
	case field == "container.name":
		return c.Name, true
	case field == "container.image":
		return c.Image, true
	case field == "container.user":
		return c.User, true
	case field == "container.privileged":
		return c.Privileged, true
	case field == "container.resources.cpu_limit":
		return c.Limits.CPULimit, c.Limits.CPULimit != ""
	case field == "container.resources.cpu_request":
		return c.Limits.CPURequest, c.Limits.CPURequest != ""
	case field == "container.resources.memory_limit":
		return c.Limits.MemoryLimit, c.Limits.MemoryLimit != ""
	case field == "container.resources.memory_request":
		return c.Limits.MemoryRequest, c.Limits.MemoryRequest != ""
	default:
		if v, ok := r.Attributes[field]; ok {
			return v, true
		}
		if v, ok := c.Attributes[field]; ok {
			return v, true
		}
		return nil, false
	}
}
