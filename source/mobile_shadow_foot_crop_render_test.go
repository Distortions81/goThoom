package main

import (
	"fmt"
	"image/color"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestRenderUprightSunShadowCropsFeet(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_SHADOW_FOOT_CROP_TEST") == "" {
		t.Skip("set GOTHOOM_RENDER_SHADOW_FOOT_CROP_TEST=1 to verify sun-shadow foot pixels")
	}
	game := &shadowFootCropRenderGame{}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

type shadowFootCropRenderGame struct {
	done bool
	err  error
}

func (g *shadowFootCropRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}

func (g *shadowFootCropRenderGame) Draw(_ *ebiten.Image) {
	defer func() { g.done = true }()
	sprite := ebiten.NewImage(32, 32)
	defer sprite.Deallocate()
	sprite.Fill(color.White)
	texture := characterShadowTexture{image: sprite, contentSize: 32, footY: 32}
	if got := uprightShadowFootCutoff(texture); got < 30 || got >= 31 {
		g.err = fmt.Errorf("shadow foot cutoff = %v, want a small crop", got)
		return
	}
	withCrop, withoutCrop := ebiten.NewImage(32, 32), ebiten.NewImage(32, 32)
	defer withCrop.Deallocate()
	defer withoutCrop.Deallocate()
	var geo ebiten.GeoM
	drawUprightShadowTexture(withCrop, texture, geo, 1, shadowMaskBlend, true)
	drawUprightShadowTexture(withoutCrop, texture, geo, 1, shadowMaskBlend, false)
	cropped, full := make([]byte, 32*32*4), make([]byte, 32*32*4)
	withCrop.ReadPixels(cropped)
	withoutCrop.ReadPixels(full)
	upper, foot := (10*32+16)*4+3, (31*32+16)*4+3
	if cropped[upper] == 0 || cropped[upper] != full[upper] || full[foot] == 0 || cropped[foot] >= full[foot] {
		g.err = fmt.Errorf("upright sun-shadow crop changed body or retained feet: body=%d/%d feet=%d/%d", cropped[upper], full[upper], cropped[foot], full[foot])
	}
}

func (g *shadowFootCropRenderGame) Layout(_, _ int) (int, int) { return 32, 32 }
