package rules

import (
	"testing"

	"github.com/cloudkops/infrasight/core/model"
	"github.com/cloudkops/infrasight/core/report"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func container(opts ...func(*model.Container)) model.Container {
	c := model.Container{Name: "app", Image: "nginx:1.25", User: 1000}
	for _, o := range opts {
		o(&c)
	}
	return c
}

func workload(name string, containers ...model.Container) model.Workload {
	return model.Workload{Name: name, Namespace: "default", Platform: "kubernetes", Containers: containers}
}

func rule(id, field string, equals interface{}) Rule {
	return Rule{ID: id, Title: id, Severity: "HIGH", Condition: Condition{Field: field, Equals: equals}}
}

func ruleExists(id, field string, exists bool) Rule {
	return Rule{ID: id, Title: id, Severity: "MEDIUM", Condition: Condition{Field: field, Exists: exists}}
}

// ── Evaluate ─────────────────────────────────────────────────────────────────

func TestEvaluate_NoRules(t *testing.T) {
	findings := Evaluate([]model.Workload{workload("w1", container())}, nil)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

func TestEvaluate_NoWorkloads(t *testing.T) {
	findings := Evaluate(nil, []Rule{rule("R1", "container.user", 0)})
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

func TestEvaluate_FindingFieldsPopulated(t *testing.T) {
	r := Rule{
		ID:          "ISG-TEST-001",
		Title:       "Root container",
		Severity:    "HIGH",
		Category:    "security",
		Description: "desc",
		Remediation: "fix",
		DocsUrl:     "https://example.com",
		Condition:   Condition{Field: "container.user", Equals: 0},
	}
	c := container(func(c *model.Container) { c.User = 0; c.Name = "nginx" })
	w := workload("myapp", c)

	findings := Evaluate([]model.Workload{w}, []Rule{r})

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	checks := []struct {
		got, want, field string
	}{
		{f.RuleID, "ISG-TEST-001", "RuleID"},
		{f.Title, "Root container", "Title"},
		{f.Severity, "HIGH", "Severity"},
		{f.Category, "security", "Category"},
		{f.Description, "desc", "Description"},
		{f.Remediation, "fix", "Remediation"},
		{f.DocsUrl, "https://example.com", "DocsUrl"},
		{f.Workload, "myapp", "Workload"},
		{f.Container, "nginx", "Container"},
		{f.Field, "container.user", "Field"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("Finding.%s = %q, want %q", c.field, c.got, c.want)
		}
	}
}

func TestEvaluate_MultipleContainersMultipleRules(t *testing.T) {
	c1 := model.Container{Name: "c1", User: 0, Privileged: false}
	c2 := model.Container{Name: "c2", User: 1000, Privileged: true}
	w := workload("app", c1, c2)

	rules := []Rule{
		rule("R1", "container.user", 0),
		rule("R2", "container.privileged", true),
	}

	findings := Evaluate([]model.Workload{w}, rules)

	// c1 triggers R1, c2 triggers R2 → 2 findings
	if len(findings) != 2 {
		t.Errorf("expected 2 findings, got %d", len(findings))
	}
}

// ── evaluateCondition: container fields ──────────────────────────────────────

func TestEvaluateCondition_ContainerUser(t *testing.T) {
	tests := []struct {
		name   string
		user   int64
		equals interface{}
		match  bool
	}{
		{"root user matches", 0, 0, true},
		{"root user int64", 0, int64(0), true},
		{"root user float64", 0, float64(0), true},
		{"non-root no match", 1000, 0, false},
		{"bad type no match", 0, "zero", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := model.Container{User: tt.user}
			got := evaluateCondition(model.Workload{}, c, Condition{Field: "container.user", Equals: tt.equals})
			if got != tt.match {
				t.Errorf("got %v, want %v", got, tt.match)
			}
		})
	}
}

func TestEvaluateCondition_ContainerPrivileged(t *testing.T) {
	tests := []struct {
		name       string
		privileged bool
		equals     interface{}
		match      bool
	}{
		{"privileged matches true", true, true, true},
		{"non-privileged no match", false, true, false},
		{"non-privileged matches false", false, false, true},
		{"bad type no match", true, "yes", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := model.Container{Privileged: tt.privileged}
			got := evaluateCondition(model.Workload{}, c, Condition{Field: "container.privileged", Equals: tt.equals})
			if got != tt.match {
				t.Errorf("got %v, want %v", got, tt.match)
			}
		})
	}
}

func TestEvaluateCondition_ContainerImage(t *testing.T) {
	c := model.Container{Image: "nginx:latest"}
	tests := []struct {
		equals interface{}
		match  bool
	}{
		{"nginx:latest", true},
		{"nginx:1.25", false},
		{123, false},
	}
	for _, tt := range tests {
		got := evaluateCondition(model.Workload{}, c, Condition{Field: "container.image", Equals: tt.equals})
		if got != tt.match {
			t.Errorf("image equals=%v: got %v, want %v", tt.equals, got, tt.match)
		}
	}
}

func TestEvaluateCondition_ContainerName(t *testing.T) {
	c := model.Container{Name: "sidecar"}
	got := evaluateCondition(model.Workload{}, c, Condition{Field: "container.name", Equals: "sidecar"})
	if !got {
		t.Error("expected match for container name")
	}
	got = evaluateCondition(model.Workload{}, c, Condition{Field: "container.name", Equals: "main"})
	if got {
		t.Error("expected no match for different container name")
	}
}

// ── evaluateCondition: workload fields ───────────────────────────────────────

func TestEvaluateCondition_WorkloadName(t *testing.T) {
	w := model.Workload{Name: "payments"}
	c := model.Container{}
	if !evaluateCondition(w, c, Condition{Field: "workload.name", Equals: "payments"}) {
		t.Error("expected match")
	}
	if evaluateCondition(w, c, Condition{Field: "workload.name", Equals: "orders"}) {
		t.Error("expected no match")
	}
}

func TestEvaluateCondition_WorkloadNamespace(t *testing.T) {
	w := model.Workload{Namespace: "production"}
	c := model.Container{}
	if !evaluateCondition(w, c, Condition{Field: "workload.namespace", Equals: "production"}) {
		t.Error("expected match for correct namespace")
	}
	if evaluateCondition(w, c, Condition{Field: "workload.namespace", Equals: "staging"}) {
		t.Error("expected no match for wrong namespace")
	}
}

func TestEvaluateCondition_WorkloadPlatform(t *testing.T) {
	w := model.Workload{Platform: "kubernetes"}
	c := model.Container{}
	if !evaluateCondition(w, c, Condition{Field: "workload.platform", Equals: "kubernetes"}) {
		t.Error("expected match")
	}
}

// ── evaluateCondition: resources ─────────────────────────────────────────────

func TestEvaluateCondition_ResourceMissing(t *testing.T) {
	tests := []struct {
		name      string
		field     string
		resources model.Resource
		exists    bool
		wantMatch bool
	}{
		{"cpu_limit missing, exists false → match", "container.resources.cpu_limit", model.Resource{}, false, true},
		{"cpu_limit set, exists false → no match", "container.resources.cpu_limit", model.Resource{CPULimit: "500m"}, false, false},
		{"cpu_limit set, exists true → match", "container.resources.cpu_limit", model.Resource{CPULimit: "500m"}, true, true},
		{"memory_limit missing, exists false → match", "container.resources.memory_limit", model.Resource{}, false, true},
		{"memory_limit set, exists false → no match", "container.resources.memory_limit", model.Resource{MemoryLimit: "256Mi"}, false, false},
		{"cpu_request missing, exists false → match", "container.resources.cpu_request", model.Resource{}, false, true},
		{"memory_request missing, exists false → match", "container.resources.memory_request", model.Resource{}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := model.Container{Resources: tt.resources}
			cond := Condition{Field: tt.field, Exists: tt.exists}
			got := evaluateCondition(model.Workload{}, c, cond)
			if got != tt.wantMatch {
				t.Errorf("got %v, want %v", got, tt.wantMatch)
			}
		})
	}
}

