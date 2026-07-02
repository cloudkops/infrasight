package pipeline

import (
	"testing"

	"github.com/cloudkops/infrasight/internal/resource"
)

func TestResolveRelationships_DropsOwnedChild(t *testing.T) {
	resources := []resource.Resource{
		{Kind: "deployment", Namespace: "prod", Name: "web"},
		{
			Kind: "pod", Namespace: "prod", Name: "web-abc123",
			Owner: &resource.Owner{Kind: "deployment", Name: "web"},
		},
		{Kind: "pod", Namespace: "prod", Name: "orphan"}, // no owner, kept
	}

	kept := ResolveRelationships(resources)
	if len(kept) != 2 {
		t.Fatalf("expected 2 resources to survive, got %d: %+v", len(kept), kept)
	}

	names := map[string]bool{}
	for _, r := range kept {
		names[r.Name] = true
	}
	if !names["web"] {
		t.Error("expected the owning deployment to survive")
	}
	if !names["orphan"] {
		t.Error("expected the ownerless pod to survive")
	}
	if names["web-abc123"] {
		t.Error("expected the owned pod to be dropped")
	}
}

func TestResolveRelationships_OwnerNotInSet(t *testing.T) {
	resources := []resource.Resource{
		{
			Kind: "pod", Namespace: "prod", Name: "orphan-owned",
			Owner: &resource.Owner{Kind: "replicaset", Name: "not-tracked"},
		},
	}
	kept := ResolveRelationships(resources)
	if len(kept) != 1 {
		t.Fatalf("expected the resource to survive when its owner isn't in the result set, got %d", len(kept))
	}
}
