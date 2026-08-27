package main

import (
	"fmt"
	"image/color"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestRenderFrameBlendPixels(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_FRAME_BLEND_TEST") == "" {
		t.Skip("set GOTHOOM_RENDER_FRAME_BLEND_TEST=1 to verify frame-blend pixels")
	}
	if err := ReloadSpriteUpscaleShader(); err != nil {
		t.Fatalf("compile artwork shaders: %v", err)
	}
	game := &frameBlendRenderGame{}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

type frameBlendRenderGame struct {
	rendered bool
	err      error
}

func (g *frameBlendRenderGame) Update() error {
	if g.rendered {
		return ebiten.Termination
	}
	return nil
}

func (g *frameBlendRenderGame) Draw(_ *ebiten.Image) {
	previous := ebiten.NewImage(2, 2)
	previous.Fill(color.RGBA{R: 255, A: 255})
	current := ebiten.NewImage(4, 2)
	current.Fill(color.RGBA{B: 255, A: 255})
	destination := ebiten.NewImage(4, 2)
	if !drawFrameBlend(destination, previous, current, frameBlendDrawOptions{
		ScaleX: 1, ScaleY: 1, Fade: 0.25,
		Red: 1, Green: 1, Blue: 1, Alpha: 1,
	}) {
		g.err = fmt.Errorf("frame blend draw was not available")
		g.rendered = true
		return
	}
	pixels := make([]byte, 4*4*2)
	destination.ReadPixels(pixels)
	if err := checkFrameBlendPixel(pixels, 4, 0, 0, color.RGBA{B: 64, A: 64}); err != nil {
		g.err = err
	} else if err := checkFrameBlendPixel(pixels, 4, 1, 0, color.RGBA{R: 191, B: 64, A: 255}); err != nil {
		g.err = err
	}
	if g.err == nil {
		fadedDestination := ebiten.NewImage(4, 2)
		if !drawFrameBlend(fadedDestination, previous, current, frameBlendDrawOptions{
			ScaleX: 1, ScaleY: 1, Fade: 0.25,
			Red: 1, Green: 1, Blue: 1, Alpha: 0.4,
		}) {
			g.err = fmt.Errorf("faded frame blend draw was not available")
		} else {
			fadedPixels := make([]byte, 4*4*2)
			fadedDestination.ReadPixels(fadedPixels)
			if err := checkFrameBlendPixel(fadedPixels, 4, 1, 0, color.RGBA{R: 77, B: 26, A: 102}); err != nil {
				g.err = fmt.Errorf("faded %w", err)
			}
		}
	}
	g.rendered = true
}

func (g *frameBlendRenderGame) Layout(_, _ int) (int, int) { return 4, 2 }

func checkFrameBlendPixel(pixels []byte, width, x, y int, want color.RGBA) error {
	offset := (y*width + x) * 4
	got := color.RGBA{R: pixels[offset], G: pixels[offset+1], B: pixels[offset+2], A: pixels[offset+3]}
	for _, values := range [][2]uint8{{got.R, want.R}, {got.G, want.G}, {got.B, want.B}, {got.A, want.A}} {
		delta := int(values[0]) - int(values[1])
		if delta < -1 || delta > 1 {
			return fmt.Errorf("frame blend pixel (%d,%d) = %#v, want %#v", x, y, got, want)
		}
	}
	return nil
}
