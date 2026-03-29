package rules

import "strings"

const (
	SeverityLow      = "LOW"
	SeverityMedium   = "MEDIUM"
	SeverityHigh     = "HIGH"
	SeverityCritical = "CRITICAL"
)

var SeverityRank = map[string]int{
	SeverityLow:      0,
	SeverityMedium:   1,
	SeverityHigh:     2,
	SeverityCritical: 3,
}

func NormalizeSeverity(s string) string {
	return strings.ToUpper(s)
}

func IsSeverityAtLeast(sev, threshold string) bool {
	return SeverityRank[NormalizeSeverity(sev)] >= SeverityRank[NormalizeSeverity(threshold)]
}

func IsValidSeverity(severity string) bool {
	_, ok := SeverityRank[NormalizeSeverity(severity)]
	return ok
}
