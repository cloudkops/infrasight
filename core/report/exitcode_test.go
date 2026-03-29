package report

import "testing"

func finding(severity string) Finding {
	return Finding{RuleID: "R1", Title: "t", Severity: severity, Workload: "w", Container: "c", Field: "f"}
}

func TestExitCode_NoFindings(t *testing.T) {
	code := ExitCode(nil, "LOW")
	if code != 0 {
		t.Errorf("expected 0 for no findings, got %d", code)
	}
}

func TestExitCode_BelowThreshold(t *testing.T) {
	findings := []Finding{finding("LOW"), finding("MEDIUM")}
	code := ExitCode(findings, "HIGH")
	if code != 0 {
		t.Errorf("expected 0 when all findings below threshold, got %d", code)
	}
}

func TestExitCode_AtThreshold(t *testing.T) {
	findings := []Finding{finding("HIGH")}
	code := ExitCode(findings, "HIGH")
	if code != 2 {
		t.Errorf("expected 2 for HIGH at HIGH threshold, got %d", code)
	}
}

func TestExitCode_AboveThreshold(t *testing.T) {
	findings := []Finding{finding("CRITICAL")}
	code := ExitCode(findings, "HIGH")
	if code != 3 {
		t.Errorf("expected 3 for CRITICAL, got %d", code)
	}
}

func TestExitCode_MaxSeverityWins(t *testing.T) {
	findings := []Finding{
		finding("LOW"),
		finding("HIGH"),
		finding("MEDIUM"),
		finding("CRITICAL"),
	}
	code := ExitCode(findings, "LOW")
	if code != 3 {
		t.Errorf("expected 3 (CRITICAL wins), got %d", code)
	}
}

func TestExitCode_MixedBelowAndAboveThreshold(t *testing.T) {
	findings := []Finding{
		finding("LOW"),    // below HIGH threshold → skipped
		finding("MEDIUM"), // below HIGH threshold → skipped
		finding("HIGH"),   // at threshold → included → code 2
	}
	code := ExitCode(findings, "HIGH")
	if code != 2 {
		t.Errorf("expected 2 (only HIGH counts), got %d", code)
	}
}

func TestExitCode_CaseInsensitiveThreshold(t *testing.T) {
	findings := []Finding{finding("HIGH")}
	code := ExitCode(findings, "high")
	if code != 2 {
		t.Errorf("expected 2 for case-insensitive threshold, got %d", code)
	}
}

func TestExitCode_CaseInsensitiveSeverity(t *testing.T) {
	findings := []Finding{finding("critical")}
	code := ExitCode(findings, "LOW")
	if code != 3 {
		t.Errorf("expected 3 for lowercase severity, got %d", code)
	}
}

func TestExitCode_SeverityRankValues(t *testing.T) {
	tests := []struct {
		severity string
		wantCode int
	}{
		{"LOW", 0},
		{"MEDIUM", 1},
		{"HIGH", 2},
		{"CRITICAL", 3},
	}
	for _, tt := range tests {
		code := ExitCode([]Finding{finding(tt.severity)}, "LOW")
		if code != tt.wantCode {
			t.Errorf("ExitCode with %q severity = %d, want %d", tt.severity, code, tt.wantCode)
		}
	}
}
