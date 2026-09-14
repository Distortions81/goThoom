package eui

import "testing"

func TestWindowTitleCacheIgnoresContentInvalidation(t *testing.T) {
	win := NewWindow()
	win.Title = "Players"
	win.Size = point{X: 640, Y: 480}
	win.TitleHeight = 24
	win.Closable = true
	win.ShowDragbar = true

	key := win.currentTitleRenderKey()
	win.titleRenderKey = key
	win.titleRenderValid = true
	win.Dirty = false
	win.markDirty()

	if got := win.currentTitleRenderKey(); got != key {
		t.Fatal("content invalidation changed the title render key")
	}
	if win.titleRenderKey != key || !win.titleRenderValid {
		t.Fatal("content invalidation discarded the title cache")
	}

	win.Title = "Players (32)"
	if got := win.currentTitleRenderKey(); got == key {
		t.Fatal("changed title reused the old title render key")
	}
	win.Title = "Players"
	win.HoverClose = true
	if got := win.currentTitleRenderKey(); got == key {
		t.Fatal("changed title hover state reused the old title render key")
	}
	win.HoverClose = false
	win.Size.X++
	if got := win.currentTitleRenderKey(); got == key {
		t.Fatal("resized window reused the old title render key")
	}
}
