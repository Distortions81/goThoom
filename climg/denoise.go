package climg

import (
	"image"
	"image/color"
)

// denoiseImage scans for pixels that are outliers compared to a uniform set
// of neighbouring pixels. Pixels marked as outliers are later smoothed with a
// 3x3 box filter, leaving the rest of the image untouched.
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

			// Gather neighbours and their average colour.
			var neigh [8]color.RGBA
			var nr, ng, nb, na int
			idx := 0
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					c := src.RGBAAt(x+dx, y+dy)
					neigh[idx] = c
					idx++
					nr += int(c.R)
					ng += int(c.G)
					nb += int(c.B)
					na += int(c.A)
				}
			}
			avg := color.RGBA{uint8(nr / 8), uint8(ng / 8), uint8(nb / 8), uint8(na / 8)}

			// Ensure all neighbours are similar to each other.
			similar := true
			for i := 0; i < 8; i++ {
				if colourDist(neigh[i], avg) > 64 { // neighbour threshold
					similar = false
					break
				}
			}
			if !similar {
				continue
			}

			// Check if the centre pixel significantly differs from neighbours.
			if colourDist(center, avg) > 144 { // centre threshold
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
