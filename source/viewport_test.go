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
	manager.enableMulti(viewportLayoutFreeform)
	for slot, view := range manager.snapshot() {
		if !view.Active || view.SessionID != SessionID(slot+1) {
			t.Fatalf("enabled slot %d = %+v", slot, view)
		}
	}
	manager.disableMulti()
	views = manager.snapshot()
	if !views[0].Active || views[1].Active || views[2].Active || views[3].Active {
		t.Fatalf("single-session views = %+v", views)
	}
}

func TestTiledViewportRectsCoverOddAreaWithoutOverlap(t *testing.T) {
	area := image.Rect(3, 5, 604, 406)
	rects := tiledViewportRects(area)
	covered := 0
	for index, rect := range rects {
		if !rect.In(area) || rect.Empty() {
			t.Fatalf("tile %d = %v outside %v", index, rect, area)
		}
		covered += rect.Dx() * rect.Dy()
		for other := 0; other < index; other++ {
			if rect.Overlaps(rects[other]) {
				t.Fatalf("tiles overlap: %v and %v", rect, rects[other])
			}
		}
	}
	if covered != area.Dx()*area.Dy() {
		t.Fatalf("covered %d pixels, want %d", covered, area.Dx()*area.Dy())
	}
}

func TestViewportHitTestAndWorldCoordinates(t *testing.T) {
	manager := newViewportManager()
	manager.enableMulti(viewportLayoutFreeform)
	if !manager.setRect(1, image.Rect(10, 20, 10+gameAreaSizeX, 20+gameAreaSizeY)) ||
		!manager.setRect(2, image.Rect(700, 20, 700+gameAreaSizeX, 20+gameAreaSizeY)) {
		t.Fatal("could not set viewport rectangles")
	}
	view, ok := manager.hitTest(image.Pt(700+fieldCenterX, 20+fieldCenterY))
	if !ok || view.SessionID != 2 {
		t.Fatalf("hit = %+v, %v", view, ok)
	}
	x, y, ok := view.worldAt(image.Pt(700+fieldCenterX, 20+fieldCenterY))
	if !ok || x != 0 || y != 0 {
		t.Fatalf("world point = %d,%d, %v", x, y, ok)
	}
	if _, _, ok := view.worldAt(image.Pt(699, 20)); ok {
		t.Fatal("accepted point outside viewport")
	}
}

func TestViewportSelectionPrecedesWorldDispatch(t *testing.T) {
	sessions := newSessionManager(mustNewSession(primarySessionID))
	sessions.enableMulti()
	manager := newViewportManager()
	manager.enableMulti(viewportLayoutFreeform)
	manager.setRect(2, image.Rect(100, 200, 100+gameAreaSizeX, 200+gameAreaSizeY))

	view, x, y, inWorld, hit := manager.selectAt(image.Pt(100+fieldCenterX, 200+fieldCenterY), sessions)
	if !hit || !inWorld || view.SessionID != 2 || x != 0 || y != 0 {
		t.Fatalf("selection = %+v world=%d,%d inWorld=%v hit=%v", view, x, y, inWorld, hit)
	}
	if sessions.selectedID() != 2 {
		t.Fatal("viewport hit did not select its session before dispatch")
	}
}
