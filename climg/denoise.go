package climg

import (
	"image"
	"image/color"
	"math"
	"sync"
)

// denoiseImage softens pixels by blending with neighbours. Pixels that are
// more similar to their neighbours are blended more strongly while
// dissimilar pixels are blended less. The sharpness parameter controls how
// quickly the blend amount falls off as colours become more different. Only
// the immediate horizontal and vertical neighbours are considered.
func denoiseImage(img *image.RGBA, sharpness, maxPercent float64) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Work on a copy so neighbour checks aren't affected by in-place writes.
	src := getTempRGBA(bounds)
	copy(src.Pix, img.Pix)

	for y := 1; y < h-1; y++ {
		yoff := y * src.Stride
		for x := 1; x < w-1; x++ {
			off := yoff + x*4
			c := color.RGBA{src.Pix[off], src.Pix[off+1], src.Pix[off+2], src.Pix[off+3]}

			// Check only direct neighbours.
			neighbours := []image.Point{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
			for _, n := range neighbours {
				nOff := (y+n.Y)*src.Stride + (x+n.X)*4
				ncol := color.RGBA{src.Pix[nOff], src.Pix[nOff+1], src.Pix[nOff+2], src.Pix[nOff+3]}
				dist := colourDist(c, ncol)
				nd := float64(dist) / 195075.0
				if nd < 1 {
					blend := maxPercent * math.Pow(1-nd, sharpness)
					if blend > 0 {
						c = mixColour(c, ncol, blend)
					}
				}
			}
			dstOff := y*img.Stride + x*4
			img.Pix[dstOff] = c.R
			img.Pix[dstOff+1] = c.G
			img.Pix[dstOff+2] = c.B
			img.Pix[dstOff+3] = c.A
		}
	}
	putTempRGBA(src)
}

var rgbaPool = sync.Pool{New: func() any { return &image.RGBA{} }}

func getTempRGBA(bounds image.Rectangle) *image.RGBA {
	img := rgbaPool.Get().(*image.RGBA)
	w, h := bounds.Dx(), bounds.Dy()
	need := w * h * 4
	if cap(img.Pix) < need {
		img.Pix = make([]uint8, need)
	}
	img.Pix = img.Pix[:need]
	img.Stride = w * 4
	img.Rect = bounds
	return img
}

func putTempRGBA(img *image.RGBA) { rgbaPool.Put(img) }

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
