package eui

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// colorWheelImage creates an Ebiten image containing a color wheel of the given size.
// The wheel ranges 0-359 degrees with black at the center and fully saturated
// color on the outer edge.
func colorWheelImage(size int) *ebiten.Image {
	if size <= 0 {
		return ebiten.NewImage(1, 1)
	}
	img := ebiten.NewImage(size, size)
	pix := make([]byte, size*size*4)
	r := float64(size) / 2
	// Use a 4x4 grid of subpixel samples for smoother edges
	offsets := []float64{0.125, 0.375, 0.625, 0.875}
	maxSamples := len(offsets) * len(offsets)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var rr, gg, bb, aa float64
			var coverage int
			for _, oy := range offsets {
				for _, ox := range offsets {
					dx := float64(x) + ox - r
					dy := float64(y) + oy - r
					dist := math.Hypot(dx, dy)
					if dist > r {
						continue
					}
					ang := math.Atan2(dy, dx) * 180 / math.Pi
					if ang < 0 {
						ang += 360
					}
					v := dist / r
					if v < 0 {
						v = 0
					} else if v > 1 {
						v = 1
					}
					col := hsvaToRGBA(ang, 1, v, 1)
					rr += float64(col.R)
					gg += float64(col.G)
					bb += float64(col.B)
					aa += float64(col.A)
					coverage++
				}
			}
			idx := 4 * (y*size + x)
			if coverage == 0 {
				pix[idx+0] = 0
				pix[idx+1] = 0
				pix[idx+2] = 0
				pix[idx+3] = 0
				continue
			}
			pix[idx+0] = uint8(rr / float64(maxSamples))
			pix[idx+1] = uint8(gg / float64(maxSamples))
			pix[idx+2] = uint8(bb / float64(maxSamples))
			pix[idx+3] = uint8(aa / float64(maxSamples))
		}
	}
	img.ReplacePixels(pix)
	return img
}
