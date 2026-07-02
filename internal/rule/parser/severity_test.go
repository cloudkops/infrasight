package parser

import "testing"

func TestMeets(t *testing.T) {
	cases := []struct {
		severity, threshold string
		want                bool
	}{
		{"HIGH", "LOW", true},
		{"low", "HIGH", false},
		{"CRITICAL", "CRITICAL", true},
		{"MEDIUM", "HIGH", false},
		{"bogus", "LOW", false},
	}
	for _, tc := range cases {
		if got := Meets(tc.severity, tc.threshold); got != tc.want {
			t.Errorf("Meets(%q, %q) = %v, want %v", tc.severity, tc.threshold, got, tc.want)
		}
	}
}

func TestValidSeverity(t *testing.T) {
	if !ValidSeverity("high") {
		t.Error("expected 'high' to be valid (case-insensitive)")
	}
	if ValidSeverity("severe") {
		t.Error("expected 'severe' to be invalid")
	}
}
