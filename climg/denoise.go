package climg

import (
	"image"
	"image/color"
)

// denoiseImage applies a simple 3x3 box blur to reduce dithering artifacts.
func denoiseImage(img *image.RGBA) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	dst := image.NewRGBA(bounds)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var r, g, b, a, count int
			for dy := -1; dy <= 1; dy++ {
				ny := y + dy
				if ny < 0 || ny >= h {
					continue
				}
				for dx := -1; dx <= 1; dx++ {
					nx := x + dx
					if nx < 0 || nx >= w {
						continue
					}
					off := img.PixOffset(nx, ny)
					r += int(img.Pix[off])
					g += int(img.Pix[off+1])
					b += int(img.Pix[off+2])
					a += int(img.Pix[off+3])
					count++
				}
			}
			dst.SetRGBA(x, y, color.RGBA{uint8(r / count), uint8(g / count), uint8(b / count), uint8(a / count)})
		}
	}
	copy(img.Pix, dst.Pix)
}
