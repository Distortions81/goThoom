package eui

import (
	"fmt"
	"image/color"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font/gofont/goregular"
)

// Run alone because Ebitengine permits one RunGame call per process.
func TestTooltipPreservesButtonCaptionPixels(t *testing.T) {
	if os.Getenv("GOTHOOM_TOOLTIP_RENDER_TEST") == "" {
		t.Skip("set GOTHOOM_TOOLTIP_RENDER_TEST=1 for pixel comparison")
	}
	if err := EnsureFontSource(goregular.TTF); err != nil {
		t.Fatal(err)
	}
	previousScale := uiScale
	t.Cleanup(func() { uiScale = previousScale })
	g := &tooltipRenderGame{}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type tooltipRenderGame struct {
	done bool
	err  error
}

func (g *tooltipRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *tooltipRenderGame) Layout(_, _ int) (int, int) { return 1400, 100 }
func (g *tooltipRenderGame) Draw(_ *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	icon := ebiten.NewImage(24, 24)
	icon.Fill(color.White)
	defer icon.Deallocate()
	for _, scale := range []float32{1, 1.3, 2} {
		uiScale = scale
		for _, caption := range []string{"Open User Data Folder", "Open Diagnostics Folder", "File Paths"} {
			for _, leadingIcon := range []*ebiten.Image{nil, icon} {
				win := NewWindow()
				win.ShowTooltipIndicators = true
				button, _ := NewButton()
				button.Text, button.FontSize = caption, 12
				button.Image = leadingIcon
				button.Filled, button.Outlined = false, false
				win.AddItem(button)
				w, h := int(660*scale), int(32*scale)
				before, after := ebiten.NewImage(w, h), ebiten.NewImage(w, h)
				clip := rect{X1: float32(w), Y1: float32(h)}
				// The help marker reserves 18 logical pixels. The rest must render
				// exactly like the same button in the remaining space, with no lost text.
				button.drawItemInternal(point{}, point{}, point{X: float32(w) - 18*scale, Y: float32(h)}, clip, before)
				button.SetTooltip("Useful details")
				button.drawItemInternal(point{}, point{}, point{X: float32(w), Y: float32(h)}, clip, after)
				a, b := make([]byte, w*h*4), make([]byte, w*h*4)
				before.ReadPixels(a)
				after.ReadPixels(b)
				before.Deallocate()
				after.Deallocate()
				win.RemoveWindow()
				for i := 0; i < len(a); i += 4 {
					if a[i+3] == 0 {
						continue
					}
					for c := 0; c < 4; c++ {
						if d := int(a[i+c]) - int(b[i+c]); d < -1 || d > 1 {
							g.err = fmt.Errorf("%q at %.2fx (leading icon=%v): caption/image pixel changed at %d", caption, scale, leadingIcon != nil, i/4)
							return
						}
					}
				}
			}
		}
	}
}
