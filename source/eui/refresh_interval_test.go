package eui

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestWindowRefreshIntervalCoalescesRepeatedInvalidations(t *testing.T) {
	win := NewWindow()
	win.Render = ebiten.NewImage(1, 1)
	t.Cleanup(func() { win.Render.Deallocate() })
	win.SetRefreshInterval(time.Second)
	win.lastRefresh = time.Now()
	win.Dirty = false

	win.markDirty()
	if win.Dirty || !win.refreshPending {
		t.Fatalf("coalesced refresh dirty=%v pending=%v, want false/true", win.Dirty, win.refreshPending)
	}

	win.SetRefreshInterval(0)
	if !win.Dirty || win.refreshPending {
		t.Fatalf("released refresh dirty=%v pending=%v, want true/false", win.Dirty, win.refreshPending)
	}
}
