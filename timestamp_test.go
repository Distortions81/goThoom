package main

import "testing"

func withoutConsoleTimestamps(t *testing.T) {
	t.Helper()
	original := gs.ConsoleTimestamps
	gs.ConsoleTimestamps = false
	t.Cleanup(func() { gs.ConsoleTimestamps = original })
}
