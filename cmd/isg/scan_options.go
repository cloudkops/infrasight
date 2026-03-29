package isg

import "github.com/spf13/cobra"

type scanSource string

const (
	scanSourceAuto scanSource = "auto"
	scanSourceK8s  scanSource = "k8s"
)

type scanOptions struct {
	rulesFlag     string
	formatFlag    string
	k8sFlag       bool
	namespace     string
	allNamespaces bool
	source        scanSource
	rulesetFlag   string
	failOn        string
	ciEnabled     bool
	outputFile    string
	verbose       bool
	exitOne       bool
}

func defaultScanOptions() *scanOptions {
	return &scanOptions{
		rulesFlag:     "",
		formatFlag:    "table",
		k8sFlag:       false,
		namespace:     "",
		allNamespaces: false,
		source:        scanSourceAuto,
		rulesetFlag:   "",
		failOn:        "LOW",
		ciEnabled:     false,
		outputFile:    "",
		verbose:       false,
		exitOne:       false,
	}
}

func bindScanFlags(cmd *cobra.Command, opts *scanOptions) {
	cmd.Flags().StringVarP(&opts.rulesFlag, "rules", "r", "", "Rules file path")
	cmd.Flags().StringVarP(&opts.rulesetFlag, "ruleset", "", "", "Ruleset directory")
	cmd.Flags().StringVarP(&opts.formatFlag, "format", "f", "table", "Output format: table, json, sarif")
	cmd.Flags().StringVarP(&opts.failOn, "fail-on", "", "LOW", "Minimum severity that sets a non-zero exit code (LOW|MEDIUM|HIGH|CRITICAL)")
	cmd.Flags().StringVarP(&opts.outputFile, "output", "o", "", "Write findings to file instead of stdout")
	cmd.Flags().BoolVarP(&opts.k8sFlag, "k8s", "", false, "Scan Kubernetes workloads (umbrella shorthand)")
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "", "Kubernetes namespace (empty = all)")
	cmd.Flags().BoolVarP(&opts.allNamespaces, "all-namespaces", "A", false, "Scan all Kubernetes namespaces")
	cmd.Flags().BoolVarP(&opts.ciEnabled, "ci", "", false, "CI mode: forces JSON output")
	cmd.Flags().BoolVarP(&opts.verbose, "verbose", "v", false, "Print debug information to stderr")
	cmd.Flags().BoolVarP(&opts.exitOne, "exit-1-on-findings", "", false, "Exit 1 on any finding regardless of severity")
}
