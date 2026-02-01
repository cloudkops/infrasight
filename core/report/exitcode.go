package report

func ExitCode(findings []Finding, failOn string) int {
	exitCode := 0
	for _, f := range findings {
		if !severityAllowed(f.Severity, failOn) {
			continue
		}
		switch f.Severity {
		case "CRITICAL":
			if exitCode < 3 {
				exitCode = 3
			}
		case "HIGH":
			if exitCode < 2 {
				exitCode = 2
			}
		case "MEDIUM":
			if exitCode < 1 {
				exitCode = 1
			}
		}
	}
	return exitCode
}

func severityAllowed(sev, failOn string) bool {
	order := map[string]int{
		"CRITICAL": 3,
		"HIGH":     2,
		"MEDIUM":   1,
		"LOW":      0,
	}
	return order[sev] >= order[failOn]
}
