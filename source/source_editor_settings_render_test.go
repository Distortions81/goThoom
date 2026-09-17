package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"gothoom/eui"
)

func TestRenderEditorSettings(t *testing.T) {
	dir := os.Getenv("GOTHOOM_RENDER_EDITOR_SETTINGS")
	if dir == "" {
		t.Skip("set GOTHOOM_RENDER_EDITOR_SETTINGS to a capture directory")
	}
	editorSettingsFixture(t)
	for _, ed := range sourceEditors {
		ed.discard = true
		ed.win.Close()
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	g := &editorSettingsRenderGame{dir: dir}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type editorSettingsRenderGame struct {
	dir  string
	done bool
	err  error
}

func (*editorSettingsRenderGame) Layout(_, _ int) (int, int) { return 1920, 1080 }
func (g *editorSettingsRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *editorSettingsRenderGame) Draw(screen *ebiten.Image) {
	if !g.done {
		g.done = true
		g.err = g.render(screen)
	}
}
func (g *editorSettingsRenderGame) render(screen *ebiten.Image) error {
	panel := sourceEditorSettings
	for _, theme := range []string{"AccentDark", "AccentLight"} {
		if err := eui.LoadTheme(theme); err != nil {
			return err
		}
		for _, scale := range []float32{1, 2} {
			eui.SetUIScale(scale)
			gs.EditorUseCustomColors, gs.EditorSyntaxColors = scale == 2, nil
			panel.refresh()
			_ = panel.win.SetPos(eui.Point{X: 20, Y: 20})
			screen.Clear()
			eui.Draw(screen)
			if err := checkRenderedControlText(panel.win.Contents, scale); err != nil {
				return err
			}
			f, err := os.Create(filepath.Join(g.dir, fmt.Sprintf("editor-settings-%s-%gx.png", theme, scale)))
			if err != nil {
				return err
			}
			err = png.Encode(f, screen)
			f.Close()
			if err != nil {
				return err
			}
		}
	}
	return nil
}
