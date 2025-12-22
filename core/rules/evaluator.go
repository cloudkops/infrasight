package rules

import (
	"github.com/cloudkops/infrasight/core/model"
	"github.com/cloudkops/infrasight/core/report"
)

func Evaluate(workload []model.Workload, rules []Rule) []report.Finding {
	var findings []report.Finding
	for _, w := range workload {
		for _, c := range w.Containers {
			for _, r := range rules {
				switch r.Condition.Field {
				case "container.user":
					expected := int64(r.Condition.Equals.(int))
					if c.User == expected {
						findings = append(findings, report.Finding{
							RuleID:    r.ID,
							Title:     r.Title,
							Severity:  r.Severity,
							Workload:  w.Name,
							Container: c.Name,
							Field:     r.Condition.Field,
						})
					}
				case "container.privileged":
					expected := r.Condition.Equals.(bool)
					if c.Privileged == expected {
						findings = append(findings, report.Finding{
							RuleID:    r.ID,
							Title:     r.Title,
							Severity:  r.Severity,
							Workload:  w.Name,
							Container: c.Name,
							Field:     r.Condition.Field,
						})
					}
				}
			}
		}
	}
	return findings
}
