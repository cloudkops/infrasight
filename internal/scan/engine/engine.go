package engine

import (
	"github.com/cloudkops/infrasight/internal/report"
	"github.com/cloudkops/infrasight/internal/resource"
	ruleengine "github.com/cloudkops/infrasight/internal/rule/engine"
	"github.com/cloudkops/infrasight/internal/rule/parser"
)

// ScanRequest carries everything needed for a single scan computation.
type ScanRequest struct {
	Resources []resource.Resource
	Rules     []parser.Rule
}

// ScanResult carries findings plus counters for summary reporting.
type ScanResult struct {
	Findings      []report.Finding
	ResourceCount int
	RuleCount     int
}

// Scan is the stateless computation core: resources + rules in, findings out. No
// provider-lifecycle concerns here — internal/scan/orchestrator owns those.
func Scan(req ScanRequest) ScanResult {
	return ScanResult{
		Findings:      ruleengine.Evaluate(req.Resources, req.Rules),
		ResourceCount: len(req.Resources),
		RuleCount:     len(req.Rules),
	}
}
