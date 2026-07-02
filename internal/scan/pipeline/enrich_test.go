package pipeline

import (
	"testing"

	"github.com/cloudkops/infrasight/internal/resource"
)

func TestEnrichAttributes_FlattensSecurityAndNetworking(t *testing.T) {
	resources := []resource.Resource{
		{
			Name:       "web",
			Security:   resource.SecurityContext{RunAsRoot: true},
			Networking: resource.Networking{Exposures: []resource.Exposure{{Type: "hostNetwork"}}},
			Runtime: resource.Runtime{Containers: []resource.Container{
				{Name: "app", Security: resource.SecurityContext{Privileged: true}},
			}},
		},
	}

	got := EnrichAttributes(resources)

	r := got[0]
	if r.Attributes["resource.security.run_as_root"] != true {
		t.Errorf("expected resource.security.run_as_root=true, got %v", r.Attributes["resource.security.run_as_root"])
	}
	if r.Attributes["networking.hostNetwork"] != true {
		t.Errorf("expected networking.hostNetwork=true, got %v", r.Attributes["networking.hostNetwork"])
	}

	c := r.Runtime.Containers[0]
	if c.Attributes["container.security.privileged"] != true {
		t.Errorf("expected container.security.privileged=true, got %v", c.Attributes["container.security.privileged"])
	}
}

func TestEnrichAttributes_DoesNotOverwriteProviderSetAttribute(t *testing.T) {
	resources := []resource.Resource{
		{
			Security:   resource.SecurityContext{Privileged: true},
			Attributes: map[string]any{"resource.security.privileged": "provider-set-value"},
		},
	}

	got := EnrichAttributes(resources)

	if got[0].Attributes["resource.security.privileged"] != "provider-set-value" {
		t.Errorf("expected the provider's own attribute to win, got %v", got[0].Attributes["resource.security.privileged"])
	}
}
