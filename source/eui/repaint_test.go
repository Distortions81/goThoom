package eui

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestBackgroundRepaintBudgetAndDeadline(t *testing.T) {
	now := time.Now()
	win := NewWindow()
	win.NoScale, win.DeferRepaint, win.Dirty = true, true, true
	win.Size = point{700, 980}
	win.Render = ebiten.NewImage(700, 980)
	t.Cleanup(win.Render.Deallocate)
	spent := 0
	if win.shouldDeferRepaint(now, false, &spent) || spent != 686000 {
		t.Fatal("first large pane did not progress")
	}
	if !win.shouldDeferRepaint(now, false, &spent) || spent != 686000 {
		t.Fatal("second large repaint exceeded the frame budget")
	}
	if win.shouldDeferRepaint(now.Add(maxRepaintDelay), false, &spent) {
		t.Fatal("deadline did not prevent starvation")
	}
	win.Size = point{350, 490}
	win.Render.Deallocate()
	win.Render = ebiten.NewImage(350, 490)
	t.Cleanup(win.Render.Deallocate)
	win.repaintRequested = time.Time{}
	spent = 0
	for range 4 {
		if win.shouldDeferRepaint(now, false, &spent) {
			t.Fatal("four smaller panes should fit in one frame")
		}
	}
}

func TestRepaintUrgencyAndInvalidCacheBypassBudget(t *testing.T) {
	for _, mode := range []string{"interaction", "resize", "first paint", "animation", "opt out"} {
		t.Run(mode, func(t *testing.T) {
			win := NewWindow()
			win.NoScale, win.DeferRepaint, win.Dirty = true, true, true
			win.Size = point{700, 980}
			win.Render = ebiten.NewImage(700, 980)
			t.Cleanup(win.Render.Deallocate)
			switch mode {
			case "resize":
				win.Size.X++
			case "first paint":
				win.Render = nil
			case "animation":
				win.HasIndeterminate = true
			case "opt out":
				win.DeferRepaint = false
			}
			spent := backgroundRepaintPixels
			if win.shouldDeferRepaint(time.Now(), mode == "interaction", &spent) {
				t.Fatal("urgent repaint was delayed")
			}
		})
	}
}
