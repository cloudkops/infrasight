package rules

import "fmt"

func ValidateRule(r Rule) error {
	if r.ID == "" {
		return fmt.Errorf("rule id is required")
	}
	if r.Title == "" {
		return fmt.Errorf("rule title is required")
	}
	if !IsValidSeverity(NormalizeSeverity(r.Severity)) {
		return fmt.Errorf("invalid severity: %s (allowed: LOW|MEDIUM|HIGH|CRITICAL)", r.Severity)
	}

	hasCompound := len(r.Conditions) > 0 || len(r.AnyOf) > 0
	hasSingle := r.Condition.Field != ""

	if !hasSingle && !hasCompound {
		return fmt.Errorf("a rule must have condition, conditions, or any_of")
	}

	for i, c := range r.Conditions {
		if c.Field == "" {
			return fmt.Errorf("conditions[%d].field is required", i)
		}
	}
	for i, c := range r.AnyOf {
		if c.Field == "" {
			return fmt.Errorf("any_of[%d].field is required", i)
		}
	}

	return nil
}

func ValidateRules(rules []Rule) error {
	for _, r := range rules {
		if err := ValidateRule(r); err != nil {
			return fmt.Errorf("rule %s: %w", r.ID, err)
		}
	}
	return nil
}
