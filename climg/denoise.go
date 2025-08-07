package climg

import (
	"image"
	"image/color"
)

// denoiseImage smooths pixels that have uniformly coloured neighbours.
// Pixels surrounded by similar colours are averaged with a 3x3 box filter,
// while edge pixels remain unchanged.
func denoiseImage(img *image.RGBA) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Work on a copy so neighbour checks aren't affected by in-place writes.
	src := image.NewRGBA(bounds)
	copy(src.Pix, img.Pix)

	// Collect points that require filtering and apply the blur after the scan.
	var toFilter []image.Point
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			center := src.RGBAAt(x, y)

			// Ensure all neighbouring pixels are similar to the centre pixel.
			similar := true
			for dy := -1; dy <= 1 && similar; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					if colourDist(src.RGBAAt(x+dx, y+dy), center) > 64 {
						similar = false
						break
					}
				}
			}
			if similar {
				toFilter = append(toFilter, image.Point{X: x, Y: y})
			}
		}
	}

	for _, p := range toFilter {
		var r, g, b, a, count int
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				c := src.RGBAAt(p.X+dx, p.Y+dy)
				r += int(c.R)
				g += int(c.G)
				b += int(c.B)
				a += int(c.A)
				count++
			}
		}
		img.SetRGBA(p.X, p.Y, color.RGBA{uint8(r / count), uint8(g / count), uint8(b / count), uint8(a / count)})
	}
}

// colourDist returns the squared Euclidean distance between two colours.
func colourDist(a, b color.RGBA) int {
	dr := int(a.R) - int(b.R)
	dg := int(a.G) - int(b.G)
	db := int(a.B) - int(b.B)
	return dr*dr + dg*dg + db*db
}
