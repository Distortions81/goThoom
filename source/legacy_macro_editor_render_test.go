package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"gothoom/eui"
)

func TestRenderMacroSourceEditor(t *testing.T) {
	dir := os.Getenv("GOTHOOM_RENDER_MACRO_SOURCE_EDITOR")
	if dir == "" {
		t.Skip("set GOTHOOM_RENDER_MACRO_SOURCE_EDITOR to a capture directory")
	}
	ed := macroEditorFixture(t)
	if theme := os.Getenv("GOTHOOM_MACRO_EDITOR_THEME"); theme != "" {
		original := eui.CurrentThemeName()
		t.Cleanup(func() { _ = eui.LoadTheme(original) })
		if err := eui.LoadTheme(theme); err != nil {
			t.Fatal(err)
		}
		updateSourceEditors()
	}
	editMacroForTest(ed, "// Name: Hello\n// Say hello with a typed command.\n\n\"/hello\"\n{\n\t\"/think Hello, \" @text \"!\\r\"\n}\n\ncontrol-h \"/think Hello!\\r\"\n")
	editMacroForTest(ed, ed.input.Text+"// "+strings.Repeat("Long macro comment. ", 12)+"\n"+strings.Repeat("// Another line\n", 50))
	ed.input.CursorPos, ed.input.SelectStart, ed.input.SelectEnd = 0, 0, 0
	ed.win.OpenSearch()
	ed.win.SearchText = "hello"
	ed.win.OnSearch("hello")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	g := &sourceEditorRenderGame{editor: ed, dir: dir, prefix: "macro"}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type sourceEditorRenderGame struct {
	editor *sourceEditor
	dir    string
	prefix string
	done   bool
	err    error
}

func (*sourceEditorRenderGame) Layout(_, _ int) (int, int) { return 1920, 1080 }
func (g *sourceEditorRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *sourceEditorRenderGame) Draw(screen *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	g.err = g.render(screen)
}
func (g *sourceEditorRenderGame) render(screen *ebiten.Image) error {
	for _, tc := range []struct{ scale, width, height float32 }{{1, 780, 540}, {2, 780, 500}, {2, 360, 400}} {
		eui.SetUIScale(tc.scale)
		ed := g.editor
		ed.win.Size = eui.Point{X: tc.width, Y: tc.height}
		ed.layout()
		wantRows := 1
		if tc.width < 500 {
			wantRows = 2
		}
		if len(ed.footer.Contents) != wantRows {
			return fmt.Errorf("footer rows %d, want %d at width %g", len(ed.footer.Contents), wantRows, tc.width)
		}
		screen.Clear()
		eui.Draw(screen)
		if err := checkRenderedControlText(ed.win.Contents, tc.scale); err != nil {
			return err
		}
		pos, size := ed.win.GetPos(), ed.win.GetSize()
		var check func([]*eui.ItemData) error
		check = func(items []*eui.ItemData) error {
			for _, it := range items {
				if it.ItemType == eui.ITEM_BUTTON && (it.DrawRect.X1-it.DrawRect.X0 < it.GetSize().X-1 || it.DrawRect.Y1-it.DrawRect.Y0 < it.GetSize().Y-1 || it.DrawRect.X1 > pos.X+size.X || it.DrawRect.Y1 > pos.Y+size.Y) {
					return fmt.Errorf("%s clipped at %gx width %g", it.Text, tc.scale, tc.width)
				}
				if err := check(it.Contents); err != nil {
					return err
				}
			}
			return nil
		}
		if err := check(ed.win.Contents); err != nil {
			return err
		}
		if ed.input.DrawRect.Y1 <= ed.input.DrawRect.Y0 || ed.input.DrawRect.Y1 > ed.status.DrawRect.Y0 {
			return fmt.Errorf("editor overlaps status")
		}
		f, err := os.Create(filepath.Join(g.dir, fmt.Sprintf("%s-%gx-%g.png", g.prefix, tc.scale, tc.width)))
		if err != nil {
			return err
		}
		err = png.Encode(f, screen)
		f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
