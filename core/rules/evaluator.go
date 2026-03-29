package rules

import (
	"fmt"
	"strings"

	"github.com/cloudkops/infrasight/core/model"
	"github.com/cloudkops/infrasight/core/report"
)

func Evaluate(workloads []model.Workload, rules []Rule) []report.Finding {
	var findings []report.Finding
	for _, w := range workloads {
		for _, c := range w.Containers {
			for _, r := range rules {
				if matchRule(w, c, r) {
					findings = append(findings, report.Finding{
						RuleID:      r.ID,
						Title:       r.Title,
						Severity:    r.Severity,
						Workload:    w.Name,
						Container:   c.Name,
						Field:       activeField(r),
						Description: r.Description,
						Category:    r.Category,
						DocsUrl:     r.DocsUrl,
						Remediation: r.Remediation,
					})
				}
			}
		}
	}
	return findings
}

func matchRule(w model.Workload, c model.Container, r Rule) bool {
	if len(r.Conditions) > 0 {
		for _, cond := range r.Conditions {
			if !evaluateCondition(w, c, cond) {
				return false
			}
		}
		return true
	}
	if len(r.AnyOf) > 0 {
		for _, cond := range r.AnyOf {
			if evaluateCondition(w, c, cond) {
				return true
			}
		}
		return false
	}
	return evaluateCondition(w, c, r.Condition)
}

func activeField(r Rule) string {
	if r.Condition.Field != "" {
		return r.Condition.Field
	}
	if len(r.Conditions) > 0 {
		return r.Conditions[0].Field
	}
	if len(r.AnyOf) > 0 {
		return r.AnyOf[0].Field
	}
	return ""
}

func evaluateCondition(w model.Workload, c model.Container, cond Condition) bool {
	switch {
	case cond.Field == "container.user":
		return matchInt64(c.User, cond)
	case cond.Field == "container.privileged":
		return matchBool(c.Privileged, cond)
	case cond.Field == "container.image":
		return matchString(c.Image, cond)
	case cond.Field == "container.name":
		return matchString(c.Name, cond)
	case cond.Field == "workload.name":
		return matchString(w.Name, cond)
	case cond.Field == "workload.namespace":
		return matchString(w.Namespace, cond)
	case cond.Field == "workload.platform":
		return matchString(w.Platform, cond)
	case cond.Field == "workload.type":
		return matchString(w.Type, cond)
	case cond.Field == "container.resources.cpu_limit":
		return matchResourceValueOrExistence(c.Resources.CPULimit, cond)
	case cond.Field == "container.resources.memory_limit":
		return matchResourceValueOrExistence(c.Resources.MemoryLimit, cond)
	case cond.Field == "container.resources.cpu_request":
		return matchResourceValueOrExistence(c.Resources.CPURequest, cond)
	case cond.Field == "container.resources.memory_request":
		return matchResourceValueOrExistence(c.Resources.MemoryRequest, cond)
	case cond.Field == "container.exposure.hostNetwork":
		expected, ok := asBool(cond.Equals)
		return ok && hasExposureType(c.Exposures, "hostNetwork") == expected
	case cond.Field == "container.exposure.publicIP":
		expected, ok := asBool(cond.Equals)
		return ok && hasExposureType(c.Exposures, "publicIP") == expected
	case cond.Field == "container.exposure.NodePort":
		expected, ok := asBool(cond.Equals)
		return ok && hasExposureType(c.Exposures, "NodePort") == expected
	case strings.HasPrefix(cond.Field, "workload.labels."):
		key := strings.TrimPrefix(cond.Field, "workload.labels.")
		if key == "" {
			return false
		}
		return matchString(w.Metadata.Labels[key], cond)
	case strings.HasPrefix(cond.Field, "workload.annotations."):
		key := strings.TrimPrefix(cond.Field, "workload.annotations.")
		if key == "" {
			return false
		}
		return matchString(w.Metadata.Annotations[key], cond)
	default:
		return false
	}
}

func matchString(actual string, cond Condition) bool {
	if cond.NotEquals != nil {
		expected, ok := asString(cond.NotEquals)
		return ok && actual != expected
	}
	if cond.Contains != "" {
		return strings.Contains(actual, cond.Contains)
	}
	expected, ok := asString(cond.Equals)
	return ok && actual == expected
}

func matchInt64(actual int64, cond Condition) bool {
	if cond.NotEquals != nil {
		expected, ok := asInt64(cond.NotEquals)
		return ok && actual != expected
	}
	expected, ok := asInt64(cond.Equals)
	return ok && actual == expected
}

func matchBool(actual bool, cond Condition) bool {
	if cond.NotEquals != nil {
		expected, ok := asBool(cond.NotEquals)
		return ok && actual != expected
	}
	expected, ok := asBool(cond.Equals)
	return ok && actual == expected
}

func matchResourceValueOrExistence(actual string, cond Condition) bool {
	if cond.Equals == nil && cond.NotEquals == nil && cond.Contains == "" {
		exists := actual != ""
		return exists == cond.Exists
	}
	return matchString(actual, cond)
}

func hasExposureType(exposures []model.Exposure, exposureType string) bool {
	for _, e := range exposures {
		if e.Type == exposureType {
			return true
		}
	}
	return false
}

func asInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		return int64(n), true
	case uint64:
		return int64(n), true
	default:
		return 0, false
	}
}

func asBool(v interface{}) (bool, bool) {
	b, ok := v.(bool)
	return b, ok
}

func asString(v interface{}) (string, bool) {
	switch s := v.(type) {
	case string:
		return s, true
	case int:
		return fmt.Sprintf("%d", s), true
	case int64:
		return fmt.Sprintf("%d", s), true
	case float64:
		return fmt.Sprintf("%v", s), true
	default:
		return "", false
	}
}
