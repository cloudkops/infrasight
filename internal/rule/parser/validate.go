package parser

import (
	"fmt"
	"regexp"
)

// ValidateRule checks the structural invariants a Rule must satisfy after loading.
func ValidateRule(r Rule) error {
	if r.ID == "" {
		return fmt.Errorf("rule: missing id")
	}
	if r.Title == "" {
		return fmt.Errorf("rule %q: missing title", r.ID)
	}
	if !ValidSeverity(r.Severity) {
		return fmt.Errorf("rule %q: invalid severity %q", r.ID, r.Severity)
	}

	hasSingle := r.Condition.Field != ""
	hasAll := len(r.Conditions) > 0
	hasAny := len(r.AnyOf) > 0
	if !hasSingle && !hasAll && !hasAny {
		return fmt.Errorf("rule %q: must set condition, conditions, or any_of", r.ID)
	}

	if hasSingle {
		if err := validateCondition(r.ID, "condition", r.Condition); err != nil {
			return err
		}
	}
	for _, cond := range r.Conditions {
		if err := validateCondition(r.ID, "conditions entry", cond); err != nil {
			return err
		}
	}
	for _, cond := range r.AnyOf {
		if err := validateCondition(r.ID, "any_of entry", cond); err != nil {
			return err
		}
	}
	return nil
}

// validateCondition catches mistakes that would otherwise fail silently at
// evaluation time (a bad regex would just never match, per resolveField's
// "unknown field fails silently" precedent — a malformed pattern is not the same
// kind of thing and deserves a load-time error instead).
func validateCondition(ruleID, label string, cond Condition) error {
	if cond.Field == "" {
		return fmt.Errorf("rule %q: %s missing field", ruleID, label)
	}
	if cond.Regex != "" {
		if _, err := regexp.Compile(cond.Regex); err != nil {
			return fmt.Errorf("rule %q: %s has invalid regex %q: %w", ruleID, label, cond.Regex, err)
		}
	}
	return nil
}

// ValidateRules validates every rule, returning the first error encountered.
func ValidateRules(rules []Rule) error {
	seen := make(map[string]bool, len(rules))
	for _, r := range rules {
		if err := ValidateRule(r); err != nil {
			return err
		}
		if seen[r.ID] {
			return fmt.Errorf("rule %q: duplicate rule id", r.ID)
		}
		seen[r.ID] = true
	}
	return nil
}
