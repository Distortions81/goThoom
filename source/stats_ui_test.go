package main

import (
	"testing"
	"time"
)

func TestStatsHistoryDuration(t *testing.T) {
	if got := time.Duration(statsHistorySize) * statsSampleInterval; got != statsHistoryDuration {
		t.Fatalf("stats history duration = %s, want %s", got, statsHistoryDuration)
	}
}

func TestStatsHistoryKeepsNewestFiveMinutes(t *testing.T) {
	originalHistory := statsHistory
	originalCount := statsHistoryCount
	originalNext := statsHistoryNext
	t.Cleanup(func() {
		statsHistory = originalHistory
		statsHistoryCount = originalCount
		statsHistoryNext = originalNext
	})

	statsHistory = [statsHistorySize]liveStatsSample{}
	statsHistoryCount = 0
	statsHistoryNext = 0
	for i := 0; i < statsHistorySize+2; i++ {
		appendStatsSample(liveStatsSample{fps: float64(i)})
	}

	samples := statsSamples()
	if len(samples) != statsHistorySize {
		t.Fatalf("stats history length = %d, want %d", len(samples), statsHistorySize)
	}
	if samples[0].fps != 2 || samples[len(samples)-1].fps != float64(statsHistorySize+1) {
		t.Fatalf("stats history range = %.0f..%.0f, want 2..%d", samples[0].fps, samples[len(samples)-1].fps, statsHistorySize+1)
	}
}
