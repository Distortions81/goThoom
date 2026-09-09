package main

import (
	"fmt"
	"image/color"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestRenderMobileOutlineColor(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_OUTLINE") == "" {
		t.Skip("set GOTHOOM_RENDER_OUTLINE=1; run alone")
	}
	game := &mobileOutlineColorGame{}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

type mobileOutlineColorGame struct {
	done bool
	err  error
}

func (g *mobileOutlineColorGame) Layout(_, _ int) (int, int) { return 32, 32 }
func (g *mobileOutlineColorGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *mobileOutlineColorGame) Draw(_ *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	sprite := ebiten.NewImage(3, 3)
	defer sprite.Deallocate()
	target := ebiten.NewImage(32, 32)
	defer target.Deallocate()
	for _, alpha := range []uint8{255, 128} {
		sprite.Set(1, 1, color.RGBA{A: alpha}) // black artwork, transparent surrounding pixels
		for _, tint := range []scriptMobileTint{{r: 255, g: 64, b: 32, a: 255}, {r: 32, g: 192, b: 255, a: 128}} {
			target.Clear()
			drawMobileSpriteOutline(target, sprite, 10, 10, 1, tint)
			got := color.RGBAModel.Convert(target.At(9, 11)).(color.RGBA)
			a := float64(alpha) * float64(tint.a) / 255
			want := color.RGBA{R: uint8(float64(tint.r) * a / 255), G: uint8(float64(tint.g) * a / 255), B: uint8(float64(tint.b) * a / 255), A: uint8(a)}
			for i, pair := range [][2]uint8{{got.R, want.R}, {got.G, want.G}, {got.B, want.B}, {got.A, want.A}} {
				if delta := int(pair[0]) - int(pair[1]); delta < -2 || delta > 2 {
					g.err = fmt.Errorf("black sprite outline channel %d: got %v, want %v", i, got, want)
					return
				}
			}
			if _, _, _, a := target.At(0, 0).RGBA(); a != 0 {
				g.err = fmt.Errorf("outline filled transparent space")
				return
			}
		}
	}
}
