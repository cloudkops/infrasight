package host

import (
	"context"
	"fmt"
	"runtime"

	"github.com/cloudkops/infrasight/internal/provider"
	"github.com/cloudkops/infrasight/internal/resource"
)

const Name = "host"

type Provider struct{}

func New() *Provider { return &Provider{} }

func init() {
	provider.Register(Name, New())
}

func (p *Provider) Name() string        { return Name }
func (p *Provider) Kind() provider.Kind { return provider.KindLive }

// Discover scans only the local machine InfraSight is running on — there is no
// remote-agent/SSH fan-out. It is /proc-based and shells out to systemctl, so it
// only functions on Linux.
func (p *Provider) Discover(ctx context.Context, opts provider.Options) ([]resource.Resource, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("host: unsupported OS %q (this provider reads /proc and requires systemd)", runtime.GOOS)
	}

	raw, err := discoverAll(ctx, defaultProcRoot, newSystemdLister())
	if err != nil {
		return nil, err
	}
	return NormalizeAll(raw), nil
}
