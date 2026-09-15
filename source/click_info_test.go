package main

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestMobileSizeUsesImageMetadata(t *testing.T) {
	origImages := clImages
	clImages = mockCLImages(160, 80)
	defer func() { clImages = origImages }()

	if got := mobileSize(1); got != 10 {
		t.Fatalf("mobileSize(1) = %d, want 10", got)
	}
}

func TestWorldHoverClearsOutsideGameViewAndOnFocusLoss(t *testing.T) {
	session := mustNewSession(2)
	session.draw.current = drawState{
		descriptors: map[uint8]frameDescriptor{7: {Index: 7, Name: "Self", PictID: 100}},
		liveMobs:    []frameMobile{{Index: 7, H: 0, V: 0}},
	}
	oldMobileSizeFunc := mobileSizeFunc
	mobileSizeFunc = func(uint16) int { return 10 }
	t.Cleanup(func() { mobileSizeFunc = oldMobileSizeFunc })
	oldHoverOnly := gs.NameTagsOnHoverOnly
	gs.NameTagsOnHoverOnly = true
	t.Cleanup(func() { gs.NameTagsOnHoverOnly = oldHoverOnly })
	clearNameTagHoverReveals()
	t.Cleanup(clearNameTagHoverReveals)

	start := time.Unix(300, 0)
	updateSessionWorldHoverForPointer(session, 0, 0, true, true)
	if !sessionHoverSnapshot(session).OnMobile {
		t.Fatal("pointer over self did not establish hover")
	}
	if got := nameTagHoverAlphaForSession(session.ID(), 7, "Self", true, start); got != 1 {
		t.Fatalf("initial name alpha = %v, want 1", got)
	}

	before := worldRenderGeneration.Load()
	updateSessionWorldHoverForPointer(session, 0, 0, false, true)
	if sessionHoverSnapshot(session).OnMobile {
		t.Fatal("pointer outside game view retained self hover")
	}
	if _, ok := session.input.cachedHover(session.draw.generation.Load(), 0, 0); ok {
		t.Fatal("outside-view hover was cached at world origin")
	}
	if worldRenderGeneration.Load() != before+1 {
		t.Fatal("leaving game view did not invalidate name-tag rendering")
	}
	if got := nameTagHoverAlphaForSession(session.ID(), 7, "Self", false, start.Add(nameTagHoverHold+nameTagHoverFade/2)); got != 0.5 {
		t.Fatalf("name alpha after leaving game view = %v, want 0.5", got)
	}

	updateSessionWorldHoverForPointer(session, 0, 0, true, true)
	if !sessionHoverSnapshot(session).OnMobile {
		t.Fatal("returning to the same world coordinate did not restore hover")
	}
	updateSessionWorldHoverForPointer(session, 0, 0, true, false)
	if sessionHoverSnapshot(session).OnMobile {
		t.Fatal("focus loss retained stale self hover")
	}
}

func TestUpdateWorldHoverCachesUnchangedQuery(t *testing.T) {
	primarySession.draw.mu.Lock()
	origState := primarySession.draw.current
	primarySession.draw.current = drawState{
		descriptors: map[uint8]frameDescriptor{1: {Index: 1, Name: "Bob", PictID: 100}},
		liveMobs:    []frameMobile{{Index: 1, H: 0, V: 0}},
	}
	primarySession.draw.mu.Unlock()

	origMobileSizeFunc := mobileSizeFunc
	sizeCalls := 0
	mobileSizeFunc = func(uint16) int {
		sizeCalls++
		return 10
	}

	origGeneration := primarySession.draw.generation.Load()
	primarySession.input.mu.Lock()
	origHover := primarySession.input.lastHover
	origHoverGeneration := primarySession.input.hoverGen
	origHoverQueryValid := primarySession.input.hoverValid
	primarySession.input.hoverValid = false
	primarySession.input.mu.Unlock()
	defer func() {
		primarySession.draw.mu.Lock()
		primarySession.draw.current = origState
		primarySession.draw.mu.Unlock()
		mobileSizeFunc = origMobileSizeFunc
		primarySession.draw.generation.Store(origGeneration)
		primarySession.input.mu.Lock()
		primarySession.input.lastHover = origHover
		primarySession.input.hoverGen = origHoverGeneration
		primarySession.input.hoverValid = origHoverQueryValid
		primarySession.input.mu.Unlock()
	}()

	updateWorldHover(0, 0)
	updateWorldHover(0, 0)
	if sizeCalls != 1 {
		t.Fatalf("unchanged hover query performed %d size lookups, want 1", sizeCalls)
	}

	primarySession.draw.mu.Lock()
	markWorldStateChanged()
	primarySession.draw.mu.Unlock()
	updateWorldHover(0, 0)
	if sizeCalls != 2 {
		t.Fatalf("state change did not invalidate hover query; calls = %d", sizeCalls)
	}
}

func TestSessionClickAndHoverStateIsIndependent(t *testing.T) {
	first := mustNewSession(2)
	second := mustNewSession(3)
	first.input.storeClick(ClickInfo{X: 10, Button: ebiten.MouseButtonRight, Mobile: Mobile{Name: "First"}})
	second.input.storeClick(ClickInfo{X: 20, Button: ebiten.MouseButtonRight, Mobile: Mobile{Name: "Second"}})
	first.input.storeHover(ClickInfo{X: 30, Mobile: Mobile{Name: "Hover One"}}, 1)
	second.input.storeHover(ClickInfo{X: 40, Mobile: Mobile{Name: "Hover Two"}}, 1)

	if got := scriptLastClickForSession(first); got.X != 10 || got.Mobile.Name != "First" {
		t.Fatalf("first click = %+v", got)
	}
	if got := scriptLastClickForSession(second); got.X != 20 || got.Mobile.Name != "Second" {
		t.Fatalf("second click = %+v", got)
	}
	if got := scriptHoverForSession(first); got.X != 30 || got.Mobile.Name != "Hover One" {
		t.Fatalf("first hover = %+v", got)
	}
	if got := scriptHoverForSession(second); got.X != 40 || got.Mobile.Name != "Hover Two" {
		t.Fatalf("second hover = %+v", got)
	}
	if got, ok := applyHotkeyVarsForSession(first, "/inspect @right.clicked"); !ok || got != "/inspect First" {
		t.Fatalf("first hotkey expansion = %q, %v", got, ok)
	}
	if got, ok := applyHotkeyVarsForSession(second, "/inspect @right.clicked"); !ok || got != "/inspect Second" {
		t.Fatalf("second hotkey expansion = %q, %v", got, ok)
	}
}
