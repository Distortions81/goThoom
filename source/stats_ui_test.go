package main

import (
	"testing"
	"time"

	"gothoom/eui"
)

func TestStatsHistoryDuration(t *testing.T) {
	if got := time.Duration(statsHistorySize) * statsSampleInterval; got != statsHistoryDuration {
		t.Fatalf("stats history duration = %s, want %s", got, statsHistoryDuration)
	}
}

func TestStatsGraphMaximumUsesReadableZeroBasedScale(t *testing.T) {
	tests := []struct {
		name    string
		values  []float64
		minimum float64
		want    float64
	}{
		{name: "network floor", values: []float64{184, 195, 191}, minimum: 500, want: 500},
		{name: "network expands", values: []float64{510, 640, 720}, minimum: 500, want: 750},
		{name: "fps floor", values: []float64{58, 59}, minimum: 120, want: 120},
		{name: "server rate", values: []float64{4.8, 5.1}, minimum: 10, want: 10},
		{name: "memory floor", values: []float64{256, 768}, minimum: 4096, want: 4096},
		{name: "empty", minimum: 10, want: 10},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := statsGraphMaximum(test.values, test.minimum); got != test.want {
				t.Fatalf("statsGraphMaximum(%v, %v) = %v, want %v", test.values, test.minimum, got, test.want)
			}
		})
	}
}

func TestStatsGraphScaleText(t *testing.T) {
	if got := statsGraphScaleText(120, "fps"); got != "Scale 120fps" {
		t.Fatalf("scale text = %q", got)
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

func TestClearStatsHistory(t *testing.T) {
	originalHistory := statsHistory
	originalCount := statsHistoryCount
	originalNext := statsHistoryNext
	originalSample := lastStatsSample
	originalRender := lastStatsRender
	t.Cleanup(func() {
		statsHistory = originalHistory
		statsHistoryCount = originalCount
		statsHistoryNext = originalNext
		lastStatsSample = originalSample
		lastStatsRender = originalRender
	})

	appendStatsSample(liveStatsSample{fps: 60})
	lastStatsSample = time.Now()
	lastStatsRender = lastStatsSample
	clearStatsHistory()

	if statsHistoryCount != 0 || statsHistoryNext != 0 || len(statsSamples()) != 0 {
		t.Fatalf("stats history was not cleared: count=%d next=%d samples=%d", statsHistoryCount, statsHistoryNext, len(statsSamples()))
	}
	if !lastStatsSample.IsZero() || !lastStatsRender.IsZero() {
		t.Fatalf("stats timers were not reset: sample=%s render=%s", lastStatsSample, lastStatsRender)
	}
}

func TestSetPNAEnabledSynchronizesCheckboxes(t *testing.T) {
	originalEnabled := gs.AltNetMode
	originalDirty := settingsDirty
	originalStatsCheckbox := statsPNACheckbox
	originalAdvancedCheckbox := advancedPNACheckbox
	pnaControllerMu.Lock()
	originalController := pnaController
	pnaControllerMu.Unlock()
	pnaFallbackMu.Lock()
	originalFallback := pnaFallback
	pnaFallbackMu.Unlock()
	t.Cleanup(func() {
		gs.AltNetMode = originalEnabled
		settingsDirty = originalDirty
		statsPNACheckbox = originalStatsCheckbox
		advancedPNACheckbox = originalAdvancedCheckbox
		pnaControllerMu.Lock()
		pnaController = originalController
		pnaControllerMu.Unlock()
		pnaFallbackMu.Lock()
		pnaFallback = originalFallback
		pnaFallbackMu.Unlock()
	})

	gs.AltNetMode = false
	statsPNACheckbox = &eui.ItemData{}
	advancedPNACheckbox = &eui.ItemData{}
	pnaControllerMu.Lock()
	pnaController = pnaControllerState{initialized: true, lead: 50 * time.Millisecond}
	pnaControllerMu.Unlock()
	pnaFallbackMu.Lock()
	pnaFallback = pnaFallbackState{activeUntil: time.Now().Add(time.Minute), reason: "recent packet loss"}
	pnaFallbackMu.Unlock()
	settingsDirty = false
	setPNAEnabled(true)

	if !gs.AltNetMode || !statsPNACheckbox.Checked || !advancedPNACheckbox.Checked {
		t.Fatalf("PNA controls were not enabled together: setting=%v stats=%v advanced=%v",
			gs.AltNetMode, statsPNACheckbox.Checked, advancedPNACheckbox.Checked)
	}
	if !settingsDirty {
		t.Fatal("enabling PNA did not mark settings dirty")
	}
	pnaControllerMu.Lock()
	controllerReset := pnaController == (pnaControllerState{})
	pnaControllerMu.Unlock()
	pnaFallbackMu.Lock()
	fallbackReset := pnaFallback == (pnaFallbackState{})
	pnaFallbackMu.Unlock()
	if !controllerReset || !fallbackReset {
		t.Fatalf("enabling PNA kept stale state: controllerReset=%v fallbackReset=%v", controllerReset, fallbackReset)
	}
}