func TestEvaluateCondition_ResourceExactValue(t *testing.T) {
	c := model.Container{Resources: model.Resource{CPULimit: "500m", MemoryLimit: "256Mi"}}
	if !evaluateCondition(model.Workload{}, c, Condition{Field: "container.resources.cpu_limit", Equals: "500m"}) {
		t.Error("expected exact cpu_limit match")
	}
	if evaluateCondition(model.Workload{}, c, Condition{Field: "container.resources.cpu_limit", Equals: "1"}) {
		t.Error("expected no match for different cpu_limit")
	}
}

// ── evaluateCondition: exposures ─────────────────────────────────────────────

func TestEvaluateCondition_HostNetwork(t *testing.T) {
	withHost := model.Container{Exposures: []model.Exposure{{Type: "hostNetwork"}}}
	withoutHost := model.Container{}

	if !evaluateCondition(model.Workload{}, withHost, Condition{Field: "container.exposure.hostNetwork", Equals: true}) {
		t.Error("expected hostNetwork match")
	}
	if evaluateCondition(model.Workload{}, withoutHost, Condition{Field: "container.exposure.hostNetwork", Equals: true}) {
		t.Error("expected no hostNetwork match on clean container")
	}
}

func TestEvaluateCondition_PublicIP(t *testing.T) {
	withIP := model.Container{Exposures: []model.Exposure{{Type: "publicIP"}}}
	withoutIP := model.Container{}

	if !evaluateCondition(model.Workload{}, withIP, Condition{Field: "container.exposure.publicIP", Equals: true}) {
		t.Error("expected publicIP match")
	}
	if evaluateCondition(model.Workload{}, withoutIP, Condition{Field: "container.exposure.publicIP", Equals: true}) {
		t.Error("expected no publicIP match")
	}
}

