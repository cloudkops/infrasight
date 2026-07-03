package engine

import (
	"strings"

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
		hasContainers := len(containers) > 0
		if !hasContainers {
			// Resources with no container/process concept (a bare host, a
			// Terraform resource, a Kubernetes ConfigMap/Secret/Ingress/
			// NetworkPolicy) still evaluate once against resource-level fields
			// via a synthetic empty container. Rules that reference a
			// container-scoped field are skipped below for these resources —
			// otherwise "container.resources.cpu_limit exists: false" would
			// spuriously match every containerless resource, since the field
			// is just as absent on the synthetic container as it would be on
			// a genuinely under-provisioned one.
			containers = []resource.Container{{}}
		}

		for _, c := range containers {
			for _, rule := range rules {
				if !hasContainers && ruleReferencesContainerField(rule) {
					continue
				}
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

// ruleReferencesContainerField reports whether any condition on rule targets a
// container-scoped field ("container.*", or "container_kind" which only exists
// on init/ephemeral containers). Checked against the rule as a whole, not
// per-condition — conditions (AND) and any_of (OR) that mix a container field
// with a resource field don't exist in any shipped ruleset today, and skipping
// the whole rule is the simpler, safer behavior until that combination is
// actually needed.
func ruleReferencesContainerField(rule parser.Rule) bool {
	if isContainerScopedField(rule.Condition.Field) {
		return true
	}
	for _, cond := range rule.Conditions {
		if isContainerScopedField(cond.Field) {
			return true
		}
	}
	for _, cond := range rule.AnyOf {
		if isContainerScopedField(cond.Field) {
			return true
		}
	}
	return false
}

func isContainerScopedField(field string) bool {
	return strings.HasPrefix(field, "container.") || field == "container_kind"
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
