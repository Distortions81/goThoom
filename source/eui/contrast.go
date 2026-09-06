package eui

import "math"

// sRGB channels are linearized once; contrast checks also run while drawing.
var linearColorChannel = func() [256]float64 {
	var values [256]float64
	for i := range values {
		c := float64(i) / 255
		if c <= 0.04045 {
			values[i] = c / 12.92
		} else {
			values[i] = math.Pow((c+0.055)/1.055, 2.4)
		}
	}
	return values
}()

func relativeLuminance(c Color) float64 {
	return 0.2126*linearColorChannel[c.R] + 0.7152*linearColorChannel[c.G] + 0.0722*linearColorChannel[c.B]
}

// colorOver composites a premultiplied RGBA color onto an opaque surface.
func colorOver(fg, bg Color) Color {
	if fg.A == 255 {
		return fg
	}
	if fg.A == 0 {
		return bg
	}
	channel := func(f, b uint8) uint8 { return uint8(min(255, int(f)+(int(b)*(255-int(fg.A))+127)/255)) }
	return NewColor(channel(fg.R, bg.R), channel(fg.G, bg.G), channel(fg.B, bg.B), 255)
}

func textContrast(fg, bg Color) float64 {
	a, b := relativeLuminance(colorOver(fg, bg)), relativeLuminance(bg)
	return (max(a, b) + 0.05) / (min(a, b) + 0.05)
}

// readableTextColor retains the palette's text whenever it meets 4.5:1.
// A contrasting neutral keeps arbitrary user-selected accents readable too.
func readableTextColor(preferred, background Color) Color {
	if textContrast(preferred, background) >= 4.5 {
		return preferred
	}
	black, white := NewColor(0, 0, 0, 255), NewColor(255, 255, 255, 255)
	if textContrast(black, background) >= textContrast(white, background) {
		return black
	}
	return white
}

func (item *itemData) surfaceTextColor(preferred, background Color, filled bool) Color {
	if item.Disabled {
		return preferred
	}
	backdrop := baseTheme.Window.BGColor
	if currentTheme != nil {
		backdrop = currentTheme.Window.BGColor
	}
	if item.Theme != nil {
		backdrop = item.Theme.Window.BGColor
	}
	if item.ParentWindow != nil {
		backdrop = item.ParentWindow.backgroundColor()
	}
	if filled {
		backdrop = colorOver(background, backdrop)
	}
	return readableTextColor(preferred, backdrop)
}
