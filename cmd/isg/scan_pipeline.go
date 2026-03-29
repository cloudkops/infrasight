package isg

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/cloudkops/infrasight/core/engine"
	"github.com/cloudkops/infrasight/core/report"
	"github.com/cloudkops/infrasight/core/rules"
	"github.com/spf13/cobra"
)

// runScan executes the full scan pipeline and returns the process exit code.
// The caller is responsible for calling os.Exit with the returned value.
func runScan(cmd *cobra.Command, opts *scanOptions) int {
	if opts.verbose {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	workloads, err := loadWorkloads(opts)
	if err != nil {
		cmd.PrintErrln("error loading workloads:", err)
		return 1
	}
	slog.Debug("workloads loaded", "count", len(workloads))

	ruleSet, err := loadRules(opts)
	if err != nil {
		cmd.PrintErrln("error loading rules:", err)
		return 1
	}
	slog.Debug("rules loaded", "count", len(ruleSet))

	eng := engine.New()
	result := eng.Scan(engine.ScanRequest{
		Workloads: workloads,
		Rules:     ruleSet,
	})

	out, closeOut, err := openOutput(opts.outputFile)
	if err != nil {
		cmd.PrintErrln("error opening output file:", err)
		return 1
	}
	defer closeOut()

	renderFindings(opts, result.Findings, out)
	report.WriteSummary(os.Stderr, result.Findings, result.WorkloadCount, result.RuleCount)

	if opts.exitOne && len(result.Findings) > 0 {
		return 1
	}
	return report.ExitCode(result.Findings, opts.failOn)
}

func loadRules(opts *scanOptions) ([]rules.Rule, error) {
	if opts.rulesetFlag != "" {
		return rules.LoadRuleset(opts.rulesetFlag)
	}
	return rules.Load(opts.rulesFlag)
}

func renderFindings(opts *scanOptions, findings []report.Finding, w io.Writer) {
	switch {
	case opts.formatFlag == "sarif":
		_ = report.WriteSARIF(w, findings)
	case opts.ciEnabled || opts.formatFlag == "json":
		_ = report.WriteJSON(w, findings, opts.formatFlag == "json")
	default:
		report.WriteTable(w, findings)
	}
}

func openOutput(path string) (io.Writer, func(), error) {
	if path == "" {
		return os.Stdout, func() {}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("create output file %q: %w", path, err)
	}
	return f, func() { _ = f.Close() }, nil
}
