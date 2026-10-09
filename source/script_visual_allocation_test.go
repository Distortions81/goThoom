package main

import (
	"runtime"
	"testing"
)

func TestScriptOverlaySnapshotsReuseIndependentStorage(t *testing.T) {
	visuals := newSessionScriptVisualState()
	session := &Session{automation: &sessionAutomationState{visuals: visuals}}
	previous := scriptOverlayOps
	t.Cleanup(func() { scriptOverlayOps = previous })
	scriptOverlayOps = make(map[string][]overlayOp)
	for _, test := range []struct {
		name    string
		session *Session
		ops     map[string][]overlayOp
	}{
		{"Session", session, visuals.overlays},
		{"Fallback", nil, scriptOverlayOps},
	} {
		t.Run(test.name, func(t *testing.T) {
			var snapshot []overlayOp
			if allocs := testing.AllocsPerRun(100, func() {
				snapshot = scriptOverlaySnapshotForSession(test.session, snapshot)
			}); allocs != 0 {
				t.Fatalf("empty snapshot allocated %g times", allocs)
			}
			test.ops["script"] = []overlayOp{
				{kind: overlayTextKind, text: "first"},
				{kind: overlayFollowCircleKind, follow: overlayFollowOp{name: "Second"}},
			}
			snapshot = scriptOverlaySnapshotForSession(test.session, snapshot)
			if len(snapshot) != 2 {
				t.Fatalf("snapshot length = %d, want 2", len(snapshot))
			}
			storage := snapshot
			snapshot[0].text = "draw-owned edit"
			if test.ops["script"][0].text != "first" {
				t.Fatal("snapshot edit changed the script's operations")
			}
			test.ops["script"][0].text = "script edit"
			if snapshot[0].text != "draw-owned edit" {
				t.Fatal("script edit changed the draw snapshot")
			}
			if allocs := testing.AllocsPerRun(100, func() {
				snapshot = scriptOverlaySnapshotForSession(test.session, snapshot)
			}); allocs != 0 {
				t.Fatalf("warmed snapshot allocated %g times", allocs)
			}
			test.ops["script"] = test.ops["script"][:1]
			snapshot = scriptOverlaySnapshotForSession(test.session, snapshot)
			if len(snapshot) != 1 || &snapshot[0] != &storage[0] {
				t.Fatal("smaller snapshot did not reuse caller storage")
			}
			if storage[1] != (overlayOp{}) {
				t.Fatal("smaller snapshot retained removed operation references")
			}
			delete(test.ops, "script")
			snapshot = scriptOverlaySnapshotForSession(test.session, snapshot)
			if len(snapshot) != 0 || storage[0] != (overlayOp{}) {
				t.Fatal("empty snapshot retained the previous operations")
			}
		})
	}
}

func TestScriptMobileEffectsSkipUnusedNameNormalization(t *testing.T) {
	visuals := newSessionScriptVisualState()
	oldTints, oldOutlines := scriptMobileTints, scriptMobileOutlines
	oldNamedTints, oldNamedOutlines := scriptNamedMobileTints, scriptNamedMobileOutlines
	t.Cleanup(func() {
		scriptMobileTints, scriptMobileOutlines = oldTints, oldOutlines
		scriptNamedMobileTints, scriptNamedMobileOutlines = oldNamedTints, oldNamedOutlines
	})
	scriptMobileTints, scriptMobileOutlines = visuals.tints, visuals.outlines
	scriptNamedMobileTints, scriptNamedMobileOutlines = visuals.namedTints, visuals.namedOutlines
	base, namedColor := scriptMobileTint{r: 255, a: 255}, scriptMobileTint{g: 255, a: 255}
	for _, outline := range []bool{false, true} {
		named, sprites := visuals.namedTints, visuals.tints
		if outline {
			named, sprites = visuals.namedOutlines, visuals.outlines
		}
		sprites["script"] = map[uint16]scriptMobileTint{42: base}
		for _, lookup := range []func(uint16, string, bool) (scriptMobileTint, bool){visuals.mobileEffect, scriptMobileEffectForMobile} {
			if allocs := testing.AllocsPerRun(100, func() {
				lookup(42, "Pebble Pockets", outline)
			}); allocs != 0 {
				t.Fatalf("sprite-only lookup allocated %g times, outline=%v", allocs, outline)
			}
		}
		named["empty-owner"] = nil
		named["named-owner"] = map[string]scriptMobileTint{"éowyn": namedColor}
		for _, lookup := range []func(uint16, string, bool) (scriptMobileTint, bool){visuals.mobileEffect, scriptMobileEffectForMobile} {
			if got, ok := lookup(42, " ÉOWYN ", outline); !ok || got != namedColor {
				t.Fatalf("case-insensitive name match = %v, %v; outline=%v", got, ok, outline)
			}
			if got, ok := lookup(42, "  ", outline); !ok || got != base {
				t.Fatalf("blank name did not fall back to sprite: %v, %v", got, ok)
			}
		}
	}
}

func BenchmarkScriptOverlaySnapshot(b *testing.B) {
	for _, count := range []int{0, 12} {
		name := "Empty"
		if count != 0 {
			name = "Populated"
		}
		b.Run(name, func(b *testing.B) {
			visuals := newSessionScriptVisualState()
			if count != 0 {
				visuals.overlays["script"] = make([]overlayOp, count)
			}
			var snapshot []overlayOp
			b.ReportAllocs()
			for b.Loop() {
				snapshot = visuals.overlaySnapshot()
			}
			runtime.KeepAlive(snapshot)
		})
	}
}

func BenchmarkScriptOverlaySnapshotReused(b *testing.B) {
	visuals := newSessionScriptVisualState()
	visuals.overlays["script"] = make([]overlayOp, 12)
	session := &Session{automation: &sessionAutomationState{visuals: visuals}}
	snapshot := scriptOverlaySnapshotForSession(session, nil)
	b.ReportAllocs()
	for b.Loop() {
		snapshot = scriptOverlaySnapshotForSession(session, snapshot)
	}
	runtime.KeepAlive(snapshot)
}

func BenchmarkScriptMobileEffectWithoutNamedEffects(b *testing.B) {
	visuals := newSessionScriptVisualState()
	b.ReportAllocs()
	for b.Loop() {
		visuals.mobileEffect(42, "Pebble Pockets", false)
		visuals.mobileEffect(42, "Pebble Pockets", true)
	}
}
