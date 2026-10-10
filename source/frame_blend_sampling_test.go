package main

import (
	"math"
	"testing"
)

func TestShaderDrawNeedsLinearFilter(t *testing.T) {
	for _, test := range []struct {
		name            string
		left, top, x, y float64
		linear, want    bool
	}{
		{name: "nearest requested", left: 0.2, top: 0.3, x: 1.25, y: 0.75},
		{name: "aligned 1x", x: 1, y: 1, linear: true},
		{name: "power of two upscaling", left: 13, top: -7, x: 2, y: 4, linear: true},
		{name: "other integer upscaling", left: 13, top: -7, x: 2, y: 3, linear: true, want: true},
		{name: "flipped", left: -20, top: 30, x: -2, y: -1, linear: true},
		{name: "fractional x", left: 0.1, x: 1, y: 1, linear: true, want: true},
		{name: "fractional y", top: -0.1, x: 1, y: 1, linear: true, want: true},
		{name: "fractional scale", x: 1.5, y: 1, linear: true, want: true},
		{name: "downscale", x: 1, y: 0.5, linear: true, want: true},
		{name: "zero scale", x: 0, y: 1, linear: true, want: true},
		{name: "near integer position", left: math.Nextafter(1, 2), x: 1, y: 1, linear: true, want: true},
		{name: "near integer scale", x: math.Nextafter(1, 2), y: 1, linear: true, want: true},
		{name: "infinite position", left: math.Inf(1), x: 1, y: 1, linear: true, want: true},
		{name: "infinite scale", x: math.Inf(-1), y: 1, linear: true, want: true},
		{name: "nan position", top: math.NaN(), x: 1, y: 1, linear: true, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			options := frameBlendDrawOptions{Left: test.left, Top: test.top, ScaleX: test.x, ScaleY: test.y, Linear: test.linear}
			if got := shaderDrawNeedsLinearFilter(options); got != test.want {
				t.Fatalf("needs linear filtering = %v, want %v", got, test.want)
			}
		})
	}
}
