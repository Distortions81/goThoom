package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// TestRenderCharacterShadowImages is an opt-in visual diagnostic. It uses the
// production shadow transform and blend path with a fixed, recognizable sprite.
func TestRenderCharacterShadowImages(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_SHADOW_TESTS") == "" {
		t.Skip("set GOTHOOM_RENDER_SHADOW_TESTS=1 to render shadow diagnostics")
	}

	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate shadow render test source")
	}
	outputDir := filepath.Join(filepath.Dir(sourceFile), "data", "Screenshots", "shadow-tests")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}

	originalSettings := gs
	gs.GameScale = 1
	gs.DetailedCharacterShadows = false
	t.Cleanup(func() { gs = originalSettings })

	game := &shadowRenderGame{outputDir: outputDir}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if !game.rendered {
		t.Fatal("Ebitengine exited before rendering the shadow diagnostics")
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

const (
	shadowTestCanvasSize = 640
	shadowTestSpriteSize = 48
	shadowTestDrawSize   = 32
	shadowTestCenterX    = shadowTestCanvasSize / 2
	shadowTestCenterY    = shadowTestCanvasSize / 2
)

type shadowRenderGame struct {
	outputDir string
	rendered  bool
	err       error
}

func (g *shadowRenderGame) Update() error {
	if g.rendered {
		return ebiten.Termination
	}
	return nil
}

func (g *shadowRenderGame) Draw(_ *ebiten.Image) {
	g.err = g.renderImages()
	g.rendered = true
}

func (g *shadowRenderGame) Layout(_, _ int) (int, int) {
	return shadowTestCanvasSize, shadowTestCanvasSize
}

func (g *shadowRenderGame) renderImages() error {
	sprite := ebiten.NewImageFromImage(shadowTestSprite(shadowTestSpriteSize))
	for _, azimuth := range []int{0, 30, 60, 90, 120, 150, 180, 210, 240, 270, 300, 330} {
		projection := newCharacterShadowProjection(azimuth)
		canvas := ebiten.NewImageFromImage(shadowTestBackground(shadowTestCanvasSize))
		drawCharacterShadow(canvas, sprite, shadowTestDrawSize, shadowTestCenterX, shadowTestCenterY, 0.75, projection, true, shadowDarkenBlend)

		op := &ebiten.DrawImageOptions{}
		op.Filter = ebiten.FilterLinear
		op.GeoM.Scale(float64(shadowTestDrawSize)/shadowTestSpriteSize, float64(shadowTestDrawSize)/shadowTestSpriteSize)
		op.GeoM.Translate(shadowTestCenterX-shadowTestDrawSize/2, shadowTestCenterY-shadowTestDrawSize/2)
		canvas.DrawImage(sprite, op)

		name := fmt.Sprintf("shadow_sa_%03d_elev_%02.0f_len_%.2f.png", azimuth, characterShadowSunHeight(azimuth), projection.length)
		file, err := os.Create(filepath.Join(g.outputDir, name))
		if err != nil {
			return err
		}
		pixels := make([]byte, 4*shadowTestCanvasSize*shadowTestCanvasSize)
		canvas.ReadPixels(pixels)
		result := &image.RGBA{
			Pix:    pixels,
			Stride: 4 * shadowTestCanvasSize,
			Rect:   image.Rect(0, 0, shadowTestCanvasSize, shadowTestCanvasSize),
		}
		if err := png.Encode(file, result); err != nil {
			file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	return nil
}

func shadowTestBackground(size int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 188, G: 178, B: 148, A: 255}}, image.Point{}, draw.Src)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if x%32 == 0 || y%32 == 0 {
				img.SetRGBA(x, y, color.RGBA{R: 156, G: 148, B: 124, A: 255})
			}
		}
	}
	return img
}

func shadowTestSprite(size int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	fill := color.RGBA{R: 80, G: 120, B: 210, A: 255}
	draw.Draw(img, image.Rect(18, 13, 30, 35), &image.Uniform{C: fill}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(11, 18, 37, 24), &image.Uniform{C: fill}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(16, 34, 22, 48), &image.Uniform{C: fill}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(26, 34, 32, 48), &image.Uniform{C: fill}, image.Point{}, draw.Src)
	for y := 3; y < 16; y++ {
		for x := 17; x < 31; x++ {
			dx, dy := x-24, y-9
			if dx*dx+dy*dy <= 49 {
				img.SetRGBA(x, y, fill)
			}
		}
	}
	return img
}
