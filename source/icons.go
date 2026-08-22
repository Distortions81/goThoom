package main

import (
	"strings"
)

type genderIcon int

const (
	genderUnknown genderIcon = iota
	genderMale
	genderFemale
)

// genderFromString maps server-provided gender strings to an icon type.
func genderFromString(s string) genderIcon {
	g := strings.ToLower(strings.TrimSpace(s))
	switch g {
	case "male", "m":
		return genderMale
	case "female", "f":
		return genderFemale
	case "other", "unknown", "", "undisclosed", "n/a":
		fallthrough
	default:
		return genderUnknown
	}
}
