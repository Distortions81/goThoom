package main

import "github.com/Distortions81/EUI/eui"

// pointInUI reports whether the given screen coordinate lies within any EUI window or overlay.
func pointInUI(x, y int) bool {
	fx, fy := float32(x), float32(y)
	for _, win := range eui.Windows() {
		pos := win.GetPos()
		size := win.GetSize()
		if fx >= pos.X && fx < pos.X+size.X && fy >= pos.Y && fy < pos.Y+size.Y {
			return true
		}
	}
	for _, ov := range eui.Overlays() {
		pos := ov.GetPos()
		size := ov.GetSize()
		if fx >= pos.X && fx < pos.X+size.X && fy >= pos.Y && fy < pos.Y+size.Y {
			return true
		}
	}
	return false
}
