package main

import (
	"testing"
	"time"
)

func TestWindowFocusPolledAtMostEveryHalfSecond(t *testing.T) {
	originalFocused := cachedWindowFocused.Load()
	originalNextPoll := nextWindowFocusPoll
	defer func() {
		cachedWindowFocused.Store(originalFocused)
		nextWindowFocusPoll = originalNextPoll
	}()

	nextWindowFocusPoll = time.Time{}
	reads := 0
	focused := true
	read := func() bool {
		reads++
		return focused
	}
	start := time.Unix(100, 0)

	if !pollWindowFocusWith(start, read) || reads != 1 {
		t.Fatalf("first poll = focused %v, reads %d; want true, 1", windowIsFocused(), reads)
	}
	focused = false
	if !pollWindowFocusWith(start.Add(499*time.Millisecond), read) || reads != 1 {
		t.Fatalf("cached poll = focused %v, reads %d; want true, 1", windowIsFocused(), reads)
	}
	if pollWindowFocusWith(start.Add(500*time.Millisecond), read) || reads != 2 {
		t.Fatalf("half-second poll = focused %v, reads %d; want false, 2", windowIsFocused(), reads)
	}
}
