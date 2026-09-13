package main

import (
	"image"
	"testing"
)

func TestViewportManagerHasStableSessionAssignments(t *testing.T) {
	manager := newViewportManager()
	views := manager.snapshot()
	for slot, view := range views {
		want, _ := sessionIDForSlot(slot)
		if view.ID != ViewportID(slot+1) || view.SessionID != want {
			t.Fatalf("slot %d = %+v", slot, view)
		}
		if view.Active != (slot == 0) {
			t.Fatalf("slot %d active = %v", slot, view.Active)
		}
	}
	firstRender := manager.renderStateForSession(1)
	secondRender := manager.renderStateForSession(2)
	if firstRender == nil || secondRender == nil || firstRender == secondRender {
		t.Fatal("viewport render caches are not independent")
	}
	firstRender.drawSnapshot.worldGeneration = 99
	if secondRender.drawSnapshot.worldGeneration != 0 {
		t.Fatal("viewport draw snapshots crossed session boundaries")
	}

	manager.showSession(7)
	for slot, view := range manager.snapshot() {
		if view.Active != (slot == 6) {
			t.Fatalf("slot %d active = %v after selecting session 7", slot, view.Active)
		}
	}
}

func TestViewportHitTestAndWorldCoordinates(t *testing.T) {
	manager := newViewportManager()
	manager.showSession(2)
	if !manager.setRect(2, image.Rect(100, 200, 100+gameAreaSizeX, 200+gameAreaSizeY)) {
		t.Fatal("could not set viewport rectangle")
	}
	view, ok := manager.hitTest(image.Pt(100+fieldCenterX, 200+fieldCenterY))
	if !ok || view.SessionID != 2 {
		t.Fatalf("hit = %+v, %v", view, ok)
	}
	x, y, ok := view.worldAt(image.Pt(100+fieldCenterX, 200+fieldCenterY))
	if !ok || x != 0 || y != 0 {
		t.Fatalf("world point = %d,%d, %v", x, y, ok)
	}
	if _, _, ok := view.worldAt(image.Pt(99, 200)); ok {
		t.Fatal("accepted point outside viewport")
	}
}

func TestViewportSelectionPrecedesWorldDispatch(t *testing.T) {
	sessions := newSessionManager(mustNewSession(primarySessionID))
	sessions.materializeAllSessions()
	manager := newViewportManager()
	manager.showSession(2)
	manager.setRect(2, image.Rect(100, 200, 100+gameAreaSizeX, 200+gameAreaSizeY))

	view, x, y, inWorld, hit := manager.selectAt(image.Pt(100+fieldCenterX, 200+fieldCenterY), sessions)
	if !hit || !inWorld || view.SessionID != 2 || x != 0 || y != 0 {
		t.Fatalf("selection = %+v world=%d,%d inWorld=%v hit=%v", view, x, y, inWorld, hit)
	}
	if sessions.selectedID() != 2 {
		t.Fatal("viewport hit did not select its session before dispatch")
	}
}
