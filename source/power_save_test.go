package main

import (
	"testing"
	"time"
)

func TestPowerSaveFPSClampAllowsHighRefreshRates(t *testing.T) {
	for _, test := range []struct {
		value int
		want  int
	}{
		{value: -1, want: 1},
		{value: 60, want: 60},
		{value: 144, want: 144},
		{value: 250, want: 250},
		{value: 300, want: 250},
	} {
		if got := clampPowerSaveFPS(test.value); got != test.want {
			t.Errorf("clampPowerSaveFPS(%d) = %d, want %d", test.value, got, test.want)
		}
	}
}

func TestFramePacingTargetFPSPriority(t *testing.T) {
	value := gsdef
	value.VSync = true
	value.LimitFPS250 = true
	if fps, active := framePacingTargetFPS(value, true); active || fps != 0 {
		t.Fatalf("VSync pacing target = (%d, %v), want inactive", fps, active)
	}

	value.VSync = false
	if fps, active := framePacingTargetFPS(value, true); !active || fps != 250 {
		t.Fatalf("VSync-off pacing target = (%d, %v), want (250, true)", fps, active)
	}

	value.LimitFPS250 = false
	if fps, active := framePacingTargetFPS(value, true); active || fps != 0 {
		t.Fatalf("disabled 250 FPS target = (%d, %v), want inactive", fps, active)
	}

	value.PowerSaveAlways = true
	value.PowerSaveFPS = 30
	if fps, active := framePacingTargetFPS(value, true); !active || fps != 30 {
		t.Fatalf("always-power-save target = (%d, %v), want (30, true)", fps, active)
	}

	value.PowerSaveAlways = false
	value.PowerSaveBackground = true
	value.PowerSaveFPS = 20
	value.LimitFPS250 = true
	if fps, active := framePacingTargetFPS(value, true); !active || fps != 250 {
		t.Fatalf("focused background-power-save target = (%d, %v), want (250, true)", fps, active)
	}
	if fps, active := framePacingTargetFPS(value, false); !active || fps != 20 {
		t.Fatalf("background power-save target = (%d, %v), want (20, true)", fps, active)
	}
}

func TestSlowShaderDetectionRequiresFocusedUnthrottledFrames(t *testing.T) {
	if !shouldTrackLowShaderFPS(true, false, 44) {
		t.Fatal("focused low FPS was not tracked")
	}
	if shouldTrackLowShaderFPS(false, false, 20) {
		t.Fatal("unfocused low FPS was tracked")
	}
	if shouldTrackLowShaderFPS(true, false, 1) {
		t.Fatal("display-sleep FPS was tracked")
	}
	if shouldTrackLowShaderFPS(true, true, 20) {
		t.Fatal("power-saved low FPS was tracked")
	}
	if shouldTrackLowShaderFPS(true, false, 45) {
		t.Fatal("acceptable focused FPS was tracked")
	}
}

func TestFrameRatePacerUsesExistingFrameInterval(t *testing.T) {
	start := time.Unix(1000, 0)
	var pacer frameRatePacer
	if delay := pacer.delay(start, true, 100); delay != 0 {
		t.Fatalf("first frame delay = %s, want 0", delay)
	}
	if delay := pacer.delay(start.Add(4*time.Millisecond), true, 100); delay != 6*time.Millisecond {
		t.Fatalf("early frame delay = %s, want 6ms", delay)
	}
	if delay := pacer.delay(start.Add(20*time.Millisecond), true, 100); delay != 0 {
		t.Fatalf("already slow frame delay = %s, want 0", delay)
	}
	if delay := pacer.delay(start.Add(21*time.Millisecond), false, 100); delay != 0 {
		t.Fatalf("inactive delay = %s, want 0", delay)
	}
	if !pacer.lastFrame.IsZero() {
		t.Fatal("inactive pacer retained its previous deadline")
	}
}

func TestFrameRatePacerAdaptsToSettingChanges(t *testing.T) {
	start := time.Unix(1000, 0)
	var pacer frameRatePacer
	pacer.delay(start, true, 250)
	if delay := pacer.delay(start.Add(2*time.Millisecond), true, 250); delay != 2*time.Millisecond {
		t.Fatalf("250 FPS delay = %s, want 2ms", delay)
	}
	if delay := pacer.delay(start.Add(10*time.Millisecond), true, 50); delay != 14*time.Millisecond {
		t.Fatalf("changed 50 FPS delay = %s, want 14ms", delay)
	}
}
