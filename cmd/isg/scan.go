package isg

import (
	"os"

	"github.com/spf13/cobra"
)

func newScanCommand() *cobra.Command {
	opts := defaultScanOptions()

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan workloads for security and compliance issues",
		Long:  "Scan workloads from supported sources (Kubernetes, file). Use subcommands for explicit source selection.",
		Run: func(cmd *cobra.Command, args []string) {
			os.Exit(runScan(cmd, opts))
		},
	}

	bindScanFlags(cmd, opts)
	cmd.AddCommand(newScanK8sCommand())
	return cmd
}

func newScanK8sCommand() *cobra.Command {
	opts := defaultScanOptions()
	opts.source = scanSourceK8s

	cmd := &cobra.Command{
		Use:   "k8s",
		Short: "Scan Kubernetes workloads (Pods, Deployments, Services)",
		Run: func(cmd *cobra.Command, args []string) {
			os.Exit(runScan(cmd, opts))
		},
	}

	bindScanFlags(cmd, opts)
	return cmd
}

func init() {
	rootCMD.AddCommand(newScanCommand())
}
