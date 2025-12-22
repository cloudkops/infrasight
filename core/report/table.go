package report

import "fmt"

func PrintTable(findings []Finding) {
	fmt.Printf("\n%-15s %-10s %-12s %-15s %-20s\n",
		"RULE", "SEVERITY", "WORKLOAD", "CONTAINER", "FIELD")

	for _, f := range findings {
		fmt.Printf("%-15s %-10s %-12s %-15s %-20s\n",
			f.RuleID, f.Severity, f.Workload, f.Container, f.Field)
	}
}
