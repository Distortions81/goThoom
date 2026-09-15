package main

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestRenderUprightSunShadowKeepsFullSilhouette(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_SHADOW_BASELINE_TEST") == "" {
		t.Skip("set GOTHOOM_RENDER_SHADOW_BASELINE_TEST=1 to verify upright sun-shadow foot pixels")
	}
	game := &shadowBaselineRenderGame{}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

type shadowBaselineRenderGame struct {
	done bool
	err  error
}

func (g *shadowBaselineRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}

func (g *shadowBaselineRenderGame) Draw(_ *ebiten.Image) {
	defer func() { g.done = true }()
	sprite := ebiten.NewImage(32, 32)
	defer sprite.Deallocate()
	sprite.Fill(color.RGBA{R: 255, G: 255, B: 255, A: 128})
	sprite.SubImage(image.Rect(0, 30, 32, 32)).(*ebiten.Image).Fill(color.White)
	texture := characterShadowTexture{image: sprite, contentSize: 32, footY: 32}
	canvas := ebiten.NewImage(32, 32)
	defer canvas.Deallocate()
	var geo ebiten.GeoM
	drawUprightShadowTexture(canvas, texture, geo, 1, shadowMaskBlend)
	if got := uprightShadowVertices[2].SrcY; got != 32 {
		g.err = fmt.Errorf("shadow source ends at y=%v, want the full 32-pixel silhouette", got)
		return
	}
	pixels := make([]byte, 32*32*4)
	canvas.ReadPixels(pixels)
	body, feet := pixels[(10*32+16)*4+3], pixels[(31*32+16)*4+3]
	if body == 0 || feet <= body {
		g.err = fmt.Errorf("upright sun shadow lost its feet: body=%d feet=%d", body, feet)
	}
}

func (g *shadowBaselineRenderGame) Layout(_, _ int) (int, int) { return 32, 32 }
