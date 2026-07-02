package app

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cloudkops/infrasight/internal/rule/parser"
)

// scanFlags carries every flag a scan subcommand can bind. Not every field applies
// to every provider (label/field selectors and kubeconfig are Kubernetes-specific);
// unused fields are simply left at their zero value for other providers.
type scanFlags struct {
	rulesFile  string
	rulesetDir string
	format     string
	failOn     string
	outputFile string
	verbose    bool
	exitOne    bool

	namespace     string
	allNamespaces bool
	labelSelector string
	fieldSelector string
	kubeconfig    string
	kubeContext   string
}

// newScanCmd builds one subcommand per registered provider — "scan kubernetes",
// "scan docker", etc. — by asking internal/provider.Manager, never by hardcoding a
// provider name here.
func newScanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan infrastructure for security and compliance issues",
	}
	for _, name := range manager.List() {
		cmd.AddCommand(newProviderScanCmd(name))
	}
	return cmd
}

func newProviderScanCmd(providerName string) *cobra.Command {
	f := &scanFlags{format: "table", failOn: parser.SeverityLow}

	cmd := &cobra.Command{
		Use:   providerName,
		Short: fmt.Sprintf("Scan %s resources", providerName),
		Run: func(cmd *cobra.Command, args []string) {
			os.Exit(dispatch(cmd, providerName, f))
		},
	}
	bindScanFlags(cmd, f)
	return cmd
}

func bindScanFlags(cmd *cobra.Command, f *scanFlags) {
	cmd.Flags().StringVarP(&f.rulesFile, "rules", "r", "", "Rules file path")
	cmd.Flags().StringVar(&f.rulesetDir, "ruleset", "", "Ruleset directory")
	cmd.Flags().StringVarP(&f.format, "format", "f", "table", "Output format: table, json, sarif, markdown")
	cmd.Flags().StringVar(&f.failOn, "fail-on", parser.SeverityLow, "Minimum severity that sets a non-zero exit code (LOW|MEDIUM|HIGH|CRITICAL)")
	cmd.Flags().StringVarP(&f.outputFile, "output", "o", "", "Write findings to file instead of stdout")
	cmd.Flags().BoolVarP(&f.verbose, "verbose", "v", false, "Print debug information to stderr")
	cmd.Flags().BoolVar(&f.exitOne, "exit-1-on-findings", false, "Exit 1 on any finding regardless of severity")

	cmd.Flags().StringVarP(&f.namespace, "namespace", "n", "", "Namespace (empty = all; Kubernetes)")
	cmd.Flags().BoolVarP(&f.allNamespaces, "all-namespaces", "A", false, "Scan all namespaces (Kubernetes)")
	cmd.Flags().StringVarP(&f.labelSelector, "label-selector", "l", "", "Label selector (Kubernetes)")
	cmd.Flags().StringVar(&f.fieldSelector, "field-selector", "", "Field selector (Kubernetes)")
	cmd.Flags().StringVar(&f.kubeconfig, "kubeconfig", "", "Path to kubeconfig file (Kubernetes)")
	cmd.Flags().StringVar(&f.kubeContext, "context", "", "Kubeconfig context to use (Kubernetes)")
}
