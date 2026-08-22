package main

import "strings"

// hasClan returns true when the provided clan string represents a real clan
// name. It treats common placeholder values as empty (e.g., "-", "—",
// "none", "no clan").
func hasClan(s string) bool {
	c := strings.TrimSpace(s)
	if c == "" {
		return false
	}
	lc := strings.ToLower(c)
	// Treat numeric zero (e.g., "(0)", "0") as no clan as seen in some
	// persisted files or server outputs.
	trimmed := strings.Trim(lc, " [](){}")
	if trimmed != "" {
		// If it's all zeros (e.g., "0", "00"), treat as no clan.
		allZero := true
		for i := 0; i < len(trimmed); i++ {
			if trimmed[i] != '0' {
				allZero = false
				break
			}
		}
		if allZero {
			return false
		}
	}
	switch lc {
	case "-", "—", "none", "no clan", "n/a", "na":
		return false
	default:
		return true
	}
}

// sameRealClan reports whether a and b are the same non-empty clan, ignoring
// case and treating placeholder values as empty.
func sameRealClan(a, b string) bool {
	if !hasClan(a) || !hasClan(b) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}
