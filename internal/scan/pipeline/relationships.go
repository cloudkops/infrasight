package pipeline

import (
	"strings"

	"github.com/cloudkops/infrasight/internal/resource"
)

// ResolveRelationships drops any resource whose Owner resolves to another resource
// already present in the same result set. This is generic and provider-agnostic —
// the same logic works whether the owner relationship came from Kubernetes
// (Pod owned by a Deployment), Docker (container owned by a compose service), or
// any future provider with an ownership concept. It is not duplicated per provider.
func ResolveRelationships(resources []resource.Resource) []resource.Resource {
	present := make(map[string]bool, len(resources))
	for _, r := range resources {
		present[identityKey(r.Kind, r.Namespace, r.Name)] = true
	}

	kept := make([]resource.Resource, 0, len(resources))
	for _, r := range resources {
		if r.Owner != nil && present[identityKey(r.Owner.Kind, r.Namespace, r.Owner.Name)] {
			continue // owner is tracked separately in this result set; drop the redundant child
		}
		kept = append(kept, r)
	}
	return kept
}

func identityKey(kind, namespace, name string) string {
	return strings.ToLower(kind) + "/" + namespace + "/" + name
}
