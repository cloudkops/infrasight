package app

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/cloudkops/infrasight/internal/provider"
)

var manager = provider.NewManager()

// Execute is the CLI Layer's single entrypoint — cmd/infrasight/main.go calls this
// and nothing else. Command construction happens here, not in an init(), so every
// provider registered by its own package init() (triggered by main.go's blank
// imports) is already visible to manager.List() by the time commands are built.
func Execute() {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "infrasight",
		Short: "Infrastructure visibility, security, and cost analysis tool by CloudKops",
	}
	root.AddCommand(newScanCmd())
	root.AddCommand(newProvidersCmd())
	root.AddCommand(newVersionCmd())
	return root
}
