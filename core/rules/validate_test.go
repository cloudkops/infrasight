package rules

import "testing"

func validRule() Rule {
	return Rule{
		ID:       "ISG-TEST-001",
		Title:    "Test rule",
		Severity: "HIGH",
		Condition: Condition{
			Field:  "container.user",
			Equals: 0,
		},
	}
}

func TestValidateRule_Valid(t *testing.T) {
	severities := []string{"LOW", "MEDIUM", "HIGH", "CRITICAL", "low", "high"}
	for _, s := range severities {
		r := validRule()
		r.Severity = s
		if err := ValidateRule(r); err != nil {
			t.Errorf("expected valid rule with severity %q, got error: %v", s, err)
		}
	}
}

func TestValidateRule_MissingID(t *testing.T) {
	r := validRule()
	r.ID = ""
	if err := ValidateRule(r); err == nil {
		t.Error("expected error for missing ID, got nil")
	}
}

func TestValidateRule_MissingTitle(t *testing.T) {
	r := validRule()
	r.Title = ""
	if err := ValidateRule(r); err == nil {
		t.Error("expected error for missing Title, got nil")
	}
}

func TestValidateRule_InvalidSeverity(t *testing.T) {
	invalid := []string{"", "BLOCKER", "NONE", "info", "warn", "urgent"}
	for _, s := range invalid {
		r := validRule()
		r.Severity = s
		if err := ValidateRule(r); err == nil {
			t.Errorf("expected error for invalid severity %q, got nil", s)
		}
	}
}

func TestValidateRule_MissingConditionField(t *testing.T) {
	r := validRule()
	r.Condition.Field = ""
	if err := ValidateRule(r); err == nil {
		t.Error("expected error for missing condition.field, got nil")
	}
}

func TestValidateRules_Empty(t *testing.T) {
	if err := ValidateRules(nil); err != nil {
		t.Errorf("expected no error for empty rules, got: %v", err)
	}
}

func TestValidateRules_AllValid(t *testing.T) {
	rules := []Rule{
		{ID: "R1", Title: "Rule 1", Severity: "LOW", Condition: Condition{Field: "container.user"}},
		{ID: "R2", Title: "Rule 2", Severity: "CRITICAL", Condition: Condition{Field: "container.privileged"}},
	}
	if err := ValidateRules(rules); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateRules_OneInvalid(t *testing.T) {
	rules := []Rule{
		{ID: "R1", Title: "Rule 1", Severity: "LOW", Condition: Condition{Field: "container.user"}},
		{ID: "", Title: "Rule 2", Severity: "HIGH", Condition: Condition{Field: "container.privileged"}}, // invalid: no ID
	}
	if err := ValidateRules(rules); err == nil {
		t.Error("expected error for invalid rule in slice, got nil")
	}
}

func TestValidateRules_ErrorWrapsRuleID(t *testing.T) {
	rules := []Rule{
		{ID: "BAD-RULE", Title: "", Severity: "HIGH", Condition: Condition{Field: "container.user"}},
	}
	err := ValidateRules(rules)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); len(got) == 0 {
		t.Error("expected non-empty error message")
	}
}
