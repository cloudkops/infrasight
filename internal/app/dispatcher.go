package app

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/cloudkops/infrasight/internal/provider"
	"github.com/cloudkops/infrasight/internal/report"
	"github.com/cloudkops/infrasight/internal/scan/orchestrator"
)

// dispatch is the Command Dispatcher: it validates args, builds the Scan
// Orchestrator request, runs the scan, renders output, and returns the process
// exit code. It resolves providers exclusively through internal/provider.Manager —
// it never imports a providers/* package.
func dispatch(cmd *cobra.Command, providerName string, f *scanFlags) int {
	req := orchestrator.Request{
		ProviderName: providerName,
		RulesFile:    f.rulesFile,
		RulesetDir:   f.rulesetDir,
		Options: provider.Options{
			Namespace:     f.namespace,
			AllNamespaces: f.allNamespaces,
			LabelSelector: f.labelSelector,
			FieldSelector: f.fieldSelector,
			Verbose:       f.verbose,
			Kubeconfig:    f.kubeconfig,
			Context:       f.kubeContext,
		},
	}

	result, err := orchestrator.Run(context.Background(), manager, req)
	if err != nil {
		cmd.PrintErrln("error:", err)
		return 1
	}

	out, closeOut, err := openOutput(f.outputFile)
	if err != nil {
		cmd.PrintErrln("error opening output file:", err)
		return 1
	}
	defer closeOut()

	renderer, err := report.Get(f.format)
	if err != nil {
		cmd.PrintErrln("error:", err)
		return 1
	}
	if err := renderer.Write(out, result.Findings); err != nil {
		cmd.PrintErrln("error rendering output:", err)
		return 1
	}

	report.WriteSummary(os.Stderr, result.Findings, result.ResourceCount, result.RuleCount)

	if f.exitOne && len(result.Findings) > 0 {
		return 1
	}
	return report.ExitCode(result.Findings, f.failOn)
}

func openOutput(path string) (io.Writer, func(), error) {
	if path == "" {
		return os.Stdout, func() {}, nil
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("create output file %q: %w", path, err)
	}
	return file, func() { _ = file.Close() }, nil
}
