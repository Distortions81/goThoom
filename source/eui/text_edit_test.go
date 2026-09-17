package eui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"gothoom/internal/inputkeys"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/goregular"
)

func editingFixture(t *testing.T, value string, multiline bool) *ItemData {
	t.Helper()
	oldFocus, oldSelection, oldTime, oldScale, oldShift := focusedItem, selectedTextItem, updateNow, uiScale, ShiftPressed
	oldSearch := activeSearch
	t.Cleanup(func() {
		focusedItem, selectedTextItem, updateNow, uiScale, ShiftPressed = oldFocus, oldSelection, oldTime, oldScale, oldShift
		activeSearch = oldSearch
	})
	if err := EnsureFontSource(goregular.TTF); err != nil {
		t.Fatal(err)
	}
	uiScale, ShiftPressed, updateNow = 1, false, time.Unix(100, 0)
	item, _ := NewInput()
	item.Text, item.Multiline = value, multiline
	item.Size = point{X: 240, Y: 90}
	item.DrawRect = rect{X0: 40, Y0: 60, X1: 280, Y1: 150}
	Focus(item)
	return item
}

func TestTextEditingSelectionReplacementAndHistory(t *testing.T) {
	item := editingFixture(t, "hello world", false)
	events := 0
	item.Handler.Handle = func(e UIEvent) {
		if e.Type == EventInputChanged {
			events++
		}
	}
	item.editSelect(11, 6)
	item.editInsert("café", "typing")
	if item.Text != "hello café" || item.CursorPos != 10 || item.SelectedText() != "" {
		t.Fatalf("replacement: %+v", item.editSnapshot())
	}
	item.editUndo(false)
	if item.Text != "hello world" || item.SelectedText() != "world" || item.CursorPos != 6 {
		t.Fatalf("undo: %+v", item.editSnapshot())
	}
	item.editUndo(true)
	if item.Text != "hello café" || events != 3 || *item.TextPtr != item.Text {
		t.Fatalf("redo/event/pointer: %q %d", item.Text, events)
	}
}

func TestTextEditingGroupsTypingAndBreaksOnMovement(t *testing.T) {
	item := editingFixture(t, "", false)
	item.editInsert("a", "typing")
	updateNow = updateNow.Add(100 * time.Millisecond)
	item.editInsert("b", "typing")
	item.editKey(ebiten.KeyArrowLeft, inputkeys.Modifiers{}, false)
	item.editInsert("X", "typing")
	item.editUndo(false)
	if item.Text != "ab" {
		t.Fatal(item.Text)
	}
	item.editUndo(false)
	if item.Text != "" {
		t.Fatal(item.Text)
	}
	item.editUndo(true)
	item.editInsert("new", "typing")
	item.editUndo(true)
	if item.Text != "anewb" {
		t.Fatalf("new edit retained stale redo: %q", item.Text)
	}
	item.Text = "external replacement"
	item.editUndo(false)
	if item.Text != "external replacement" {
		t.Fatal("undo restored a different document")
	}
}

func TestTextEditingDeletesGraphemesAndSelection(t *testing.T) {
	for _, value := range []string{"é", "e\u0301", "👩‍💻", "🇺🇸"} {
		t.Run(value, func(t *testing.T) {
			item := editingFixture(t, "A"+value+"B", false)
			item.editMove(1, false)
			item.editKey(ebiten.KeyDelete, inputkeys.Modifiers{}, false)
			if item.Text != "AB" {
				t.Fatalf("split grapheme: %q", item.Text)
			}
			item.editUndo(false)
			item.editKey(ebiten.KeyArrowRight, inputkeys.Modifiers{}, false)
			item.editKey(ebiten.KeyBackspace, inputkeys.Modifiers{}, false)
			if item.Text != "AB" {
				t.Fatalf("backspace split grapheme: %q", item.Text)
			}
			item.editSelect(2, 0)
			item.editKey(ebiten.KeyDelete, inputkeys.Modifiers{}, false)
			if item.Text != "" {
				t.Fatal("delete did not remove selection")
			}
		})
	}
}

