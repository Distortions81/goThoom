package climg

import (
	"image"
	"image/color"
)

// denoiseImage softens pixels by blending with neighbours within a given
// colour distance threshold. Only the immediate horizontal and vertical
// neighbours are considered.
func denoiseImage(img *image.RGBA, threshold int, percent float64) {
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
			for _, n := range neighbours {
				ncol := src.RGBAAt(x+n.X, y+n.Y)
				if colourDist(c, ncol) <= threshold {
					c = mixColour(c, ncol, percent)
				}
			}
			img.SetRGBA(x, y, c)
		}
	}
}

// colourDist returns the squared Euclidean distance between two colours.
func colourDist(a, b color.RGBA) int {
	dr := int(a.R) - int(b.R)
	dg := int(a.G) - int(b.G)
	db := int(a.B) - int(b.B)
	if a.A < 0xFF || b.A < 0xFF {
		return 65536
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
