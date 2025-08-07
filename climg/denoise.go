package climg

import (
	"image"
	"image/color"
)

// denoiseImage scans for pixels that are completely different from their
// immediate neighbours and applies a 3x3 box filter on only those pixels.
// This helps smooth out single noisy pixels without blurring the entire
// image.
func denoiseImage(img *image.RGBA) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	dst := image.NewRGBA(bounds)

	// Start with the original image so untouched pixels remain the same.
	copy(dst.Pix, img.Pix)

	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			center := img.RGBAAt(x, y)
			isolated := true
			for dy := -1; dy <= 1 && isolated; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					if img.RGBAAt(x+dx, y+dy) == center {
						isolated = false
						break
					}
				}
			}
			if !isolated {
				continue
			}

			var r, g, b, a, count int
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					c := img.RGBAAt(x+dx, y+dy)
					r += int(c.R)
					g += int(c.G)
					b += int(c.B)
					a += int(c.A)
					count++
				}
			}
			dst.SetRGBA(x, y, color.RGBA{uint8(r / count), uint8(g / count), uint8(b / count), uint8(a / count)})
		}
	}

	copy(img.Pix, dst.Pix)
}
