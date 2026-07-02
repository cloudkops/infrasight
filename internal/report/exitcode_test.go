package report

import "testing"

func TestExitCode(t *testing.T) {
	cases := []struct {
		name     string
		findings []Finding
		failOn   string
		want     int
	}{
		{"no findings", nil, "LOW", 0},
		{"low below high threshold", []Finding{{Severity: "LOW"}}, "HIGH", 0},
		{"low meets low threshold", []Finding{{Severity: "LOW"}}, "LOW", 1},
		{"medium meets low threshold", []Finding{{Severity: "MEDIUM"}}, "LOW", 2},
		{"high meets low threshold", []Finding{{Severity: "HIGH"}}, "LOW", 3},
		{"critical meets low threshold", []Finding{{Severity: "CRITICAL"}}, "LOW", 3},
		{
			"highest of mixed findings wins",
			[]Finding{{Severity: "LOW"}, {Severity: "CRITICAL"}, {Severity: "MEDIUM"}},
			"LOW", 3,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExitCode(tc.findings, tc.failOn); got != tc.want {
				t.Errorf("ExitCode() = %d, want %d", got, tc.want)
			}
		})
	}
}
