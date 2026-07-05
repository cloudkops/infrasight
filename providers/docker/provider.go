package docker

import (
	"context"
	"fmt"

	"github.com/cloudkops/infrasight/internal/provider"
	"github.com/cloudkops/infrasight/internal/resource"
)

const Name = "docker"

type Provider struct{}

func (p *Provider) Name() string {
	return Name
}

func (p *Provider) Kind() provider.Kind {
	return provider.KindLive
}

func (p *Provider) Discover(ctx context.Context, opts provider.Options) ([]resource.Resource, error) {
	client, err := newClient(opts.DockerHost)
	if err != nil {
		return nil, fmt.Errorf("docker: client: %w", err)
	}
	defer client.Close()
	raw, err := discoverAll(ctx, client)
	if err != nil {
		return nil, err
	}
	return NormalizeAll(raw), nil
}

func New() *Provider { return &Provider{} }

func init() {
	provider.Register(Name, New())
}
