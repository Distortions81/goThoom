package main

import (
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type genderIcon int

const (
	genderUnknown genderIcon = iota
	genderMale
	genderFemale
)

// genderFromString maps server-provided gender strings to an icon type.
func genderFromString(s string) genderIcon {
	g := strings.ToLower(strings.TrimSpace(s))
	switch g {
	case "male", "m":
		return genderMale
	case "female", "f":
		return genderFemale
	case "other", "unknown", "", "undisclosed", "n/a":
		fallthrough
	default:
		return genderUnknown
	}
}

type iconKey struct {
	g    genderIcon
	size int
}

var genderIconCache = map[iconKey]*ebiten.Image{}

// getGenderIcon returns a small pictographic icon indicating gender.
// The image is cached per (gender,size) pair.
func getGenderIcon(g genderIcon, size int) *ebiten.Image {
	if size <= 4 {
		size = 4
	}
	key := iconKey{g: g, size: size}
	if img, ok := genderIconCache[key]; ok && img != nil {
		return img
	}
	img := ebiten.NewImage(size, size)
	// Colors: subtle but readable defaults.
	var col color.NRGBA
	switch g {
	case genderMale:
		col = color.NRGBA{R: 0x5a, G: 0x8b, B: 0xff, A: 0xff} // bluish
	case genderFemale:
		col = color.NRGBA{R: 0xff, G: 0x66, B: 0xb2, A: 0xff} // pinkish
	default:
		col = color.NRGBA{R: 0xa0, G: 0xa0, B: 0xa0, A: 0xff} // gray
	}

	// Common geometry helpers
	s := float32(size)
	stroke := maxf(1, s/16) // line thickness relative to size

	switch g {
	case genderMale:
		// Circle with NE arrow (Mars symbol)
		cx := s * 0.40
		cy := s * 0.60
		r := s * 0.25
		vector.DrawFilledCircle(img, cx, cy, r, col, true)
		// Arrow shaft
		x0 := cx + r*0.7
		y0 := cy - r*0.7
		x1 := s * 0.86
		y1 := s * 0.14
		vector.StrokeLine(img, x0, y0, x1, y1, stroke, col, true)
		// Arrow head
		ah := r * 0.8
		vector.StrokeLine(img, x1, y1, x1-ah*0.6, y1, stroke, col, true)
		vector.StrokeLine(img, x1, y1, x1, y1+ah*0.6, stroke, col, true)
	case genderFemale:
		// Circle with cross (Venus symbol)
		cx := s * 0.50
		cy := s * 0.38
		r := s * 0.25
		vector.DrawFilledCircle(img, cx, cy, r, col, true)
		// Vertical stem
		y0 := cy + r
		y1 := s * 0.86
		vector.StrokeLine(img, cx, y0, cx, y1, stroke, col, true)
		// Horizontal crossbar
		vector.StrokeLine(img, cx-r*0.6, (y0+y1)/2, cx+r*0.6, (y0+y1)/2, stroke, col, true)
	default:
		// Simple neutral circle with small center dot
		cx := s * 0.5
		cy := s * 0.5
		r := s * 0.28
		// Outer ring (by drawing two circles: thick ring via strokes isn't available, approximate)
		vector.DrawFilledCircle(img, cx, cy, r, col, true)
		// Punch a smaller hole with alpha by drawing in transparent color
		vector.DrawFilledCircle(img, cx, cy, r*0.65, color.NRGBA{0, 0, 0, 0}, true)
		// Center dot
		vector.DrawFilledCircle(img, cx, cy, r*0.18, col, true)
	}

	genderIconCache[key] = img
	return img
}

func maxf(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
