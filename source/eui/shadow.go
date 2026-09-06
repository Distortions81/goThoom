package eui

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

type shadowMaskKey struct {
	spread, radius int
	falloff        float32
}

// Small, shared masks are stretched around the edges of a window. Neither
// moving a window nor changing its color requires a window-sized shadow cache.
var shadowMasks = map[shadowMaskKey]*ebiten.Image{}

func (win *windowData) drawShadow(screen *ebiten.Image) {
	if win.Docked || win.NoBGColor || !windowShadows || win.Opacity <= 0 {
		return
	}
	rr := roundRect{Size: win.GetSize(), Position: win.getPosition(), Fillet: win.Fillet}
	drawDropShadow(screen, &rr, win.ShadowSize*win.scale(), win.ShadowColor, win.ShadowFalloff, win.Opacity)
}

// ShadowColor uses straight RGB plus alpha, as in a conventional #RRGGBBAA
// color. Black gives a shadow; a bright color gives a glow with the same falloff.
func drawDropShadow(screen *ebiten.Image, box *roundRect, size float32, col Color, falloff, opacity float32) {
	if size <= 0 || col.A == 0 || opacity <= 0 || box.Size.X <= 0 || box.Size.Y <= 0 {
		return
	}
	if falloff <= 0 {
		falloff = 2
	}
	key := shadowMaskKey{
		spread:  int(math.Ceil(float64(min(size, 128)))),
		radius:  int(math.Ceil(float64(max(0, min(box.Fillet, box.Size.X/2, box.Size.Y/2, 128))))),
		falloff: max(0.25, min(falloff, 8)),
	}
	mask := shadowMasks[key]
	if mask == nil {
		// Bound memory while editing themes with many different effect sizes.
		if len(shadowMasks) >= 16 {
			for _, cached := range shadowMasks {
				cached.Deallocate()
			}
			clear(shadowMasks)
		}
		mask = makeShadowMask(key)
		shadowMasks[key] = mask
	}
	corner := key.spread + key.radius
	span := 2*corner + 1
	w, h := box.Size.X+float32(2*key.spread), box.Size.Y+float32(2*key.spread)
	x, y := box.Position.X-float32(key.spread), box.Position.Y-float32(key.spread)
	cw, ch := min(float32(corner), w/2), min(float32(corner), h/2)
	src := [4]int{0, corner, corner + 1, span}
	dx, dy := [4]float32{x, x + cw, x + w - cw, x + w}, [4]float32{y, y + ch, y + h - ch, y + h}
	for row := range 3 {
		for column := range 3 {
			if row == 1 && column == 1 {
				continue // The window interior does not receive an outer effect.
			}
			dw, dh := dx[column+1]-dx[column], dy[row+1]-dy[row]
			if dw <= 0 || dh <= 0 {
				continue
			}
			part := mask.SubImage(image.Rect(src[column], src[row], src[column+1], src[row+1])).(*ebiten.Image)
			op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
			op.GeoM.Scale(float64(dw)/float64(part.Bounds().Dx()), float64(dh)/float64(part.Bounds().Dy()))
			op.GeoM.Translate(float64(dx[column]), float64(dy[row]))
			op.ColorScale.ScaleWithColor(color.NRGBA{R: col.R, G: col.G, B: col.B, A: col.A})
			op.ColorScale.ScaleAlpha(min(opacity, 1))
			screen.DrawImage(part, op)
		}
	}
}

func makeShadowMask(key shadowMaskKey) *ebiten.Image {
	corner := key.spread + key.radius
	span := 2*corner + 1
	pixels := make([]byte, span*span*4)
	center := float64(span) / 2
	for y := range span {
		for x := range span {
			// Signed distance to the small rounded rectangle at the mask center.
			qx := math.Abs(float64(x)+0.5-center) - 0.5
			qy := math.Abs(float64(y)+0.5-center) - 0.5
			distance := math.Hypot(max(qx, 0), max(qy, 0)) + min(max(qx, qy), 0) - float64(key.radius)
			if distance <= 0 || distance >= float64(key.spread) {
				continue
			}
			a := uint8(math.Round(255 * math.Pow(1-distance/float64(key.spread), float64(key.falloff))))
			i := (y*span + x) * 4
			pixels[i], pixels[i+1], pixels[i+2], pixels[i+3] = a, a, a, a
		}
	}
	mask := newManagedImage(span, span)
	mask.WritePixels(pixels)
	return mask
}
