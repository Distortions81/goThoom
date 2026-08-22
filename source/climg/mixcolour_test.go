package climg

import (
	"image/color"
	"testing"
)

// mixColour64 mirrors the previous float64-based implementation for comparison.
func mixColour64(a, b color.RGBA, p float32) color.RGBA {
	pf := float64(p)
	inv := 1 - pf
	return color.RGBA{
		R: uint8(float64(a.R)*inv + float64(b.R)*pf),
		G: uint8(float64(a.G)*inv + float64(b.G)*pf),
		B: uint8(float64(a.B)*inv + float64(b.B)*pf),
		A: uint8(float64(a.A)*inv + float64(b.A)*pf),
	}
}

// Test that the float32 mixColour behaves identically to the previous float64
// implementation for typical inputs.
func TestMixColourFloat32Consistency(t *testing.T) {
	colours := []color.RGBA{
		{0, 0, 0, 0},
		{255, 0, 0, 128},
		{0, 255, 0, 255},
		{0, 0, 255, 255},
		{200, 100, 50, 255},
	}
	percents := []float32{0, 0.25, 0.5, 0.75, 1}
	for _, a := range colours {
		for _, b := range colours {
			for _, p := range percents {
				got := mixColour(a, b, p)
				want := mixColour64(a, b, p)
				if diff := channelDiff(got, want); diff > 1 {
					t.Fatalf("mixColour mismatch for %v %v %.2f: diff %d", a, b, p, diff)
				}
			}
		}
	}
}

func channelDiff(a, b color.RGBA) int {
	dr := int(a.R) - int(b.R)
	if dr < 0 {
		dr = -dr
	}
	dg := int(a.G) - int(b.G)
	if dg < 0 {
		dg = -dg
	}
	db := int(a.B) - int(b.B)
	if db < 0 {
		db = -db
	}
	da := int(a.A) - int(b.A)
	if da < 0 {
		da = -da
	}
	if dr > dg {
		dg = dr
	}
	if dg > db {
		db = dg
	}
	if db > da {
		da = db
	}
	return da
}
