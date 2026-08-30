package main

import (
	"testing"
	"time"
)

func TestToolbarMinimumWidthCoversDockedControls(t *testing.T) {
	const controlsWidth = 84 + 5*68
	const hostChrome = 16
	if dockedToolbarMinimumWidth < controlsWidth+hostChrome {
		t.Fatalf("toolbar minimum width = %v, need at least %d", dockedToolbarMinimumWidth, controlsWidth+hostChrome)
	}
}

func TestFormatToolbarLatencyUsesOneDecimalPlace(t *testing.T) {
	if got := formatToolbarLatency(1234567 * time.Nanosecond); got != "1.2ms" {
		t.Fatalf("formatted latency = %q, want %q", got, "1.2ms")
	}
}

func TestFormatToolbarLossSuppressesZeroDecimal(t *testing.T) {
	if got := formatToolbarLoss(0); got != "0%" {
		t.Fatalf("zero loss = %q, want 0%%", got)
	}
	if got := formatToolbarLoss(0.3); got != "0.3%" {
		t.Fatalf("nonzero loss = %q, want 0.3%%", got)
	}
}
