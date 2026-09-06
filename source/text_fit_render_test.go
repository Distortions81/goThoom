package main

import (
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"gothoom/eui"
)

// Run alone, since this test starts Ebitengine's game loop.
func TestRenderControlTextFit(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_TEXT_FIT") == "" {
		t.Skip("set GOTHOOM_RENDER_TEXT_FIT=1")
	}
	initFont()
	eui.SetScreenSize(1600, 1200)
	eui.SetUIScale(1)
	if err := eui.LoadTheme("AccentDark"); err != nil {
		t.Fatal(err)
	}
	makeSettingsWindow()
	settingsWin.MarkOpen()
	_ = settingsWin.SetPos(eui.Point{X: 20, Y: 20})
	dir := os.Getenv("GOTHOOM_TEXT_FIT_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	g := &controlTextFitGame{dir: dir}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type controlTextFitGame struct {
	done bool
	err  error
	dir  string
}

func (g *controlTextFitGame) Layout(_, _ int) (int, int) { return 1600, 1200 }
func (g *controlTextFitGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *controlTextFitGame) Draw(screen *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	loadMaterialIcons()
	for _, style := range []string{"Breeze", "Flat", "Borderless", "Rounded", "Outline", "HighContrast"} {
		if g.err = eui.LoadStyle(style); g.err != nil {
			return
		}
		for _, scale := range []float32{1, 1.25, 1.5} {
			eui.SetUIScale(scale)
			for i, tab := range settingsWin.Contents[0].Tabs {
				settingsWin.Contents[0].ActiveTab = i
				settingsWin.Refresh()
				settingsWin.Dirty = true
				screen.Clear()
				eui.Draw(screen)
				for _, header := range settingsWin.Contents[0].Tabs {
					if header.DrawRect.Y0 != settingsWin.Contents[0].Tabs[0].DrawRect.Y0 {
						g.err = fmt.Errorf("Settings tabs wrapped at %.2fx", scale)
						return
					}
				}
				if g.err = checkRenderedControlText(settingsWin.Contents, scale); g.err != nil {
					g.err = fmt.Errorf("%s/%s at %.2fx: %w", style, tab.Name, scale, g.err)
					return
				}
				if style == "Breeze" && scale == 1 && (tab.Name == "Display" || tab.Name == "Performance") {
					f, err := os.Create(filepath.Join(g.dir, strings.ToLower(tab.Name)+".png"))
					if err != nil {
						g.err = err
						return
					}
					g.err = png.Encode(f, screen)
					_ = f.Close()
					if g.err != nil {
						return
					}
				}
			}
		}
	}
	settingsWin.Close()
	for _, docked := range []bool{false, true} {
		win := eui.NewWindow()
		win.AutoSize = true
		win.AddItem(buildToolbarRoot(docked))
		win.AddWindow(false)
		win.MarkOpen()
		_ = win.SetPos(eui.Point{X: 20, Y: 20})
		loadMaterialIcons()
		for _, scale := range []float32{1, 1.25, 1.5, 2} {
			eui.SetUIScale(scale)
			win.Refresh()
			win.Dirty = true
			screen.Clear()
			eui.Draw(screen)
			if g.err = checkRenderedControlText(win.Contents, scale); g.err != nil {
				g.err = fmt.Errorf("toolbar docked=%t at %.2fx: %w", docked, scale, g.err)
				return
			}
		}
		win.RemoveWindow()
	}

}

func checkRenderedControlText(items []*eui.ItemData, scale float32) error {
	for _, it := range items {
		if len(it.Tabs) > 0 {
			for _, tab := range it.Tabs {
				if tab.DrawRect.X0 < it.DrawRect.X0 || tab.DrawRect.X1 > it.DrawRect.X1+1 {
					return fmt.Errorf("tab %q outside panel: %v / %v", tab.Name, tab.DrawRect, it.DrawRect)
				}
			}
			if err := checkRenderedControlText(it.Tabs[it.ActiveTab].Contents, scale); err != nil {
				return err
			}
		} else if err := checkRenderedControlText(it.Contents, scale); err != nil {
			return err
		}
		if it.ItemType != eui.ITEM_BUTTON || it.Text == "" {
			continue
		}
		face := *mainFont.(*text.GoTextFace)
		face.Size = float64(it.FontSize*scale + 2)
		if it.ParentWindow.DefaultButton == it {
			face.Source = mainFontBold.(*text.GoTextFace).Source
		}
		var width float64
		for line := range strings.SplitSeq(it.Text, "\n") {
			w, _ := text.Measure(line, &face, 0)
			width = math.Max(width, w)
		}
		need := width + 6*float64(scale)
		if it.Image != nil {
			need += 21 * float64(scale)
		}
		if it.Tooltip != "" && it.ParentWindow.ShowTooltipIndicators {
			need += 18 * float64(scale)
		}
		if need > float64(it.DrawRect.X1-it.DrawRect.X0)+1 {
			return fmt.Errorf("button %q needs %.1f, has %.1f", it.Text, need, it.DrawRect.X1-it.DrawRect.X0)
		}
	}
	return nil
}
