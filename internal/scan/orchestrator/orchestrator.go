package orchestrator

import (
	"context"
	"fmt"

	"github.com/cloudkops/infrasight/internal/provider"
	scanengine "github.com/cloudkops/infrasight/internal/scan/engine"
	"github.com/cloudkops/infrasight/internal/scan/pipeline"

	"github.com/cloudkops/infrasight/internal/rule/parser"
)

// Request carries everything needed to run one scan: which provider, its options,
// and where to load rules from.
type Request struct {
	ProviderName string
	Options      provider.Options
	RulesFile    string // single YAML file — mutually exclusive with RulesetDir
	RulesetDir   string // directory of YAML files, filtered by provider tag
}

// Run is the full scan lifecycle: resolve provider -> discover/normalize/resolve
// relationships (internal/scan/pipeline) -> load rules -> evaluate
// (internal/scan/engine). The orchestrator knows nothing about Kubernetes, Docker,
// or any other concrete provider — it only calls internal/provider.Manager and the
// stage packages.
func Run(ctx context.Context, mgr *provider.Manager, req Request) (scanengine.ScanResult, error) {
	p, err := mgr.Get(req.ProviderName)
	if err != nil {
		return scanengine.ScanResult{}, err
	}

	if err := mgr.Initialize(ctx, p); err != nil {
		return scanengine.ScanResult{}, fmt.Errorf("orchestrator: initialize %s: %w", req.ProviderName, err)
	}
	defer mgr.Shutdown(ctx, p)

	resources, err := pipeline.Run(ctx, p, req.Options)
	if err != nil {
		return scanengine.ScanResult{}, err
	}

	rules, err := loadRules(req, p.Name())
	if err != nil {
		return scanengine.ScanResult{}, err
	}

	return scanengine.Scan(scanengine.ScanRequest{Resources: resources, Rules: rules}), nil
}

func loadRules(req Request, providerName string) ([]parser.Rule, error) {
	if req.RulesetDir != "" {
		return parser.LoadRuleset(req.RulesetDir, providerName)
	}
	if req.RulesFile != "" {
		return parser.Load(req.RulesFile)
	}
	return nil, fmt.Errorf("orchestrator: no rules source given (set --rules or --ruleset)")
}
