package eui

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Run alone: Ebitengine permits only one RunGame call per process.
func TestRenderPixelAlignedRect(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_RECT_TEST") == "" {
		t.Skip("set GOTHOOM_RENDER_RECT_TEST=1 to compare rectangle pixels")
	}
	g := &rectRenderGame{}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type rectRenderGame struct {
	done bool
	err  error
}

func (g *rectRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}

func (g *rectRenderGame) Layout(_, _ int) (int, int) { return 64, 64 }

func (g *rectRenderGame) Draw(_ *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	for _, background := range []color.Color{color.Transparent, color.RGBA{R: 30, G: 60, B: 90, A: 255}} {
		for _, fill := range []color.Color{color.White, color.NRGBA{R: 150, G: 90, B: 30, A: 128}} {
			for _, box := range [][4]float32{{5.4, 7.6, 20.5, 13.4}, {-3.6, 3.1, 24.4, 18.9}, {7, 8, 0, 12}, {8, 9, -4, -5}} {
				before, after := ebiten.NewImage(64, 64), ebiten.NewImage(64, 64)
				before.Fill(background)
				after.Fill(background)
				// Nonzero subimage bounds also exercise clipping against a pane.
				clip := image.Rect(3, 4, 50, 51)
				rounded := box
				for i := range rounded {
					rounded[i] = float32(math.Round(float64(rounded[i])))
				}
				vector.FillRect(before.SubImage(clip).(*ebiten.Image), rounded[0], rounded[1], rounded[2], rounded[3], fill, true)
				drawFilledRect(after.SubImage(clip).(*ebiten.Image), box[0], box[1], box[2], box[3], fill, true)
				a, b := make([]byte, 64*64*4), make([]byte, 64*64*4)
				before.ReadPixels(a)
				after.ReadPixels(b)
				before.Deallocate()
				after.Deallocate()
				for i := range a {
					// Different shader paths may round a color channel by one.
					if delta := int(a[i]) - int(b[i]); delta < -1 || delta > 1 {
						g.err = fmt.Errorf("rectangle %v differs at byte %d: before=%d after=%d", box, i, a[i], b[i])
						return
					}
				}
			}
		}
	}
}
