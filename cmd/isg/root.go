package isg

import (
	"log"
	"os"

	"github.com/spf13/cobra"
)

var rootCMD = &cobra.Command{
	Use:   "infrasight [command]",
	Short: "Infrastructure visibility, security, and cost analysis tool by CloudKops",
	Long:  `Infrastructure visibility, security, and cost analysis tool by CloudKops.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("Welcome to InfraSight")
	},
}

func Execute() {
	err := rootCMD.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.kubespace.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	//rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
