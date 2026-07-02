package engine

import (
	"testing"

	"github.com/cloudkops/infrasight/internal/resource"
	"github.com/cloudkops/infrasight/internal/rule/parser"
)

func TestEvaluate_SingleAndCompoundConditions(t *testing.T) {
	resources := []resource.Resource{
		{
			Name: "web", Namespace: "prod", Kind: "pod",
			Runtime: resource.Runtime{Containers: []resource.Container{
				{Name: "nginx", User: 0, Privileged: false},
			}},
		},
		{
			Name: "db", Namespace: "prod", Kind: "pod",
			Runtime: resource.Runtime{Containers: []resource.Container{
				{Name: "mysql", User: 999, Privileged: true},
			}},
		},
	}

	rules := []parser.Rule{
		{
			ID: "ROOT", Title: "root", Severity: "HIGH",
			Condition: parser.Condition{Field: "container.user", Equals: 0},
		},
		{
			ID: "PRIV", Title: "privileged", Severity: "CRITICAL",
			Condition: parser.Condition{Field: "container.privileged", Equals: true},
		},
	}

	findings := Evaluate(resources, rules)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d: %+v", len(findings), findings)
	}

	byRule := map[string]string{}
	for _, f := range findings {
		byRule[f.RuleID] = f.ResourceName
	}
	if byRule["ROOT"] != "web" {
		t.Errorf("ROOT should have matched web, matched %q", byRule["ROOT"])
	}
	if byRule["PRIV"] != "db" {
		t.Errorf("PRIV should have matched db, matched %q", byRule["PRIV"])
	}
}

func TestEvaluate_NoContainers(t *testing.T) {
	resources := []resource.Resource{
		{Name: "svc", Namespace: "prod", Kind: "service"},
	}
	rules := []parser.Rule{
		{
			ID: "NS", Title: "namespace check", Severity: "LOW",
			Condition: parser.Condition{Field: "resource.namespace", Equals: "prod"},
		},
	}

	findings := Evaluate(resources, rules)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for a container-less resource, got %d", len(findings))
	}
}

func TestEvaluate_AnyOf(t *testing.T) {
	// Networking fields are resolved via Attributes, populated in production by
	// internal/scan/pipeline.EnrichAttributes (not by Evaluate itself) — flatten
	// here to simulate that stage having already run, the same way a real scan
	// would reach this function.
	networking := resource.Networking{Exposures: []resource.Exposure{{Type: "hostNetwork"}}}
	resources := []resource.Resource{
		{
			Name: "web", Kind: "pod",
			Networking: networking,
			Attributes: networking.Flatten("networking"),
		},
	}
	rules := []parser.Rule{
		{
			ID: "EXPOSED", Title: "exposed", Severity: "HIGH",
			AnyOf: []parser.Condition{
				{Field: "networking.hostNetwork", Equals: true},
				{Field: "networking.publicIP", Equals: true},
			},
		},
	}

	findings := Evaluate(resources, rules)
	if len(findings) != 1 {
		t.Fatalf("expected any_of to match via hostNetwork, got %d findings", len(findings))
	}
}
