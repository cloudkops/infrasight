package evaluator

import (
	"testing"

	"github.com/cloudkops/infrasight/internal/resource"
	"github.com/cloudkops/infrasight/internal/rule/parser"
)

func TestMatch_TypedFields(t *testing.T) {
	r := resource.Resource{
		Name:      "my-pod",
		Namespace: "prod",
		Metadata:  resource.Metadata{Labels: map[string]string{"team": "payments"}},
	}
	c := resource.Container{
		Name:       "app",
		User:       0,
		Privileged: true,
		Limits:     resource.Limits{CPULimit: "500m"},
	}

	cases := []struct {
		name string
		cond parser.Condition
		want bool
	}{
		{"root user", parser.Condition{Field: "container.user", Equals: 0}, true},
		{"not root", parser.Condition{Field: "container.user", NotEquals: 0}, false},
		{"privileged", parser.Condition{Field: "container.privileged", Equals: true}, true},
		{"label present", parser.Condition{Field: "resource.labels.team", Equals: "payments"}, true},
		{"label missing exists=false", parser.Condition{Field: "resource.labels.env", Exists: false}, true},
		{"cpu limit exists", parser.Condition{Field: "container.resources.cpu_limit", Exists: true}, true},
		{"memory limit missing", parser.Condition{Field: "container.resources.memory_limit", Exists: false}, true},
		{"namespace contains", parser.Condition{Field: "resource.namespace", Contains: "pro"}, true},
		{"unknown field", parser.Condition{Field: "does.not.exist", Equals: "x"}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Match(r, c, tc.cond); got != tc.want {
				t.Errorf("Match(%+v) = %v, want %v", tc.cond, got, tc.want)
			}
		})
	}
}

// TestMatch_SecurityAndNetworkingViaAttributes documents that Security/Networking
// fields are NOT typed cases in resolveField — they only work because something
// upstream (internal/scan/pipeline.EnrichAttributes in production; this test,
// standing in for it) has already flattened them into Attributes. This is the
// fix for the gap where adding strict-runtime's rules required editing this
// package: a new SecurityContext/Networking field now only requires updating
// SecurityContext.Flatten/Networking.Flatten (internal/resource), never
// resolveField.
func TestMatch_SecurityAndNetworkingViaAttributes(t *testing.T) {
	trueVal := true
	r := resource.Resource{
		Security:   resource.SecurityContext{RunAsRoot: true},
		Networking: resource.Networking{Exposures: []resource.Exposure{{Type: "hostNetwork"}}},
	}
	r.Attributes = r.Security.Flatten("resource.security")
	for k, v := range r.Networking.Flatten("networking") {
		r.Attributes[k] = v
	}

	c := resource.Container{Security: resource.SecurityContext{AllowPrivilegeEscalation: &trueVal}}
	c.Attributes = c.Security.Flatten("container.security")

	cases := []struct {
		name string
		cond parser.Condition
		want bool
	}{
		{"resource run_as_root", parser.Condition{Field: "resource.security.run_as_root", Equals: true}, true},
		{"networking hostNetwork", parser.Condition{Field: "networking.hostNetwork", Equals: true}, true},
		{"networking publicIP absent but explicit false", parser.Condition{Field: "networking.publicIP", Equals: false}, true},
		{"container allow_privilege_escalation", parser.Condition{Field: "container.security.allow_privilege_escalation", Equals: true}, true},
		{"container read_only_root_filesystem unset is not present", parser.Condition{Field: "container.security.read_only_root_filesystem", Exists: false}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Match(r, c, tc.cond); got != tc.want {
				t.Errorf("Match(%+v) = %v, want %v", tc.cond, got, tc.want)
			}
		})
	}
}

