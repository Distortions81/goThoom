package main

import (
	"testing"
	"time"
)

func TestToolbarMinimumWidthCoversDockedControls(t *testing.T) {
	const dockedControlsWidth = 84 + 3*(84+4)
	const dockedWindowPadding = 8
	if dockedToolbarMinimumWidth < dockedControlsWidth+dockedWindowPadding {
		t.Fatalf("toolbar minimum width = %v, need at least %d", dockedToolbarMinimumWidth, dockedControlsWidth+dockedWindowPadding)
	}
	const floatingControlsWidth = 84 + 3*(88+4)
	const floatingWindowPadding = 8
	if floatingToolbarMinimumWidth < floatingControlsWidth+floatingWindowPadding {
		t.Fatalf("floating toolbar minimum width = %v, need at least %d", floatingToolbarMinimumWidth, floatingControlsWidth+floatingWindowPadding)
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
