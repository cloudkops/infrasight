package parser

import "testing"

func TestValidateRule(t *testing.T) {
	cases := []struct {
		name    string
		rule    Rule
		wantErr bool
	}{
		{
			name:    "missing id",
			rule:    Rule{Title: "t", Severity: "HIGH", Condition: Condition{Field: "x"}},
			wantErr: true,
		},
		{
			name:    "missing title",
			rule:    Rule{ID: "r1", Severity: "HIGH", Condition: Condition{Field: "x"}},
			wantErr: true,
		},
		{
			name:    "invalid severity",
			rule:    Rule{ID: "r1", Title: "t", Severity: "SEVERE", Condition: Condition{Field: "x"}},
			wantErr: true,
		},
		{
			name:    "no condition set",
			rule:    Rule{ID: "r1", Title: "t", Severity: "HIGH"},
			wantErr: true,
		},
		{
			name:    "valid single condition",
			rule:    Rule{ID: "r1", Title: "t", Severity: "HIGH", Condition: Condition{Field: "x", Equals: 1}},
			wantErr: false,
		},
		{
			name: "valid compound conditions",
			rule: Rule{ID: "r1", Title: "t", Severity: "LOW", Conditions: []Condition{
				{Field: "a", Equals: 1}, {Field: "b", Equals: 2},
			}},
			wantErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRule(tc.rule)
			if tc.wantErr && err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateRules_DuplicateID(t *testing.T) {
	rules := []Rule{
		{ID: "r1", Title: "a", Severity: "LOW", Condition: Condition{Field: "x", Equals: 1}},
		{ID: "r1", Title: "b", Severity: "LOW", Condition: Condition{Field: "y", Equals: 2}},
	}
	if err := ValidateRules(rules); err == nil {
		t.Fatal("expected a duplicate-id error")
	}
}
