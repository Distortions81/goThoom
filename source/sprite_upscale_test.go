package main

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func TestScale2xRGBAProducesExpectedEdges(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3, 3))
	blue := color.RGBA{0, 0, 255, 255}
	red := color.RGBA{255, 0, 0, 255}
	green := color.RGBA{0, 255, 0, 255}
	white := color.RGBA{255, 255, 255, 255}

	src.SetRGBA(1, 0, blue)  // B
	src.SetRGBA(0, 1, blue)  // D
	src.SetRGBA(1, 1, white) // E
	src.SetRGBA(2, 1, red)   // F
	src.SetRGBA(1, 2, green) // H

	dst := scale2xRGBA(src)
	if dst.Bounds().Dx() != 6 || dst.Bounds().Dy() != 6 {
		t.Fatalf("unexpected size: %v", dst.Bounds())
	}

	// Center pixel block starts at (2,2) in the scaled image.
	topLeft := dst.RGBAAt(2, 2)
	if topLeft != blue {
		t.Fatalf("expected top-left pixel to match left/top neighbor, got %#v", topLeft)
	}
	topRight := dst.RGBAAt(3, 2)
	if topRight != white {
		t.Fatalf("expected top-right pixel to stay center color, got %#v", topRight)
	}
}

func TestScale3xRGBAProducesExpectedEdges(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3, 3))
	blue := color.RGBA{0, 0, 255, 255}
	red := color.RGBA{200, 0, 0, 255}
	green := color.RGBA{0, 255, 0, 255}
	white := color.RGBA{255, 255, 255, 255}
	black := color.RGBA{0, 0, 0, 255}

	src.SetRGBA(1, 0, red)   // B
	src.SetRGBA(0, 1, blue)  // D
	src.SetRGBA(1, 1, white) // E
	src.SetRGBA(2, 1, red)   // F
	src.SetRGBA(1, 2, green) // H
	src.SetRGBA(2, 2, black) // I

	dst := scale3xRGBA(src)
	if dst.Bounds().Dx() != 9 || dst.Bounds().Dy() != 9 {
		t.Fatalf("unexpected size: %v", dst.Bounds())
	}

	// Center block origin at (3,3). Verify E2 (top-right) and E5 (middle-right).
	topRight := dst.RGBAAt(5, 3)
	if topRight != red {
		t.Fatalf("expected top-right pixel to adopt right neighbor, got %#v", topRight)
	}
	middleRight := dst.RGBAAt(5, 4)
	if middleRight != red {
		t.Fatalf("expected middle-right pixel to adopt right neighbor, got %#v", middleRight)
	}
}

func TestScale4xRGBAChainsTwoScale2xPasses(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3, 3))
	blue := color.RGBA{0, 0, 255, 255}
	red := color.RGBA{255, 0, 0, 255}
	green := color.RGBA{0, 255, 0, 255}
	white := color.RGBA{255, 255, 255, 255}

	src.SetRGBA(1, 0, blue)
	src.SetRGBA(0, 1, blue)
	src.SetRGBA(1, 1, white)
	src.SetRGBA(2, 1, red)
	src.SetRGBA(1, 2, green)

	dst := scale4xRGBA(src)
	if dst.Bounds().Dx() != 12 || dst.Bounds().Dy() != 12 {
		t.Fatalf("unexpected size: %v", dst.Bounds())
	}

	expected := scale2xRGBA(scale2xRGBA(src))
	if !dst.Bounds().Eq(expected.Bounds()) {
		t.Fatalf("expected bounds %v, got %v", expected.Bounds(), dst.Bounds())
	}
	if !bytes.Equal(dst.Pix, expected.Pix) {
		t.Fatalf("4x upscale should match two chained 2x passes")
	}

	center := dst.RGBAAt(6, 6)
	if center != white {
		t.Fatalf("expected center pixel to remain white, got %#v", center)
	}
}

func TestSpriteUpscaleColorSimilarityThresholds(t *testing.T) {
	base := rgbaPixel{r: 120, g: 100, b: 90, a: 255}
	slight := rgbaPixel{r: 121, g: 101, b: 89, a: 255}
	if !similarColor(base, slight) {
		t.Fatalf("expected colours within threshold to be similar")
	}

	brightnessShift := rgbaPixel{r: 180, g: 160, b: 150, a: 255}
	if similarColor(base, brightnessShift) {
		t.Fatalf("expected large brightness change to exceed threshold")
	}

	hueShift := rgbaPixel{r: 90, g: 170, b: 100, a: 255}
	if similarColor(base, hueShift) {
		t.Fatalf("expected large hue change to exceed threshold")
	}

	saturationShift := rgbaPixel{r: 120, g: 120, b: 120, a: 255}
	if similarColor(base, saturationShift) {
		t.Fatalf("expected large saturation change to exceed threshold")
	}

	alphaChange := rgbaPixel{r: 120, g: 100, b: 90, a: 200}
	if similarColor(base, alphaChange) {
		t.Fatalf("pixels with different alpha should not be similar")
	}

	transparentA := rgbaPixel{r: 10, g: 10, b: 10, a: 0}
	transparentB := rgbaPixel{r: 200, g: 200, b: 200, a: 0}
	if !similarColor(transparentA, transparentB) {
		t.Fatalf("fully transparent pixels should be treated as similar")
	}
}

func TestScale2xRGBAIsolatedPixelSmoothing(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3, 3))
	white := color.RGBA{255, 255, 255, 255}
	src.SetRGBA(1, 1, white)

	dst := scale2xRGBA(src)
	if dst.Bounds().Dx() != 6 || dst.Bounds().Dy() != 6 {
		t.Fatalf("unexpected size: %v", dst.Bounds())
	}

	corner := dst.RGBAAt(2, 2)
	if corner.A >= white.A {
		t.Fatalf("expected softened alpha below source alpha, got %d", corner.A)
	}
	if corner.A < 180 {
		t.Fatalf("expected softened alpha to stay visible, got %d", corner.A)
	}
	if corner.R != corner.G || corner.G != corner.B {
		t.Fatalf("expected greyscale smoothing, got %#v", corner)
	}

	other := []color.RGBA{
		dst.RGBAAt(3, 2),
		dst.RGBAAt(2, 3),
		dst.RGBAAt(3, 3),
	}
	for idx, pix := range other {
		if pix != corner {
			t.Fatalf("expected symmetrical smoothing, mismatch at index %d: %#v vs %#v", idx, pix, corner)
		}
	}
}

func TestApplyUpscaleAntialiasSmoothsEdges(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 3, 3))
	black := color.RGBA{0, 0, 0, 255}
	white := color.RGBA{255, 255, 255, 255}
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			img.SetRGBA(x, y, black)
		}
	}
	img.SetRGBA(1, 0, white)
	img.SetRGBA(1, 2, white)
	aa := applyUpscaleAntialias(img)

	if aa == nil {
		t.Fatalf("expected anti-aliased image, got nil")
	}
	center := aa.RGBAAt(1, 1)
	if center.R <= 40 {
		t.Fatalf("expected center pixel to blend towards neighbours, got %#v", center)
	}
	if aa.RGBAAt(1, 0) != white {
		t.Fatalf("expected top pixel to remain white")
	}
	if aa.RGBAAt(0, 1) != black {
		t.Fatalf("expected adjacent black pixel to remain black")
	}
}
