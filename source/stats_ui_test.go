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
	t.Cleanup(func() {
		gs.AltNetMode = originalEnabled
		settingsDirty = originalDirty
		statsPNACheckbox = originalStatsCheckbox
		advancedPNACheckbox = originalAdvancedCheckbox
	})

	statsPNACheckbox = &eui.ItemData{}
	advancedPNACheckbox = &eui.ItemData{}
	settingsDirty = false
	setPNAEnabled(true)

	if !gs.AltNetMode || !statsPNACheckbox.Checked || !advancedPNACheckbox.Checked {
		t.Fatalf("PNA controls were not enabled together: setting=%v stats=%v advanced=%v",
			gs.AltNetMode, statsPNACheckbox.Checked, advancedPNACheckbox.Checked)
	}
	if !settingsDirty {
		t.Fatal("enabling PNA did not mark settings dirty")
	}
}
