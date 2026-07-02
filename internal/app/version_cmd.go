package app

import "github.com/spf13/cobra"

const version = "0.1.0"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the infrasight version",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println(version)
		},
	}
}
