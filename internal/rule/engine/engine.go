package engine

import (
	"github.com/cloudkops/infrasight/internal/report"
	"github.com/cloudkops/infrasight/internal/resource"
	"github.com/cloudkops/infrasight/internal/rule/evaluator"
	"github.com/cloudkops/infrasight/internal/rule/parser"
)

// Evaluate ties parser-loaded rules and evaluator condition matching together: for
// every resource x container x rule, it checks whether the rule matches and
// produces a Finding. Called by internal/scan/engine, never directly by a provider
// or the CLI.
func Evaluate(resources []resource.Resource, rules []parser.Rule) []report.Finding {
	var findings []report.Finding
	for _, r := range resources {
		containers := r.Runtime.Containers
		if len(containers) == 0 {
			// Resources with no container/process concept (a bare host, a
			// Terraform resource) still evaluate once against resource-level
			// fields via a synthetic empty container.
			containers = []resource.Container{{}}
		}

		for _, c := range containers {
			for _, rule := range rules {
				if !matchRule(r, c, rule) {
					continue
				}
				findings = append(findings, report.Finding{
					RuleID:        rule.ID,
					Title:         rule.Title,
					Severity:      rule.Severity,
					Category:      rule.Category,
					Description:   rule.Description,
					Remediation:   rule.Remediation,
					DocsUrl:       rule.DocsUrl,
					ResourceName:  r.Name,
					ResourceKind:  r.Kind,
					Namespace:     r.Namespace,
					ContainerName: c.Name,
					Field:         activeField(rule),
				})
			}
		}
	}
	return findings
}

func matchRule(r resource.Resource, c resource.Container, rule parser.Rule) bool {
	if len(rule.Conditions) > 0 {
		for _, cond := range rule.Conditions {
			if !evaluator.Match(r, c, cond) {
				return false
			}
		}
		return true
	}
	if len(rule.AnyOf) > 0 {
		for _, cond := range rule.AnyOf {
			if evaluator.Match(r, c, cond) {
				return true
			}
		}
		return false
	}
	return evaluator.Match(r, c, rule.Condition)
}

func activeField(rule parser.Rule) string {
	if rule.Condition.Field != "" {
		return rule.Condition.Field
	}
	if len(rule.Conditions) > 0 {
		return rule.Conditions[0].Field
	}
	if len(rule.AnyOf) > 0 {
		return rule.AnyOf[0].Field
	}
	return ""
}
