package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
)

// Run alone: verifies the complete pages, including navigation, after layout.
func TestRenderSetupWizardLayout(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_WIZARD_LAYOUT") == "" {
		t.Skip("set GOTHOOM_RENDER_WIZARD_LAYOUT=1")
	}
	gs = gsdef
	initFont()
	if err := eui.LoadTheme("AccentDark"); err != nil {
		t.Fatal(err)
	}
	eui.SetScreenSize(1920, 1200)
	setupWizardWin = eui.NewWindow()
	setupWizardWin.ShowTooltipIndicators = true
	setupWizardWin.Closable = false
	setupWizardWin.AutoSize = true
	setupWizardWin.Padding = 10
	setupWizardWin.BorderPad = 4
	setupWizardWin.AddWindow(false)
	setupWizardWin.MarkOpen()
	dir := os.Getenv("GOTHOOM_WIZARD_LAYOUT_DIR")
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	g := &wizardLayoutRenderGame{dir: dir}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type wizardLayoutRenderGame struct {
	done bool
	dir  string
	err  error
}

func (g *wizardLayoutRenderGame) Layout(_, _ int) (int, int) { return 1920, 1200 }
func (g *wizardLayoutRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *wizardLayoutRenderGame) Draw(screen *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	loadMaterialIcons()
	for _, style := range []string{"Breeze", "HighContrast"} {
		if g.err = eui.LoadStyle(style); g.err != nil {
			return
		}
		for _, scale := range []float32{1, 1.25, 1.5} {
			eui.SetUIScale(scale)
			for _, tiled := range []bool{false, true} {
				gs.TiledWindows = tiled
				for _, page := range []int{setupWizardInterfacePage, setupWizardLayoutPage} {
					setupWizardPage = page
					rebuildSetupWizard()
					setupWizardWin.Dirty = true
					screen.Clear()
					eui.Draw(screen)
					size := setupWizardWin.GetSize()
					if size.Y > 650*scale || size.X > 850*scale {
						g.err = fmt.Errorf("%s page %d tiled=%v scale %.2f: window %.0fx%.0f exceeds compact page budget", style, page, tiled, scale, size.X, size.Y)
						return
					}
					if g.dir != "" && style == "Breeze" && scale == 1 && tiled {
						f, err := os.Create(filepath.Join(g.dir, fmt.Sprintf("page-%d.png", page+1)))
						if err != nil {
							g.err = err
							return
						}
						g.err = png.Encode(f, screen.SubImage(setupWizardWin.Render.Bounds()))
						_ = f.Close()
						if g.err != nil {
							return
						}
					}
				}
			}
		}
	}
}
