package rules

import "testing"

func TestNormalizeSeverity(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"low", "LOW"},
		{"HIGH", "HIGH"},
		{"Critical", "CRITICAL"},
		{"medium", "MEDIUM"},
		{"", ""},
	}
	for _, tt := range tests {
		got := NormalizeSeverity(tt.in)
		if got != tt.want {
			t.Errorf("NormalizeSeverity(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestIsValidSeverity(t *testing.T) {
	valid := []string{"LOW", "MEDIUM", "HIGH", "CRITICAL", "low", "medium", "high", "critical", "High"}
	for _, s := range valid {
		if !IsValidSeverity(s) {
			t.Errorf("IsValidSeverity(%q) = false, want true", s)
		}
	}

	invalid := []string{"", "NONE", "BLOCKER", "info", "warn", "0", "unknown"}
	for _, s := range invalid {
		if IsValidSeverity(s) {
			t.Errorf("IsValidSeverity(%q) = true, want false", s)
		}
	}
}

func TestIsSeverityAtLeast(t *testing.T) {
	tests := []struct {
		sev, threshold string
		want           bool
	}{
		// same level
		{"LOW", "LOW", true},
		{"MEDIUM", "MEDIUM", true},
		{"HIGH", "HIGH", true},
		{"CRITICAL", "CRITICAL", true},
		// above threshold
		{"MEDIUM", "LOW", true},
		{"HIGH", "LOW", true},
		{"HIGH", "MEDIUM", true},
		{"CRITICAL", "LOW", true},
		{"CRITICAL", "MEDIUM", true},
		{"CRITICAL", "HIGH", true},
		// below threshold
		{"LOW", "MEDIUM", false},
		{"LOW", "HIGH", false},
		{"LOW", "CRITICAL", false},
		{"MEDIUM", "HIGH", false},
		{"MEDIUM", "CRITICAL", false},
		{"HIGH", "CRITICAL", false},
		// case-insensitive
		{"high", "medium", true},
		{"low", "HIGH", false},
		{"critical", "low", true},
	}
	for _, tt := range tests {
		got := IsSeverityAtLeast(tt.sev, tt.threshold)
		if got != tt.want {
			t.Errorf("IsSeverityAtLeast(%q, %q) = %v, want %v", tt.sev, tt.threshold, got, tt.want)
		}
	}
}

func TestSeverityRankOrder(t *testing.T) {
	order := []string{SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical}
	for i := 1; i < len(order); i++ {
		if SeverityRank[order[i]] <= SeverityRank[order[i-1]] {
			t.Errorf("SeverityRank[%q] should be > SeverityRank[%q]", order[i], order[i-1])
		}
	}
}
