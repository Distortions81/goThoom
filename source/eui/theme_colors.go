package eui

// namedColors holds theme-specific color mappings
var namedColors map[string]Color

// Variables tracking the accent color in HSV space. Saturation can be adjusted
// independently via the theme selector slider.
var (
	accentHue        float64
	accentSaturation float64 = 1
	accentValue      float64
	accentAlpha      float64 = 1
)
