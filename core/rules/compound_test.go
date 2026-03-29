package rules

import (
	"testing"

	"github.com/cloudkops/infrasight/core/model"
)

func TestMatchRule_SingleCondition(t *testing.T) {
	r := Rule{
		ID: "R1", Title: "t", Severity: "HIGH",
		Condition: Condition{Field: "container.user", Equals: 0},
	}
	if !matchRule(emptyWL(), mkContainer(0, false), r) {
		t.Error("expected match for user=0")
	}
	if matchRule(emptyWL(), mkContainer(1000, false), r) {
		t.Error("expected no match for user=1000")
	}
}

func TestMatchRule_CompoundAND_AllMatch(t *testing.T) {
	r := Rule{
		ID: "R-AND", Title: "t", Severity: "HIGH",
		Conditions: []Condition{
			{Field: "container.user", Equals: 0},
			{Field: "container.privileged", Equals: true},
		},
	}
	if !matchRule(emptyWL(), mkContainer(0, true), r) {
		t.Error("expected match when both AND conditions met")
	}
}

func TestMatchRule_CompoundAND_PartialNoMatch(t *testing.T) {
	r := Rule{
		ID: "R-AND", Title: "t", Severity: "HIGH",
		Conditions: []Condition{
			{Field: "container.user", Equals: 0},
			{Field: "container.privileged", Equals: true},
		},
	}
	if matchRule(emptyWL(), mkContainer(0, false), r) {
		t.Error("expected no match when only first AND condition met")
	}
}

func TestMatchRule_CompoundOR_FirstMatches(t *testing.T) {
	r := Rule{
		ID: "R-OR", Title: "t", Severity: "HIGH",
		AnyOf: []Condition{
			{Field: "container.user", Equals: 0},
			{Field: "container.privileged", Equals: true},
		},
	}
	if !matchRule(emptyWL(), mkContainer(0, false), r) {
		t.Error("expected match when first OR condition met")
	}
}

func TestMatchRule_CompoundOR_SecondMatches(t *testing.T) {
	r := Rule{
		ID: "R-OR", Title: "t", Severity: "HIGH",
		AnyOf: []Condition{
			{Field: "container.user", Equals: 0},
			{Field: "container.privileged", Equals: true},
		},
	}
	if !matchRule(emptyWL(), mkContainer(1000, true), r) {
		t.Error("expected match when second OR condition met")
	}
}

func TestMatchRule_CompoundOR_NoneMatch(t *testing.T) {
	r := Rule{
		ID: "R-OR", Title: "t", Severity: "HIGH",
		AnyOf: []Condition{
			{Field: "container.user", Equals: 0},
			{Field: "container.privileged", Equals: true},
		},
	}
	if matchRule(emptyWL(), mkContainer(1000, false), r) {
		t.Error("expected no match when no OR conditions met")
	}
}

func TestMatchRule_EmptyConditions_FallsThrough(t *testing.T) {
	r := Rule{
		ID: "R", Title: "t", Severity: "HIGH",
		Conditions: []Condition{},
		Condition:  Condition{Field: "container.user", Equals: 0},
	}
	if !matchRule(emptyWL(), mkContainer(0, false), r) {
		t.Error("expected fallthrough to single condition when Conditions is empty")
	}
}

func TestEvaluateCondition_NotEquals_String(t *testing.T) {
	tests := []struct {
		image, notEquals string
		want             bool
	}{
		{"nginx:stable", "nginx:latest", true},
		{"nginx:latest", "nginx:latest", false},
		{"", "nginx:latest", true},
	}
	for _, tt := range tests {
		c := model.Container{Image: tt.image}
		got := evaluateCondition(emptyWL(), c, Condition{Field: "container.image", NotEquals: tt.notEquals})
		if got != tt.want {
			t.Errorf("NotEquals(%q, %q) = %v, want %v", tt.image, tt.notEquals, got, tt.want)
		}
	}
}

func TestEvaluateCondition_NotEquals_User(t *testing.T) {
	c := mkContainer(0, false)
	if evaluateCondition(emptyWL(), c, Condition{Field: "container.user", NotEquals: 0}) {
		t.Error("expected no match when user equals NotEquals value")
	}
	if !evaluateCondition(emptyWL(), c, Condition{Field: "container.user", NotEquals: 1000}) {
		t.Error("expected match when user differs from NotEquals value")
	}
}

func TestEvaluateCondition_Contains(t *testing.T) {
	tests := []struct {
		image, contains string
		want            bool
	}{
		{"nginx:latest", ":latest", true},
		{"nginx:1.25", ":latest", false},
		{"myrepo/api:latest", "latest", true},
		{"myrepo/api:v1.0", "latest", false},
	}
	for _, tt := range tests {
		c := model.Container{Image: tt.image}
		got := evaluateCondition(emptyWL(), c, Condition{Field: "container.image", Contains: tt.contains})
		if got != tt.want {
			t.Errorf("Contains(%q in %q) = %v, want %v", tt.contains, tt.image, got, tt.want)
		}
	}
}

func TestEvaluateCondition_WorkloadType(t *testing.T) {
	w := model.Workload{Type: "deployment"}
	c := mkContainer(1000, false)
	if !evaluateCondition(w, c, Condition{Field: "workload.type", Equals: "deployment"}) {
		t.Error("expected match on workload.type=deployment")
	}
	if evaluateCondition(w, c, Condition{Field: "workload.type", Equals: "pod"}) {
		t.Error("expected no match for pod on deployment workload")
	}
}

func TestEvaluateCondition_NodePort(t *testing.T) {
	withNodePort := model.Container{Exposures: []model.Exposure{{Type: "NodePort"}}}
	without := mkContainer(1000, false)
	if !evaluateCondition(emptyWL(), withNodePort, Condition{Field: "container.exposure.NodePort", Equals: true}) {
		t.Error("expected NodePort match")
	}
	if evaluateCondition(emptyWL(), without, Condition{Field: "container.exposure.NodePort", Equals: true}) {
		t.Error("expected no NodePort match on clean container")
	}
}

func TestActiveField_Single(t *testing.T) {
	r := Rule{Condition: Condition{Field: "container.user"}}
	if activeField(r) != "container.user" {
		t.Errorf("got %q, want container.user", activeField(r))
	}
}

func TestActiveField_Conditions(t *testing.T) {
	r := Rule{Conditions: []Condition{{Field: "container.user"}, {Field: "container.privileged"}}}
	if activeField(r) != "container.user" {
		t.Errorf("got %q, want container.user", activeField(r))
	}
}

func TestActiveField_AnyOf(t *testing.T) {
	r := Rule{AnyOf: []Condition{{Field: "container.image"}, {Field: "container.privileged"}}}
	if activeField(r) != "container.image" {
		t.Errorf("got %q, want container.image", activeField(r))
	}
}

func TestActiveField_Empty(t *testing.T) {
	if activeField(Rule{}) != "" {
		t.Error("expected empty string for zero-value Rule")
	}
}

// ── test helpers ──────────────────────────────────────────────────────────────

func emptyWL() model.Workload { return model.Workload{} }
func mkContainer(user int64, privileged bool) model.Container {
	return model.Container{User: user, Privileged: privileged}
}
