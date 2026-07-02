package kubernetes

import (
	"context"
	"fmt"

	"github.com/cloudkops/infrasight/internal/provider"
	"github.com/cloudkops/infrasight/internal/resource"
)

// Name is this provider's registration name ("kubernetes").
const Name = "kubernetes"

// Provider implements internal/provider.Provider for Kubernetes. It is the only
// file in this package that imports internal/provider — client.go, discovery.go,
// and normalize.go know nothing about the Provider interface.
type Provider struct{}

func New() *Provider { return &Provider{} }

func init() {
	provider.Register(Name, New())
}

func (p *Provider) Name() string        { return Name }
func (p *Provider) Kind() provider.Kind { return provider.KindLive }

func (p *Provider) Discover(ctx context.Context, opts provider.Options) ([]resource.Resource, error) {
	client, err := newClient(opts.Kubeconfig, opts.Context)
	if err != nil {
		return nil, fmt.Errorf("kubernetes: client: %w", err)
	}

	namespace := opts.Namespace
	if opts.AllNamespaces {
		namespace = ""
	}

	raw, err := discoverAll(ctx, client, namespace, opts.LabelSelector, opts.FieldSelector)
	if err != nil {
		return nil, err
	}

	return NormalizeAll(raw), nil
}
