package main

import (
	"sync"

	"github.com/Distortions81/EUI/eui"
)

// overlayLogOnce ensures we only dump overlay bounds a single time for debugging.
var overlayLogOnce sync.Once

// pointInUI reports whether the given screen coordinate lies within any EUI window or overlay.
func pointInUI(x, y int) bool {
	fx, fy := float32(x), float32(y)

	wins := eui.Windows()
	for i := len(wins) - 1; i >= 0; i-- {
		win := wins[i]
		if !win.IsOpen() || win.MainPortal {
			continue
		}
		pos := win.GetPos()
		size := win.GetSize()
		s := eui.UIScale()
		frame := (win.Margin + win.Border + win.BorderPad + win.Padding) * s
		title := win.GetTitleSize()
		x0, y0 := pos.X, pos.Y
		x1 := x0 + size.X + frame*2
		y1 := y0 + size.Y + frame*2 + title
		if fx >= x0 && fx < x1 && fy >= y0 && fy < y1 {
			return true
		}
	}

	// Log overlay bounds once to aid debugging of hit detection.
	overlayLogOnce.Do(logOverlayBounds)

	ovs := eui.Overlays()
	for i := len(ovs) - 1; i >= 0; i-- {
		ov := ovs[i]
		if !ov.Open {
			continue
		}
		r := ov.DrawRect
		if fx >= r.X0 && fx < r.X1 && fy >= r.Y0 && fy < r.Y1 {
			return true
		}
	}
	return false
}

// logOverlayBounds prints the screen position and size of each overlay and its items.
func logOverlayBounds() {
	for i, ov := range eui.Overlays() {
		r := ov.DrawRect
		logDebug("overlay %d: pos=(%.0f,%.0f) size=(%.0f,%.0f)", i, r.X0, r.Y0, r.X1-r.X0, r.Y1-r.Y0)
		for j, it := range ov.Contents {
			ir := it.DrawRect
			logDebug("  item %d %q: pos=(%.0f,%.0f) size=(%.0f,%.0f)", j, it.Text, ir.X0, ir.Y0, ir.X1-ir.X0, ir.Y1-ir.Y0)
		}
	}
}
