package main

import (
	"fmt"
	"image/color"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestRenderMobileFlashPixels(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_MOBILE_FLASH") == "" {
		t.Skip("set GOTHOOM_RENDER_MOBILE_FLASH=1; run alone")
	}
	if err := ReloadSpriteUpscaleShader(); err != nil {
		t.Fatal(err)
	}
	game := &mobileFlashRenderGame{}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

type mobileFlashRenderGame struct {
	done bool
	err  error
}

func (g *mobileFlashRenderGame) Layout(_, _ int) (int, int) { return 3, 1 }
func (g *mobileFlashRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *mobileFlashRenderGame) Draw(_ *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	source := ebiten.NewImage(3, 1)
	defer source.Deallocate()
	colors := []color.RGBA{{A: 255}, {R: 4, G: 16, B: 32, A: 128}, {}}
	for x, c := range colors {
		source.Set(x, 0, c)
	}
	influence := ebiten.NewImage(3, 1)
	defer influence.Deallocate()
	palette := testMobilePalette(makeMobileKey(400, 0, []byte{1}), 0, 0, 0, 0)
	target := ebiten.NewImage(3, 1)
	defer target.Deallocate()
	paths := []string{"image", "blend", "recolor", "recolor-blend"}
	for _, path := range paths {
		for _, opacity := range []float32{1, .5} {
			// The final zero checks that cached shader uniforms stop flashing.
			for _, strength := range []float32{0, .5, 1, 0} {
				target.Clear()
				op := frameBlendDrawOptions{ScaleX: 1, ScaleY: 1, Fade: .35, Red: .2, Green: .3, Blue: .4, Alpha: opacity, FlashColor: [4]float32{1, .2, .1, strength}}
				var ok bool
				switch path {
				case "image":
					drawMobileFlashImage(target, source, op)
					ok = true
				case "blend":
					ok = drawFrameBlend(target, source, source, op)
				case "recolor":
					ok = drawRecoloredMobile(target, source, influence, palette, op)
				case "recolor-blend":
					ok = drawRecoloredMobileFrameBlend(target, source, influence, source, influence, palette, palette, op)
				}
				if !ok {
					g.err = fmt.Errorf("%s flash path unavailable", path)
					return
				}
				pixels := make([]byte, 12)
				target.ReadPixels(pixels)
				for x, c := range colors {
					a := float32(c.A) * opacity
					channel := func(value uint8, shade, flash float32) uint8 {
						return uint8(float32(value)*shade*opacity*(1-strength) + flash*a*strength + .5)
					}
					want := color.RGBA{R: channel(c.R, op.Red, 1), G: channel(c.G, op.Green, .2), B: channel(c.B, op.Blue, .1), A: uint8(a + .5)}
					if err := checkFrameBlendPixel(pixels, 3, x, 0, want); err != nil {
						g.err = fmt.Errorf("%s flash=%g opacity=%g: %w", path, strength, opacity, err)
						return
					}
				}
			}
		}
	}
}
