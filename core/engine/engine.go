package engine

import (
	"github.com/cloudkops/infrasight/core/model"
	"github.com/cloudkops/infrasight/core/report"
	"github.com/cloudkops/infrasight/core/rules"
)

// ScanRequest carries all input needed for a single scan run.
type ScanRequest struct {
	Workloads []model.Workload
	Rules     []rules.Rule
}

// ScanResult carries findings plus counters for summary reporting.
type ScanResult struct {
	Findings      []report.Finding
	WorkloadCount int
	RuleCount     int
}

// Engine orchestrates rule evaluation across workloads.
type Engine struct{}

func New() *Engine { return &Engine{} }

// Scan evaluates all rules against all workloads and returns the result.
func (e *Engine) Scan(req ScanRequest) ScanResult {
	return ScanResult{
		Findings:      rules.Evaluate(req.Workloads, req.Rules),
		WorkloadCount: len(req.Workloads),
		RuleCount:     len(req.Rules),
	}
}
