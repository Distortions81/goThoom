package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestRenderTransientBubbleViews(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_TRANSIENT_VIEWS") == "" {
		t.Skip("set GOTHOOM_RENDER_TRANSIENT_VIEWS=1")
	}
	g := &transientViewsGame{t: t}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type transientViewsGame struct {
	t    *testing.T
	done bool
	err  error
}

func (*transientViewsGame) Layout(int, int) (int, int) { return 512, 256 }
func (g *transientViewsGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *transientViewsGame) Draw(*ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	oldMask := thoughtBubbleCompositeMask
	defer func() { thoughtBubbleCompositeMask = oldMask }()
	var results [2][]byte
	for variant := range 2 {
		canvas := ebiten.NewImage(512, 256)
		mask := ebiten.NewImage(512, 256)
		thoughtBubbleCompositeMask = mask
		pixels := make([]byte, 512*256*4)
		i := 0
		draw := func() {
			// Moving clips use a new rectangle each time. Read back only after
			// both clipped clear and source draw have been submitted/recycled.
			x, y := 5+i%400, 3+i/400
			region := image.Rect(x, y, x+12+i%7, y+16+i%9)
			i++
			mask.Fill(color.White)
			if variant == 0 {
				mask.SubImage(region).(*ebiten.Image).Clear()
			} else {
				bubbleBackgroundTarget(canvas, kBubbleThought, color.RGBA64{}, region)
			}
			// Cross the cleared region so incorrect clip reuse is visible.
			region = region.Add(image.Pt(2, 3))
			tint := color.RGBA{R: 80, G: 120, B: 160, A: 255}
			if variant == 0 {
				op := &ebiten.DrawImageOptions{}
				op.ColorScale.ScaleWithColor(tint)
				op.GeoM.Translate(float64(region.Min.X), float64(region.Min.Y))
				canvas.DrawImage(mask.SubImage(region).(*ebiten.Image), op)
			} else {
				compositeThoughtBubbleBackground(canvas, mask, tint, region)
			}
			canvas.ReadPixels(pixels)
		}
		allocs := testing.AllocsPerRun(1000, draw)
		g.t.Logf("recyclable=%v moving bubble clear/composite including readback: %.1f allocs/op", variant == 1, allocs)
		results[variant] = pixels
		canvas.Deallocate()
		mask.Deallocate()
	}
	if !bytes.Equal(results[0], results[1]) {
		g.err = fmt.Errorf("recycled moving bubble clips changed rendered pixels")
	}
}
