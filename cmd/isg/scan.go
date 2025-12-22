package isg

import (
	"github.com/cloudkops/infrasight/core/model"
	"github.com/cloudkops/infrasight/core/rules"
	"github.com/spf13/cobra"
)

var scan = &cobra.Command{
	Use:   "scan",
	Short: "Scan Workload",
	Long:  "Scan Workload",
	Run: func(cmd *cobra.Command, args []string) {
		w1 := []model.Workload{
			{
				Name: "App",
				Containers: []model.Container{
					{
						Name:       "nginx",
						Image:      "nginx:latest",
						User:       0,
						Privileged: false,
					},
				},
			},
			{
				Name: "DB",
				Containers: []model.Container{
					{
						Name:       "mysql",
						Image:      "mysql:latest",
						User:       1000,
						Privileged: true,
					},
				},
			},
		}

		r1 := []rules.Rule{
			{
				ID:       "ISG-SEC-001",
				Title:    "Container running as root",
				Severity: "HIGH",
				Condition: rules.Condition{
					Field:  "container.user",
					Equals: 0,
				},
			},
			{
				ID:       "ISG-SEC-002",
				Title:    "Container running with privileged mode",
				Severity: "HIGH",
				Condition: rules.Condition{
					Field:  "container.privileged",
					Equals: true,
				},
			},
		}

		f := rules.Evaluate(w1, r1)
		for _, v := range f {
			cmd.Printf("ID: %s, Title: %s, Severity: %s, Workload: %s, Container: %s, Field: %s \n", v.RuleID, v.Title, v.Severity, v.Workload, v.Container, v.Field)
		}
	},
}

func init() {
	rootCMD.AddCommand(scan)
}
