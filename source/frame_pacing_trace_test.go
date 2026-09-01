package main

import (
	"testing"
	"time"
)

func TestFramePacingOutsideDuration(t *testing.T) {
	if got := framePacingOutsideDuration(20*time.Millisecond, 3*time.Millisecond, 5*time.Millisecond); got != 12*time.Millisecond {
		t.Fatalf("outside duration = %v, want 12ms", got)
	}
	if got := framePacingOutsideDuration(5*time.Millisecond, 3*time.Millisecond, 4*time.Millisecond); got != 0 {
		t.Fatalf("overlapping work produced negative outside duration: %v", got)
	}
}
