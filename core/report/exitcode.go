package report

import "strings"

var severityRank = map[string]int{
	"LOW":      0,
	"MEDIUM":   1,
	"HIGH":     2,
	"CRITICAL": 3,
}

func normalizeSeverity(s string) string {
	return strings.ToUpper(s)
}

func isSeverityAtLeast(sev, threshold string) bool {
	return severityRank[normalizeSeverity(sev)] >= severityRank[normalizeSeverity(threshold)]
}

func ExitCode(findings []Finding, failOn string) int {
	exitCode := 0

	for _, f := range findings {
		if !isSeverityAtLeast(f.Severity, failOn) {
			continue
		}

		code := severityRank[normalizeSeverity(f.Severity)]
		if code > exitCode {
			exitCode = code
		}
	}
	return exitCode
}
