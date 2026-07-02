package app

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newProvidersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "providers",
		Short: "List and inspect registered providers",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List every registered provider",
		Run: func(cmd *cobra.Command, args []string) {
			for _, name := range manager.List() {
				p, err := manager.Get(name)
				if err != nil {
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", p.Name(), p.Kind())
			}
		},
	})
	return cmd
}
