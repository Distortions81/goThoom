package main

import (
	"testing"
	"time"
)

func TestFormatToolbarLatencyUsesOneDecimalPlace(t *testing.T) {
	if got := formatToolbarLatency(1234567 * time.Nanosecond); got != "1.2ms" {
		t.Fatalf("formatted latency = %q, want %q", got, "1.2ms")
	}
}

func TestFormatToolbarPNAStatus(t *testing.T) {
	resetPNAFallback()
	t.Cleanup(resetPNAFallback)
	if got := formatToolbarPNAStatus(false, 5, 50*time.Millisecond, true, 0); got != "PNA off" {
		t.Fatalf("disabled PNA status = %q, want PNA off", got)
	}
	if got := formatToolbarPNAStatus(true, 5, 50*time.Millisecond, true, 0); got != "PNA lead50ms@5Hz" {
		t.Fatalf("active PNA status = %q, want PNA lead50ms@5Hz", got)
	}
	if got := formatToolbarPNAStatus(true, 5, 0, false, 0); got != "PNA learning" {
		t.Fatalf("warming PNA status = %q, want PNA learning", got)
	}
	if got := formatToolbarPNAStatus(true, 5, 50*time.Millisecond, true, 1); got != "PNA paused" {
		t.Fatalf("fallback PNA status = %q, want PNA paused", got)
	}
}