// ── evaluateCondition: labels & annotations ───────────────────────────────────

func TestEvaluateCondition_WorkloadLabels(t *testing.T) {
	w := model.Workload{Metadata: model.Metadata{Labels: map[string]string{"env": "production", "team": "platform"}}}
	c := model.Container{}

	if !evaluateCondition(w, c, Condition{Field: "workload.labels.env", Equals: "production"}) {
		t.Error("expected env label match")
	}
	if evaluateCondition(w, c, Condition{Field: "workload.labels.env", Equals: "staging"}) {
		t.Error("expected no match for wrong label value")
	}
	// missing label → map returns "" → equals "" triggers finding
	if !evaluateCondition(w, c, Condition{Field: "workload.labels.app", Equals: ""}) {
		t.Error("expected match for missing label (empty value)")
	}
}

func TestEvaluateCondition_WorkloadAnnotations(t *testing.T) {
	w := model.Workload{Metadata: model.Metadata{Annotations: map[string]string{"owner": "jane"}}}
	c := model.Container{}

	if !evaluateCondition(w, c, Condition{Field: "workload.annotations.owner", Equals: "jane"}) {
		t.Error("expected annotation match")
	}
	if evaluateCondition(w, c, Condition{Field: "workload.annotations.owner", Equals: "bob"}) {
		t.Error("expected no match for wrong annotation")
	}
}

func TestEvaluateCondition_UnknownField(t *testing.T) {
	c := model.Container{}
	got := evaluateCondition(model.Workload{}, c, Condition{Field: "container.unknown.field", Equals: "x"})
	if got {
		t.Error("unknown field should never match")
	}
}

// ── end-to-end Evaluate with all categories ───────────────────────────────────

