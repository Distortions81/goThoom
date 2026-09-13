package main

import (
	"testing"
	"time"
)

func TestSessionScriptVisualsAreIndependent(t *testing.T) {
	first := mustNewSession(2)
	second := mustNewSession(3)
	const owner = "same-script"

	scriptOverlayRectForSession(first, owner, 10, 11, 12, 13, 1, 2, 3, 4)
	scriptOverlayRectForSession(second, owner, 20, 21, 22, 23, 5, 6, 7, 8)
	firstOps := first.automation.visuals.overlaySnapshot()
	secondOps := second.automation.visuals.overlaySnapshot()
	if len(firstOps) != 1 || firstOps[0].x != 10 || len(secondOps) != 1 || secondOps[0].x != 20 {
		t.Fatalf("overlay state crossed sessions: first=%+v second=%+v", firstOps, secondOps)
	}

	firstTint := scriptMobileTint{r: 100, a: 255}
	secondTint := scriptMobileTint{b: 200, a: 255}
	scriptSetMobileEffectForSession(first, owner, 42, firstTint, false)
	scriptSetMobileEffectForSession(second, owner, 42, secondTint, false)
	if got, ok := scriptMobileEffectForSession(first, 42, "", false); !ok || got != firstTint {
		t.Fatalf("first tint = %+v, %v", got, ok)
	}
	if got, ok := scriptMobileEffectForSession(second, 42, "", false); !ok || got != secondTint {
		t.Fatalf("second tint = %+v, %v", got, ok)
	}

	scriptFlashMobileForSession(first, owner, 7, 1, 2, 3, 255, time.Minute)
	if _, ok := scriptMobileFlashForSession(first, 7); !ok {
		t.Fatal("first session flash is missing")
	}
	if _, ok := scriptMobileFlashForSession(second, 7); ok {
		t.Fatal("first session flash appeared in second session")
	}

	first.automation.visuals.clearOwner(owner)
	if len(first.automation.visuals.overlaySnapshot()) != 0 {
		t.Fatal("owner cleanup retained first-session overlays")
	}
	if len(second.automation.visuals.overlaySnapshot()) != 1 {
		t.Fatal("first-session cleanup removed second-session overlays")
	}
}
