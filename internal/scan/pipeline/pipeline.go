package pipeline

import (
	"context"
	"fmt"

	"github.com/cloudkops/infrasight/internal/provider"
	"github.com/cloudkops/infrasight/internal/resource"
)

// Run drives Discovery -> Normalization -> Attribute Enrichment -> Relationship
// Resolution for a single provider. Normalization already happened inside
// Discover (each provider owns its own discovery and normalization, per
// high_level_architecture.md); this function adds the stages that are generic
// across every provider: validating the shape of what came back, making typed
// Security/Networking signals rule-queryable, and resolving ownership
// relationships.
func Run(ctx context.Context, p provider.Provider, opts provider.Options) ([]resource.Resource, error) {
	resources, err := p.Discover(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("pipeline: %s: discover: %w", p.Name(), err)
	}

	for _, r := range resources {
		if err := resource.Validate(r); err != nil {
			return nil, fmt.Errorf("pipeline: %s: %w", p.Name(), err)
		}
	}

	resources = EnrichAttributes(resources)
	return ResolveRelationships(resources), nil
}
