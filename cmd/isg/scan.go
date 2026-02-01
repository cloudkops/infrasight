package isg

import (
	"log"
	"os"

	"github.com/cloudkops/infrasight/adapters/kubernetes"
	"github.com/cloudkops/infrasight/core/model"
	"github.com/cloudkops/infrasight/core/report"
	"github.com/cloudkops/infrasight/core/rules"
	"github.com/spf13/cobra"
)

var (
	rulesFlag   string // rules file path
	formatFlag  string
	k8sFlag     bool
	rulesetFlag string
	failOn      string
	ciEnabled   bool
)
var scan = &cobra.Command{
	Use:   "scan",
	Short: "Scan Workload",
	Long:  "Scan Workload",
	Run: func(cmd *cobra.Command, args []string) {
		var w1 []model.Workload
		var err error
		if k8sFlag {
			client, err := kubernetes.NewClient()
			if err != nil {
				cmd.Println(err)
				return
			}
			pods, err := kubernetes.PodList(client, "kube-system")
			if err != nil {
				cmd.Println(err)
				return
			}
			for _, p := range pods {
				pw := kubernetes.PodToWorkload(p)
				w1 = append(w1, pw)
			}
		}

		var r1 []rules.Rule

		if rulesetFlag != "" {
			r1, err = rules.LoadRuleset(rulesetFlag)
			if err != nil {
				cmd.Println(err)
				os.Exit(1)
			}
		} else {
			r1, err = rules.Load(rulesFlag)
			if err != nil {
				cmd.Println(err)
				os.Exit(1)
			}
		}
		f := rules.Evaluate(w1, r1)
		if ciEnabled || formatFlag == "json" {
			report.JsonFormat(f)
		} else {
			report.PrintTable(f)
		}
		code := report.ExitCode(f, failOn)
		log.Println("Exit code: ", code)
		os.Exit(code)
	},
}

func init() {
	rootCMD.AddCommand(scan)

	// Here you will define your flags and configuration settings.
	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	scan.Flags().StringVarP(&rulesFlag, "rules", "r", "", "Rules file path")
	scan.Flags().StringVarP(&formatFlag, "format", "f", "table", "Output format")
	scan.Flags().BoolVarP(&k8sFlag, "k8s", "", false, "Kubernetes workload")
	scan.Flags().StringVarP(&rulesetFlag, "ruleset", "", "", "Ruleset directory")
	scan.Flags().StringVarP(&failOn, "fail-on", "", "LOW", "Severity level to fail the scan")
	scan.Flags().BoolVarP(&ciEnabled, "ci", "", false, "CI mode")
}
