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

// Run alone: this visual check uses Ebitengine's game loop.
func TestRenderColorPicker(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_COLOR_PICKER") == "" {
		t.Skip("set GOTHOOM_RENDER_COLOR_PICKER=1")
	}
	initFont()
	eui.SetScreenSize(960, 800)
	eui.SetUIScale(1)
	eui.SetWindowShadows(true)
	if err := eui.LoadTheme("SeaGlass"); err != nil {
		t.Fatal(err)
	}
	makeSettingsWindow()
	settingsWin.MarkOpen()
	_ = settingsWin.SetPos(eui.Point{X: 20, Y: 20})
	openColorPicker("Accent Color", eui.AccentColor(), func(eui.Color) {})
	_ = colorPickerWin.SetPos(eui.Point{X: 270, Y: 160})
	dir := os.Getenv("GOTHOOM_COLOR_PICKER_RENDER_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	g := &colorPickerRenderGame{path: filepath.Join(dir, "color-picker.png")}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type colorPickerRenderGame struct {
	done bool
	path string
	err  error
}

func (g *colorPickerRenderGame) Layout(_, _ int) (int, int) { return 960, 800 }
func (g *colorPickerRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *colorPickerRenderGame) Draw(_ *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	g.err = g.capture(g.path)
	if g.err != nil {
		return
	}
	colorPickerWin.HoverClose = true
	colorPickerWin.Refresh()
	g.err = g.capture(filepath.Join(filepath.Dir(g.path), "color-picker-close-hover.png"))
	if g.err != nil {
		return
	}
	g.err = verifyColorPickerControlBounds()
	if g.err != nil {
		return
	}
	colorPickerWin.Close()
	settingsWin.Close()
	makeTextColorsWindow()
	textColorsWin.MarkOpen()
	_ = textColorsWin.SetPos(eui.Point{X: 40, Y: 40})
	g.err = g.capture(filepath.Join(filepath.Dir(g.path), "text-color-swatches.png"))
}

func verifyColorPickerControlBounds() error {
	canvas := ebiten.NewImage(960, 800)
	defer canvas.Deallocate()
	defer eui.SetUIScale(1)
	defer eui.LoadStyle("Borderless")
	for _, style := range []string{"Breeze", "Borderless", "Flat", "Rounded", "Outline", "HighContrast"} {
		if err := eui.LoadStyle(style); err != nil {
			return err
		}
		for _, scale := range []float32{1, 1.5} {
			eui.SetUIScale(scale)
			_ = colorPickerWin.SetPos(eui.Point{X: 20, Y: 20})
			colorPickerWin.Refresh()
			canvas.Clear()
			eui.Draw(canvas)
			var check func([]*eui.ItemData) error
			check = func(items []*eui.ItemData) error {
				for _, item := range items {
					if item.ItemType == eui.ITEM_INPUT || item.ItemType == eui.ITEM_SLIDER {
						r, size := item.DrawRect, item.GetSize()
						if r.X1-r.X0 < size.X-0.1 || r.Y1-r.Y0 < size.Y-0.1 {
							return fmt.Errorf("%s at scale %.1f clipped picker control: %v, want %v", style, scale, r, size)
						}
					}
					if err := check(item.Contents); err != nil {
						return err
					}
				}
				return nil
			}
			if err := check(colorPickerWin.Contents); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *colorPickerRenderGame) capture(path string) error {
	canvas := ebiten.NewImage(960, 800)
	defer canvas.Deallocate()
	canvas.Fill(eui.NewColor(49, 57, 61, 255))
	eui.Draw(canvas)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, canvas)
}
