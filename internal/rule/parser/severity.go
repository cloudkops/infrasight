package parser

import (
	"fmt"
	"strings"
)

const (
	SeverityLow      = "LOW"
	SeverityMedium   = "MEDIUM"
	SeverityHigh     = "HIGH"
	SeverityCritical = "CRITICAL"
)

var rank = map[string]int{
	SeverityLow:      1,
	SeverityMedium:   2,
	SeverityHigh:     3,
	SeverityCritical: 4,
}

// NormalizeSeverity upper-cases a severity string for case-insensitive comparison.
func NormalizeSeverity(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

// ValidSeverity reports whether s (case-insensitive) is one of the four levels.
func ValidSeverity(s string) bool {
	_, ok := rank[NormalizeSeverity(s)]
	return ok
}

// RankOf returns a severity's rank (higher = more severe), erroring on an unknown
// level rather than silently ranking it as zero.
func RankOf(s string) (int, error) {
	r, ok := rank[NormalizeSeverity(s)]
	if !ok {
		return 0, fmt.Errorf("severity: unknown level %q", s)
	}
	return r, nil
}

// Meets reports whether severity s is at or above the threshold.
func Meets(s, threshold string) bool {
	sr, err := RankOf(s)
	if err != nil {
		return false
	}
	tr, err := RankOf(threshold)
	if err != nil {
		return false
	}
	return sr >= tr
}
