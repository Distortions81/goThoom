package main

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestRenderToolbarHandFitsHDItem(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_TOOLBAR_HANDS_TEST") == "" {
		t.Skip("set GOTHOOM_RENDER_TOOLBAR_HANDS_TEST=1 to verify HD held-item sizing")
	}
	game := &toolbarHandsRenderGame{}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

type toolbarHandsRenderGame struct {
	done bool
	err  error
}

func (g *toolbarHandsRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}

func (g *toolbarHandsRenderGame) Draw(_ *ebiten.Image) {
	defer func() { g.done = true }()
	hand := ebiten.NewImage(42, 42)
	defer hand.Deallocate()
	hand.Fill(color.RGBA{R: 255, A: 255})
	item := ebiten.NewImage(168, 168)
	defer item.Deallocate()
	item.SubImage(image.Rect(42, 42, 126, 126)).(*ebiten.Image).Fill(color.White)
	composite := overlayItemOnHand(hand, item, 0.25, 0.25)
	defer composite.Deallocate()
	if got := composite.Bounds().Size(); got.X != 168 || got.Y != 168 {
		g.err = fmt.Errorf("HD held-item composite = %v, want the artwork's 168x168 pixel density", got)
		return
	}
	pixels := make([]byte, 168*168*4)
	composite.ReadPixels(pixels)
	center, edge := (84*168+84)*4, 0
	if pixels[center] <= pixels[edge] || pixels[center+1] == 0 || pixels[edge+1] != 0 {
		g.err = fmt.Errorf("held item does not fit inside the hand slot: center=%v edge=%v", pixels[center:center+4], pixels[edge:edge+4])
	}
}

func (g *toolbarHandsRenderGame) Layout(_, _ int) (int, int) { return 48, 48 }
