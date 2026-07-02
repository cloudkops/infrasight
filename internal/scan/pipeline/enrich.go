package pipeline

import "github.com/cloudkops/infrasight/internal/resource"

// EnrichAttributes makes every typed Security/Networking signal a provider
// collected queryable through the Rule Engine's generic Attributes fallback,
// without a dedicated evaluator case per field. This runs once, generically, for
// every resource regardless of which provider produced it — the same "one
// cross-cutting concern, one place" principle as ResolveRelationships. Providers
// only need to populate the typed structs; this stage is what makes a new
// SecurityContext/Networking field rule-queryable without touching
// internal/rule/evaluator.
func EnrichAttributes(resources []resource.Resource) []resource.Resource {
	for i := range resources {
		r := &resources[i]
		r.Attributes = mergeAttributes(r.Attributes, r.Security.Flatten("resource.security"))
		r.Attributes = mergeAttributes(r.Attributes, r.Networking.Flatten("networking"))

		for j := range r.Runtime.Containers {
			c := &r.Runtime.Containers[j]
			c.Attributes = mergeAttributes(c.Attributes, c.Security.Flatten("container.security"))
		}
	}
	return resources
}

// mergeAttributes merges src into dst without overwriting a key the provider set
// explicitly — a provider's own Attributes value always wins over the generic
// mirror.
func mergeAttributes(dst, src map[string]any) map[string]any {
	if dst == nil {
		dst = make(map[string]any, len(src))
	}
	for k, v := range src {
		if _, exists := dst[k]; !exists {
			dst[k] = v
		}
	}
	return dst
}
