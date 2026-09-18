package eui

import (
	"testing"

	"gothoom/internal/inputkeys"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestTextCompletionTabAndUndo(t *testing.T) {
	item := editingFixture(t, "Blue, Élo", false)
	item.CompleteText = func(value string) string {
		if value == "Blue, Élo" {
			return "die"
		}
		return ""
	}
	changed := 0
	item.Handler.Handle = func(event UIEvent) {
		if event.Type == EventInputChanged {
			changed++
		}
	}
	if item.textCompletion() != "die" || item.Text != "Blue, Élo" {
		t.Fatal("prediction mutated the draft or ignored the Unicode cursor")
	}
	if !item.editKey(ebiten.KeyTab, inputkeys.Modifiers{}, false) || item.Text != "Blue, Élodie" || changed != 1 {
		t.Fatal("Tab did not accept the suggestion as a normal input edit")
	}
	if item.textCompletion() != "" || item.editKey(ebiten.KeyTab, inputkeys.Modifiers{}, false) {
		t.Fatal("Tab without a completion must remain available for focus navigation")
	}
	item.editUndo(false)
	if item.Text != "Blue, Élo" || item.CursorPos != len([]rune(item.Text)) {
		t.Fatal("completion did not undo as one edit")
	}
	item.editUndo(true)
	if item.Text != "Blue, Élodie" {
		t.Fatal("completion did not redo")
	}
}

func TestTextCompletionOnlyAtUnselectedSingleLineEnd(t *testing.T) {
	item := editingFixture(t, "Blu", false)
	item.CompleteText = func(string) string { return "e" }
	for _, mods := range []inputkeys.Modifiers{{Control: true}, {Meta: true}, {Alt: true}} {
		if item.editKey(ebiten.KeyTab, mods, false) || item.Text != "Blu" {
			t.Fatal("modified Tab accepted a completion")
		}
	}
	if item.editKey(ebiten.KeyTab, inputkeys.Modifiers{}, true) || item.Text != "Blu" {
		t.Fatal("Shift+Tab accepted a completion")
	}
	for _, selection := range [][2]int{{1, 1}, {0, 3}} {
		item.editSelect(selection[0], selection[1])
		if item.textCompletion() != "" || item.editKey(ebiten.KeyTab, inputkeys.Modifiers{}, false) {
			t.Fatal("completion replaced a selection or inserted in the middle")
		}
	}
	item.editSelect(3, 3)
	item.HideText = true
	item.CompleteText = func(string) string { t.Fatal("password reached completion provider"); return "" }
	if item.textCompletion() != "" {
		t.Fatal("password offered a completion")
	}
	item.HideText = false
	item.Multiline, item.AcceptTab = true, true
	if !item.editKey(ebiten.KeyTab, inputkeys.Modifiers{}, false) || item.Text != "Blu\t" {
		t.Fatal("completion interfered with multiline indentation")
	}
}
