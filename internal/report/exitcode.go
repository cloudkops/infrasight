package report

import "github.com/cloudkops/infrasight/internal/rule/parser"

// ExitCode computes a severity-based process exit code: 0 if no finding meets the
// fail-on threshold, otherwise 1/2/3 for the highest severity that does (LOW=1,
// MEDIUM=2, HIGH or CRITICAL=3).
func ExitCode(findings []Finding, failOn string) int {
	highest := 0
	for _, f := range findings {
		if !parser.Meets(f.Severity, failOn) {
			continue
		}
		r, err := parser.RankOf(f.Severity)
		if err != nil {
			continue
		}
		code := severityRankToExitCode(r)
		if code > highest {
			highest = code
		}
	}
	return highest
}

func severityRankToExitCode(rank int) int {
	switch {
	case rank >= 3: // HIGH or CRITICAL
		return 3
	case rank == 2: // MEDIUM
		return 2
	case rank == 1: // LOW
		return 1
	default:
		return 0
	}
}
