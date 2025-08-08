package climg

import (
	"image"
	"image/color"
	"math"
)

// denoiseImage softens pixels by blending with neighbours. Pixels that are
// more similar to their neighbours are blended more strongly while
// dissimilar pixels are blended less. The sharpness parameter controls how
// quickly the blend amount falls off as colours become more different. Only
// the immediate horizontal and vertical neighbours are considered. If all of
// those neighbours are transparent they are still blended to soften isolated
// pixels.
func denoiseImage(img *image.RGBA, sharpness, maxPercent float64) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Work on a copy so neighbour checks aren't affected by in-place writes.
	src := image.NewRGBA(bounds)
	copy(src.Pix, img.Pix)

	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			c := src.RGBAAt(x, y)

			// Check only direct neighbours.
			neighbours := []image.Point{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}

			allTransparent := true
			for _, n := range neighbours {
				if src.RGBAAt(x+n.X, y+n.Y).A != 0 {
					allTransparent = false
					break
				}
			}

			for _, n := range neighbours {
				ncol := src.RGBAAt(x+n.X, y+n.Y)
				dist := colourDist(c, ncol)
				if allTransparent {
					dist = 0
				}
				nd := float64(dist) / 195075.0
				if nd < 1 {
					blend := maxPercent * math.Pow(1-nd, sharpness)
					if blend > 0 {
						c = mixColour(c, ncol, blend)
					}
				}
			}
			img.SetRGBA(x, y, c)
		}
	}
}

// colourDist returns the squared Euclidean distance between two colours.
const dt = 15

func colourDist(a, b color.RGBA) int {
	dr := int(a.R) - int(b.R)
	dg := int(a.G) - int(b.G)
	db := int(a.B) - int(b.B)
	if a.A < 0xFF || b.A < 0xFF ||
		(a.R < dt && a.G < dt && a.B < dt) ||
		(b.R < dt && b.G < dt && b.B < dt) {
		return 195076
	}
	return dr*dr + dg*dg + db*db
}

// mixColour blends two colours together by the provided percentage.
func mixColour(a, b color.RGBA, p float64) color.RGBA {
	inv := 1 - p
	return color.RGBA{
		R: uint8(float64(a.R)*inv + float64(b.R)*p),
		G: uint8(float64(a.G)*inv + float64(b.G)*p),
		B: uint8(float64(a.B)*inv + float64(b.B)*p),
		A: uint8(float64(a.A)*inv + float64(b.A)*p),
	}
}
