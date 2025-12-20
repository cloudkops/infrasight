package isg

import "github.com/spf13/cobra"

var versionCmd = &cobra.Command{
	Use: "version",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Println("0.0.1")
	},
}

func init() {
	rootCMD.AddCommand(versionCmd)
}
