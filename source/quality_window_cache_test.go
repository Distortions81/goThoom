package main

import (
	"testing"
	"time"
)

func TestQualityWindowUsesRenderCache(t *testing.T) {
	initFont()
	originalWindow := qualityWin
	qualityWin = nil
	t.Cleanup(func() {
		if qualityWin != nil {
			qualityWin.RemoveWindow()
		}
		qualityWin = originalWindow
	})

	makeQualityWindow()
	if qualityWin.NoCache {
		t.Fatal("quality window redraws its full contents every frame")
	}
	if qualityWin.RefreshInterval() != 100*time.Millisecond {
		t.Fatalf("quality refresh interval = %v, want 100ms", qualityWin.RefreshInterval())
	}
}
