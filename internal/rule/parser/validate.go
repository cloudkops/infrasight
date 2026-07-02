package parser

import "fmt"

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

	for _, cond := range r.Conditions {
		if cond.Field == "" {
			return fmt.Errorf("rule %q: conditions entry missing field", r.ID)
		}
	}
	for _, cond := range r.AnyOf {
		if cond.Field == "" {
			return fmt.Errorf("rule %q: any_of entry missing field", r.ID)
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