func TestTextEditingPlatformNavigationAndSelection(t *testing.T) {
	for _, tc := range []struct {
		name string
		mods inputkeys.Modifiers
	}{
		{"Control", inputkeys.Modifiers{Control: true}}, {"Command", inputkeys.Modifiers{Mac: true, Meta: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := editingFixture(t, "first\nsecond", true)
			item.editKey(ebiten.KeyA, tc.mods, false)
			if item.SelectedText() != item.Text {
				t.Fatal("select all")
			}
			item.editKey(ebiten.KeyArrowLeft, inputkeys.Modifiers{}, false)
			if item.CursorPos != 0 || item.SelectedText() != "" {
				t.Fatal("collapse selection")
			}
			item.editKey(ebiten.KeyArrowRight, inputkeys.Modifiers{}, true)
			if item.SelectedText() != "f" {
				t.Fatal("extend selection")
			}
			item.editKey(ebiten.KeyEnd, tc.mods, true)
			if item.SelectedText() != item.Text {
				t.Fatal("extend to document end")
			}
		})
	}
	item := editingFixture(t, "one two three", false)
	item.editMove(0, false)
	item.editKey(ebiten.KeyArrowRight, inputkeys.Modifiers{Control: true}, false)
	if item.CursorPos != 4 {
		t.Fatal("Control+Right", item.CursorPos)
	}
	item.editMove(0, false)
	item.editKey(ebiten.KeyArrowRight, inputkeys.Modifiers{Mac: true, Alt: true}, false)
	if item.CursorPos != 3 {
		t.Fatal("Option+Right", item.CursorPos)
	}
	item.editKey(ebiten.KeyArrowRight, inputkeys.Modifiers{Mac: true, Meta: true}, false)
	if item.CursorPos != 13 {
		t.Fatal("Command+Right", item.CursorPos)
	}
	item.editKey(ebiten.KeyBackspace, inputkeys.Modifiers{Mac: true, Meta: true}, false)
	if item.Text != "" {
		t.Fatal("Command+Backspace", item.Text)
	}
}

func TestTextEditingSingleLineAndMultilinePolicy(t *testing.T) {
	item := editingFixture(t, "", false)
	if item.editKey(ebiten.KeyEnter, inputkeys.Modifiers{}, false) || item.editKey(ebiten.KeyTab, inputkeys.Modifiers{}, false) {
		t.Fatal("single line consumed submit or focus traversal")
	}
	item.editInsert("a\r\nb\rc\td\x00", "")
	if item.Text != "a b c d" {
		t.Fatalf("single line paste: %q", item.Text)
	}
	item.Multiline, item.AcceptTab = true, true
	item.editKey(ebiten.KeyEnter, inputkeys.Modifiers{}, false)
	item.editKey(ebiten.KeyTab, inputkeys.Modifiers{}, false)
	item.editInsert("hello\r\nworld", "")
	if item.Text != "a b c d\n\thello\nworld" {
		t.Fatalf("multiline: %q", item.Text)
	}
	if item.editKey(ebiten.KeyTab, inputkeys.Modifiers{Control: true}, false) {
		t.Fatal("Ctrl+Tab must permit leaving editor")
	}
}

func TestTextEditingIndentAndUndo(t *testing.T) {
	item := editingFixture(t, "one\ntwo\nthree", true)
	item.AcceptTab = true
	item.editSelect(0, 8)
	item.editKey(ebiten.KeyTab, inputkeys.Modifiers{}, false)
	if item.Text != "\tone\n\ttwo\nthree" {
		t.Fatalf("indent selected lines: %q", item.Text)
	}
	item.editKey(ebiten.KeyTab, inputkeys.Modifiers{}, true)
	if item.Text != "one\ntwo\nthree" {
		t.Fatalf("outdent: %q", item.Text)
	}
	item.editUndo(false)
	if item.Text != "\tone\n\ttwo\nthree" {
		t.Fatal("indent undo")
	}
}

