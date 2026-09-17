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

func TestRenderNotesAndConfigEditors(t *testing.T) {
	dir := os.Getenv("GOTHOOM_RENDER_NOTES_EDITORS")
	if dir == "" {
		t.Skip("set GOTHOOM_RENDER_NOTES_EDITORS to a capture directory")
	}
	personalNotesFixture(t)
	oldActive := storagePathsActivated
	oldTheme, oldStyle := eui.CurrentThemeName(), eui.CurrentStyleName()
	eui.SetUserDataRoot(t.TempDir())
	gs.AssetsPath, storagePathsActivated = t.TempDir(), false
	t.Cleanup(func() {
		storagePathsActivated = oldActive
		eui.SetUserDataRoot("")
		_ = eui.LoadTheme(oldTheme)
		_ = eui.LoadStyle(oldStyle)
	})
	if err := eui.LoadTheme("AccentDark"); err != nil {
		t.Fatal(err)
	}
	note, err := createPersonalNote("Training plan", "practice, weekly", "Gaia")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := createPersonalNote("Hunt supplies and preparation for a long expedition", "hunt, supplies, preparation", ""); err != nil {
		t.Fatal(err)
	}
	showPersonalNotes()
	details := openPersonalNoteDetails(note, "Gaia")
	ed := openPersonalNote(note)
	editMacroForTest(ed, "Training plan\n\n- Practice before the hunt.\n- Bring supplies for the group.\n")
	tts := openTTSSubstitutionEditor()
	palette, style := openThemeSourceEditor(false), openThemeSourceEditor(true)
	if tts == nil || palette == nil || style == nil {
		t.Fatal("configuration editor did not open")
	}
	for _, editor := range sourceEditors {
		editor.win.Open = false
	}
	scenes := []notesEditorScene{
		{"notes", personalNotes.win, 660, 500, personalNotes.refreshList},
		{"notes-narrow", personalNotes.win, 360, 500, personalNotes.refreshList},
		{"note-details", details, 0, 0, nil},
		{"note", ed.win, 780, 540, ed.layout},
		{"tts", tts.win, 780, 540, tts.layout},
		{"palette", palette.win, 780, 540, palette.layout},
		{"style", style.win, 780, 540, style.layout},
	}
	for _, scene := range scenes {
		scene.win.Open = false
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	g := &notesEditorRenderGame{dir: dir, scenes: scenes}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type notesEditorScene struct {
	name          string
	win           *eui.WindowData
	width, height float32
	layout        func()
}
type notesEditorRenderGame struct {
	dir    string
	scenes []notesEditorScene
	done   bool
	err    error
}

func (*notesEditorRenderGame) Layout(_, _ int) (int, int) { return 1920, 1080 }
func (g *notesEditorRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *notesEditorRenderGame) Draw(screen *ebiten.Image) {
	if !g.done {
		g.done = true
		g.err = g.render(screen)
	}
}
func (g *notesEditorRenderGame) render(screen *ebiten.Image) error {
	loadMaterialIcons()
	for _, scale := range []float32{1, 2} {
		eui.SetUIScale(scale)
		for _, scene := range g.scenes {
			if scene.width > 0 {
				scene.win.Size = eui.Point{X: scene.width, Y: scene.height}
			}
			if scene.layout != nil {
				scene.layout()
			}
			scene.win.MarkOpen()
			_ = scene.win.SetPos(eui.Point{X: 20, Y: 20})
			screen.Clear()
			eui.Draw(screen)
			if err := checkRenderedControlText(scene.win.Contents, scale); err != nil {
				return fmt.Errorf("%s at %gx: %w", scene.name, scale, err)
			}
			f, err := os.Create(filepath.Join(g.dir, fmt.Sprintf("%s-%gx.png", scene.name, scale)))
			if err != nil {
				return err
			}
			err = png.Encode(f, screen)
			f.Close()
			if err != nil {
				return err
			}
			scene.win.Open = false
		}
	}
	return nil
}
