package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudkops/infrasight/internal/provider"
	"github.com/cloudkops/infrasight/internal/resource"
)

// stubProvider is a fake internal/provider.Provider used to exercise the full
// orchestrator -> pipeline -> rule engine chain without a live Kubernetes cluster.
type stubProvider struct{}

func (stubProvider) Name() string        { return "stub" }
func (stubProvider) Kind() provider.Kind { return provider.KindLive }

func (stubProvider) Discover(ctx context.Context, opts provider.Options) ([]resource.Resource, error) {
	return []resource.Resource{
		{
			ID: "1", Name: "root-pod", Namespace: "prod", Kind: "pod", Provider: "stub",
			Metadata: resource.Metadata{Labels: map[string]string{}, Annotations: map[string]string{}},
			Runtime: resource.Runtime{Containers: []resource.Container{
				{Name: "app", User: 0, Privileged: false},
			}},
		},
		{
			ID: "2", Name: "safe-pod", Namespace: "prod", Kind: "pod", Provider: "stub",
			Metadata: resource.Metadata{Labels: map[string]string{}, Annotations: map[string]string{}},
			Runtime: resource.Runtime{Containers: []resource.Container{
				{Name: "app", User: 1000, Privileged: false},
			}},
		},
	}, nil
}

func init() {
	provider.Register("stub", stubProvider{})
}

func TestRun_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	rulesFile := filepath.Join(dir, "rules.yaml")
	rules := `
- id: "TEST-ROOT"
  title: "root container"
  severity: "HIGH"
  condition:
    field: "container.user"
    equals: 0
`
	if err := os.WriteFile(rulesFile, []byte(rules), 0o644); err != nil {
		t.Fatalf("write rules file: %v", err)
	}

	mgr := provider.NewManager()
	result, err := Run(context.Background(), mgr, Request{
		ProviderName: "stub",
		RulesFile:    rulesFile,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.ResourceCount != 2 {
		t.Errorf("expected 2 resources, got %d", result.ResourceCount)
	}
	if result.RuleCount != 1 {
		t.Errorf("expected 1 rule, got %d", result.RuleCount)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(result.Findings), result.Findings)
	}
	if result.Findings[0].ResourceName != "root-pod" {
		t.Errorf("expected the finding to be against root-pod, got %q", result.Findings[0].ResourceName)
	}
}

func TestRun_UnknownProvider(t *testing.T) {
	mgr := provider.NewManager()
	_, err := Run(context.Background(), mgr, Request{ProviderName: "does-not-exist"})
	if err == nil {
		t.Fatal("expected an error for an unknown provider")
	}
}