func TestTextEditingPasswordAndDisabled(t *testing.T) {
	item := editingFixture(t, "******", false)
	secret := "secret"
	item.HideText, item.SecretText, item.TextPtr = true, secret, &secret
	item.editSelect(0, 6)
	if item.SelectedText() != "" {
		t.Fatal("password can be copied")
	}
	item.editInsert("new", "")
	if item.Text != "***" || secret != "new" || item.SecretText != "new" {
		t.Fatal("password mask/binding", item.Text, secret)
	}
	item.editUndo(false)
	if item.SecretText != "new" || len(item.editor().undo) != 0 {
		t.Fatal("password retained history")
	}
	item.Disabled = true
	if item.editKey(ebiten.KeyBackspace, inputkeys.Modifiers{}, false) || itemAcceptsTextEditing(item) {
		t.Fatal("disabled field accepted input")
	}
}

func TestTextEditingClickLayoutWithPaddingScaleTabsAndScroll(t *testing.T) {
	item := editingFixture(t, "\talpha\nsecond\nthird", true)
	for _, scale := range []float32{1, 1.5, 2} {
		uiScale = scale
		item.textDrawOrigin, item.textDrawSize = point{X: 40, Y: 60}, point{X: 240, Y: 90}
		item.editor().scroll = point{X: 7, Y: 4}
		layout := item.editLayout()
		pad := (item.BorderPad + item.Padding + currentStyle.TextPadding) * scale
		// Click after the tab and 'a' using the engine's actual shaped advance.
		x := float32(text.AdvanceAt("    alpha", 5, layout.face))
		pos := item.editCursorAt(point{X: 40 + pad + x - 7, Y: 60 + pad + layout.lineHeight/2 - 4})
		if pos != 2 {
			t.Fatalf("scale %v: tab hit %d", scale, pos)
		}
		x = float32(text.AdvanceAt("second", 3, layout.face))
		pos = item.editCursorAt(point{X: 40 + pad + x - 7, Y: 60 + pad + layout.lineHeight*1.5 - 4})
		if pos != 10 {
			t.Fatalf("scale %v: second line hit %d", scale, pos)
		}
	}
}

func TestTextEditingVerticalMovementPreservesColumn(t *testing.T) {
	item := editingFixture(t, "abcdefgh\nx\nabcdefgh", true)
	item.editMove(6, false)
	item.editKey(ebiten.KeyArrowDown, inputkeys.Modifiers{}, false)
	if item.CursorPos != 10 {
		t.Fatal("short line", item.CursorPos)
	}
	item.editKey(ebiten.KeyArrowDown, inputkeys.Modifiers{}, false)
	if item.CursorPos != 17 {
		t.Fatal("lost preferred column", item.CursorPos)
	}
}

func TestTextEditingCaretFollowsAndBlinks(t *testing.T) {
	item := editingFixture(t, strings.Repeat("long line here\n", 30)+strings.Repeat("W", 90), true)
	viewport, _ := item.editGeometry()
	item.followEditCaret(viewport)
	s := item.editor()
	if s.scroll.X <= 0 || s.scroll.Y <= 0 {
		t.Fatal("caret did not scroll into view", s.scroll)
	}
	item.editMove(0, false)
	item.followEditCaret(viewport)
	if s.scroll != (point{}) {
		t.Fatal("home did not scroll to start", s.scroll)
	}
	item.Dirty = false
	item.updateCaretBlink(updateNow.Add(600 * time.Millisecond))
	if s.caretOn || !item.Dirty {
		t.Fatal("blink did not invalidate rendering")
	}
	item.editInsert("x", "typing")
	if !s.caretOn {
		t.Fatal("typing did not reveal caret")
	}
}

func TestTextEditingClickShiftAndWordSelection(t *testing.T) {
	item := editingFixture(t, "hello world", false)
	_, origin := item.editGeometry()
	face := item.editLayout().face
	at := func(pos int) point {
		return point{X: origin.X + float32(text.AdvanceAt(item.Text, pos, face)), Y: origin.Y + 2}
	}
	item.clickEditableText(at(2), false)
	if item.CursorPos != 2 {
		t.Fatal(item.CursorPos)
	}
	item.clickEditableText(at(8), true)
	if item.SelectedText() != "llo wo" {
		t.Fatal(item.SelectedText())
	}
	updateNow = updateNow.Add(time.Second)
	item.clickEditableText(at(7), false)
	updateNow = updateNow.Add(100 * time.Millisecond)
	item.clickEditableText(at(7), false)
	if item.SelectedText() != "world" {
		t.Fatal("double click", item.SelectedText())
	}
}

