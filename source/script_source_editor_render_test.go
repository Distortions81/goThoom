package main

import (
	"os"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestRenderScriptSourceEditor(t *testing.T) {
	dir := os.Getenv("GOTHOOM_RENDER_SCRIPT_SOURCE_EDITOR")
	if dir == "" {
		t.Skip("set GOTHOOM_RENDER_SCRIPT_SOURCE_EDITOR to a capture directory")
	}
	ed := scriptSourceEditorFixture(t, false)
	for _, other := range sourceEditors {
		if other != ed {
			other.discard = true
			other.win.Close()
		}
	}
	editMacroForTest(ed, editorScriptSource+"// "+strings.Repeat("long comment ", 14)+"\n"+strings.Repeat("// More source\n", 50))
	ed.win.OpenSearch()
	ed.win.SearchText = "Version"
	ed.win.OnSearch("Version")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	g := &sourceEditorRenderGame{editor: ed, dir: dir, prefix: "script"}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}
