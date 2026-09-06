package eui

import "testing"

func TestCloseHoverRefreshesCachedWindows(t *testing.T) {
	win := NewWindow()
	win.Open = true
	win.Position = point{40, 40}
	win.Size = point{200, 100}
	win.TitleHeight = 24
	r := win.xRect()
	pointer := point{(r.X0 + r.X1) / 2, (r.Y0 + r.Y1) / 2}
	order := []*windowData{win}
	win.Dirty = false
	updateCloseHover(order, pointer)
	if !win.HoverClose || !win.Dirty {
		t.Fatal("entering close button did not refresh its cached title bar")
	}
	win.Dirty = false
	updateCloseHover(order, pointer)
	if !win.HoverClose || win.Dirty {
		t.Fatal("stationary hover should retain state without repainting")
	}
	updateCloseHover(order, point{})
	if win.HoverClose || !win.Dirty {
		t.Fatal("leaving close button did not clear its cached highlight")
	}
	updateCloseHover(order, pointer)
	front := NewWindow()
	front.Open = true
	front.Position = win.Position
	front.Size = win.Size
	front.Closable = false
	updateCloseHover([]*windowData{front, win}, pointer)
	if win.HoverClose || front.HoverClose {
		t.Fatal("covered close button remained highlighted")
	}
	win.Docked = true
	updateCloseHover(order, pointer)
	if win.HoverClose {
		t.Fatal("docked window highlighted a hidden close button")
	}
}
