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

func TestRenderLegacyTriggerEditors(t *testing.T) {
	dir := os.Getenv("GOTHOOM_RENDER_MACRO_EDITORS")
	if dir == "" {
		t.Skip("opt-in native macro editor render check")
	}
	doc := triggerEditorFixture(t, []byte("// Name: Basic Commands\n\"/lookaround\" \"/look\\r\"\n'pp' \"/ponder \"\nf6 \"/look\\r\"\ncontrol-click2 \"/info\\r\"\n"), false)
	initFont()
	eui.SetScreenSize(1920, 1080)
	if err := eui.LoadTheme("AccentDark"); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	game := &legacyTriggerRenderGame{dir: dir, entry: doc.entry}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

type legacyTriggerRenderGame struct {
	dir   string
	entry legacyMacroLibraryEntry
	done  bool
	err   error
}

func (g *legacyTriggerRenderGame) Layout(_, _ int) (int, int) { return 1920, 1080 }
func (g *legacyTriggerRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *legacyTriggerRenderGame) Draw(screen *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	g.err = g.render(screen)
}
func (g *legacyTriggerRenderGame) render(screen *ebiten.Image) error {
	for _, scale := range []float32{1, 2} {
		eui.SetUIScale(scale)
		for _, keys := range []bool{false, true} {
			win := openLegacyTriggerEditor(g.entry, keys)
			if win == nil || !win.IsOpen() {
				return fmt.Errorf("editor did not open")
			}
			win.SetPos(eui.Point{X: 0, Y: 0})
			win.OnResize()
			win.Refresh()
			screen.Clear()
			eui.Draw(screen)
			if err := checkRenderedControlText(win.Contents, scale); err != nil {
				return err
			}
			pos, size := win.GetPos(), win.GetSize()
			var check func([]*eui.ItemData) error
			check = func(items []*eui.ItemData) error {
				for _, it := range items {
					if it.ItemType == eui.ITEM_BUTTON && (it.Text == "Save" || it.Text == "Cancel") && (it.DrawRect.Y1 > pos.Y+size.Y-4*scale || it.DrawRect.X1 > pos.X+size.X-4*scale) {
						return fmt.Errorf("%s footer clipped at %gx", it.Text, scale)
					}
					if err := check(it.Contents); err != nil {
						return err
					}
				}
				return nil
			}
			if err := check(win.Contents); err != nil {
				return err
			}
			file, err := os.Create(filepath.Join(g.dir, fmt.Sprintf("keys-%t-%gx.png", keys, scale)))
			if err != nil {
				return err
			}
			err = png.Encode(file, screen)
			closeErr := file.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
			win.Close()
		}
	}
	return nil
}