func TestMatch_NewOperators(t *testing.T) {
	r := resource.Resource{
		Name: "web-7f8b9-xyz",
		Attributes: map[string]any{
			"replica_count": 5,
			"cpu_cores":     "0.5",
		},
	}
	c := resource.Container{Name: "app", Image: "docker.io/library/nginx:1.25"}

	cases := []struct {
		name string
		cond parser.Condition
		want bool
	}{
		{"not_contains matches", parser.Condition{Field: "container.image", NotContains: "deprecated"}, true},
		{"not_contains fails when substring present", parser.Condition{Field: "container.image", NotContains: "nginx"}, false},
		{"starts_with", parser.Condition{Field: "container.image", StartsWith: "docker.io/"}, true},
		{"ends_with", parser.Condition{Field: "container.image", EndsWith: ":1.25"}, true},
		{"regex", parser.Condition{Field: "resource.name", Regex: `^web-[0-9a-f]+-[a-z0-9]+$`}, true},
		{"regex no match", parser.Condition{Field: "resource.name", Regex: `^db-`}, false},
		{"in matches", parser.Condition{Field: "container.image", In: []interface{}{"nginx:1.25", "docker.io/library/nginx:1.25"}}, true},
		{"in no match", parser.Condition{Field: "container.image", In: []interface{}{"redis:7"}}, false},
		{"not_in", parser.Condition{Field: "container.image", NotIn: []interface{}{"redis:7"}}, true},
		{"greater_than int attribute", parser.Condition{Field: "replica_count", GreaterThan: 1}, true},
		{"greater_than false when equal", parser.Condition{Field: "replica_count", GreaterThan: 5}, false},
		{"less_than", parser.Condition{Field: "replica_count", LessThan: 10}, true},
		{"greater_than_or_equal", parser.Condition{Field: "replica_count", GreaterThanOrEqual: 5}, true},
		{"less_than_or_equal", parser.Condition{Field: "replica_count", LessThanOrEqual: 5}, true},
		{"numeric string attribute parses", parser.Condition{Field: "cpu_cores", GreaterThan: 0.1}, true},
		{"quantity-suffixed string does not parse as numeric", parser.Condition{Field: "cpu_cores", GreaterThan: "0.1m"}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Match(r, c, tc.cond); got != tc.want {
				t.Errorf("Match(%+v) = %v, want %v", tc.cond, got, tc.want)
			}
		})
	}
}

func TestMatch_ContainsOnStringSlice(t *testing.T) {
	// container.security.capabilities_add is exposed as a []string via
	// SecurityContext.Flatten (through EnrichAttributes in production) — contains
	// must work as membership here, not substring-of-a-string.
	c := resource.Container{
		Security: resource.SecurityContext{CapabilitiesAdd: []string{"NET_ADMIN", "SYS_TIME"}},
	}
	c.Attributes = c.Security.Flatten("container.security")
	r := resource.Resource{}

	if !Match(r, c, parser.Condition{Field: "container.security.capabilities_add", Contains: "NET_ADMIN"}) {
		t.Error("expected contains to match a capability present in the slice")
	}
	if Match(r, c, parser.Condition{Field: "container.security.capabilities_add", Contains: "SYS_ADMIN"}) {
		t.Error("expected contains to not match a capability absent from the slice")
	}
}

func TestOperatorNameFor_Precedence(t *testing.T) {
	// contains wins over equals when both are set on one condition (documented
	// pitfall in docs/rulesets_manual.md — not_equals/not_in/not_contains/in still
	// outrank contains, per conditionOperators' order).
	cond := parser.Condition{Equals: "x", Contains: "y"}
	if got := operatorNameFor(cond); got != "contains" {
		t.Errorf("expected contains to win over equals, got %q", got)
	}
}

func TestMatch_AttributeFallback(t *testing.T) {
	r := resource.Resource{
		Attributes: map[string]any{"become": true},
	}
	c := resource.Container{
		Attributes: map[string]any{"no_log": false},
	}

	if !Match(r, c, parser.Condition{Field: "become", Equals: true}) {
		t.Error("expected resource attribute fallback to match")
	}
	if !Match(r, c, parser.Condition{Field: "no_log", Equals: false}) {
		t.Error("expected container attribute fallback to match")
	}
}