func TestTextEditingCutIsAtomic(t *testing.T) {
	item := editingFixture(t, "keep this text", false)
	item.editSelect(5, 9)
	item.editCut(func(string) error { return fmt.Errorf("clipboard unavailable") })
	if item.Text != "keep this text" || item.SelectedText() != "this" {
		t.Fatal("failed clipboard write deleted text")
	}
	copied := ""
	item.editCut(func(value string) error { copied = value; return nil })
	if copied != "this" || item.Text != "keep  text" {
		t.Fatal("cut", copied, item.Text)
	}
	item.editUndo(false)
	if item.Text != "keep this text" || item.SelectedText() != "this" {
		t.Fatal("cut undo")
	}
}

func TestTextEditingWordSelectionSurvivesHeldMouse(t *testing.T) {
	item := editingFixture(t, "hello world", false)
	_, origin := item.editGeometry()
	face := item.editLayout().face
	p := point{X: origin.X + float32(text.AdvanceAt(item.Text, 7, face)), Y: origin.Y + 2}
	item.clickEditableText(p, false)
	updateNow = updateNow.Add(100 * time.Millisecond)
	item.clickEditableText(p, false)
	item.dragEditableText(p)
	if item.SelectedText() != "world" {
		t.Fatal("held double click lost word selection", item.SelectedText())
	}
}

func TestTextEditingExternalOwnerKeepsKeyboardControl(t *testing.T) {
	item := editingFixture(t, "app-owned", true)
	item.ExternalTextEditing = true
	for _, key := range []ebiten.Key{ebiten.KeyDelete, ebiten.KeyEnter, ebiten.KeyZ, ebiten.KeyA} {
		if item.editKey(key, inputkeys.Modifiers{Control: true}, false) {
			t.Fatal("EUI consumed externally owned keyboard input")
		}
	}
	if item.Text != "app-owned" {
		t.Fatal(item.Text)
	}
}

func TestTextEditingSmallFieldKeepsFullLine(t *testing.T) {
	item := editingFixture(t, "Normal text", false)
	for _, scale := range []float32{1, 1.5, 2} {
		uiScale = scale
		item.Padding = 12
		size := point{X: 240 * scale, Y: 24 * scale}
		viewport := item.editViewport(point{}, size)
		if viewport.Y1-viewport.Y0 < min(size.Y, item.editLayout().lineHeight) {
			t.Fatalf("scale %v clipped text with theme padding", scale)
		}
	}
}

func TestTextEditingExternalInputUsesDrawnCaretPosition(t *testing.T) {
	item := editingFixture(t, "hello world", false)
	item.ExternalTextEditing = true
	_, origin := item.editGeometry()
	x := float32(text.AdvanceAt(item.Text, 6, item.editLayout().face))
	if got := item.cursorIndexAt(point{X: origin.X + x, Y: origin.Y + 2}); got != 6 {
		t.Fatalf("external input hit %d, want 6", got)
	}
}

func TestFocusedTextInputRejectsUnavailableControls(t *testing.T) {
	item := editingFixture(t, "draft", true)
	if FocusedTextInput() != item {
		t.Fatal("focused editor not reported")
	}
	item.Disabled = true
	if FocusedTextInput() != nil {
		t.Fatal("disabled input captured keyboard")
	}
	item.Disabled = false
	item.Invisible = true
	if FocusedTextInput() != nil {
		t.Fatal("hidden input captured keyboard")
	}
	item.Invisible = false
	win := NewWindow()
	win.AddItem(item)
	if FocusedTextInput() != nil {
		t.Fatal("closed window captured keyboard")
	}
}