func TestEvaluate_EndToEnd(t *testing.T) {
	workloads := []model.Workload{
		{
			Name:      "api",
			Namespace: "production",
			Platform:  "kubernetes",
			Metadata: model.Metadata{
				Labels: map[string]string{"team": "backend"},
			},
			Containers: []model.Container{
				{
					Name:       "api-server",
					Image:      "myrepo/api:latest",
					User:       0,
					Privileged: false,
					Resources:  model.Resource{CPULimit: "500m"},
					Exposures:  []model.Exposure{},
				},
			},
		},
	}

	rules := []Rule{
		{ID: "R1", Title: "Root user", Severity: "HIGH", Condition: Condition{Field: "container.user", Equals: 0}},
		{ID: "R2", Title: "Privileged", Severity: "CRITICAL", Condition: Condition{Field: "container.privileged", Equals: true}},
		{ID: "R3", Title: "Missing mem limit", Severity: "MEDIUM", Condition: Condition{Field: "container.resources.memory_limit", Exists: false}},
		{ID: "R4", Title: "Missing team label", Severity: "LOW", Condition: Condition{Field: "workload.labels.app", Equals: ""}},
	}

	findings := Evaluate(workloads, rules)

	want := []string{"R1", "R3", "R4"}
	if len(findings) != len(want) {
		t.Fatalf("expected %d findings, got %d: %+v", len(want), len(findings), findingIDs(findings))
	}
	for i, id := range want {
		if findings[i].RuleID != id {
			t.Errorf("findings[%d].RuleID = %q, want %q", i, findings[i].RuleID, id)
		}
	}
}

func findingIDs(fs []report.Finding) []string {
	ids := make([]string, len(fs))
	for i, f := range fs {
		ids[i] = f.RuleID
	}
	return ids
}

// ── coercion helpers ──────────────────────────────────────────────────────────

func TestAsInt64(t *testing.T) {
	tests := []struct {
		in   interface{}
		want int64
		ok   bool
	}{
		{int(0), 0, true},
		{int(42), 42, true},
		{int64(100), 100, true},
		{float64(3), 3, true},
		{uint64(7), 7, true},
		{"str", 0, false},
		{nil, 0, false},
		{true, 0, false},
	}
	for _, tt := range tests {
		got, ok := asInt64(tt.in)
		if ok != tt.ok || (ok && got != tt.want) {
			t.Errorf("asInt64(%v) = (%v, %v), want (%v, %v)", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestAsBool(t *testing.T) {
	tests := []struct {
		in   interface{}
		want bool
		ok   bool
	}{
		{true, true, true},
		{false, false, true},
		{"true", false, false},
		{1, false, false},
		{nil, false, false},
	}
	for _, tt := range tests {
		got, ok := asBool(tt.in)
		if ok != tt.ok || (ok && got != tt.want) {
			t.Errorf("asBool(%v) = (%v, %v), want (%v, %v)", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestAsString(t *testing.T) {
	tests := []struct {
		in   interface{}
		want string
		ok   bool
	}{
		{"hello", "hello", true},
		{"", "", true},
		{int(42), "42", true},
		{int64(7), "7", true},
		{float64(3.14), "3.14", true},
		{nil, "", false},
		{true, "", false},
	}
	for _, tt := range tests {
		got, ok := asString(tt.in)
		if ok != tt.ok || (ok && got != tt.want) {
			t.Errorf("asString(%v) = (%q, %v), want (%q, %v)", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

// ── matchResourceValueOrExistence ─────────────────────────────────────────────

func TestMatchResourceValueOrExistence(t *testing.T) {
	tests := []struct {
		name   string
		actual string
		cond   Condition
		want   bool
	}{
		{"empty, exists false → match", "", Condition{Exists: false}, true},
		{"set, exists false → no match", "500m", Condition{Exists: false}, false},
		{"set, exists true → match", "500m", Condition{Exists: true}, true},
		{"empty, exists true → no match", "", Condition{Exists: true}, false},
		{"equals exact match", "256Mi", Condition{Equals: "256Mi"}, true},
		{"equals mismatch", "128Mi", Condition{Equals: "256Mi"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchResourceValueOrExistence(tt.actual, tt.cond)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// ── hasExposureType ───────────────────────────────────────────────────────────

func TestHasExposureType(t *testing.T) {
	exposures := []model.Exposure{
		{Type: "port", Port: 8080},
		{Type: "hostNetwork"},
	}
	if !hasExposureType(exposures, "hostNetwork") {
		t.Error("expected hostNetwork found")
	}
	if hasExposureType(exposures, "publicIP") {
		t.Error("expected publicIP not found")
	}
	if hasExposureType(nil, "hostNetwork") {
		t.Error("expected no match on nil exposures")
	}
}
