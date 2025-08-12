package eui

import "image/color"

func NewColor(r, g, b, a uint8) Color {
	return Color(color.RGBA{R: r, G: g, B: b, A: a})
}
