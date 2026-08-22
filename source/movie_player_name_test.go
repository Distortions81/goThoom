package main

import "testing"

func TestValidMoviePlayerName(t *testing.T) {
	valid := []string{"Gaia O'Neill", "Anne-Marie", "Éowyn"}
	for _, name := range valid {
		if got := validMoviePlayerName(name); got != name {
			t.Errorf("validMoviePlayerName(%q) = %q", name, got)
		}
	}

	invalid := []string{"", "\x00broken", "\x01\x02", "name\nother", "###", "This Player Name Is Deliberately Longer Than Forty Eight Characters"}
	for _, name := range invalid {
		if got := validMoviePlayerName(name); got != "" {
			t.Errorf("validMoviePlayerName(%q) = %q, want empty", name, got)
		}
	}
}
