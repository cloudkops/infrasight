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
