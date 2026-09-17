package eui

import (
	"strings"
	"testing"

	"gothoom/internal/inputkeys"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestTextZoomKeysPreserveDraftAndSelection(t *testing.T) {
	item := editingFixture(t, "hello world", true)
	item.editSelect(2, 8)
	before := item.editSnapshot()
	steps := 0
	item.OnTextZoom = func(delta int) { steps += delta }
	for _, mods := range []inputkeys.Modifiers{{Control: true}, {Mac: true, Meta: true}} {
		for _, key := range []ebiten.Key{ebiten.KeyEqual, ebiten.KeyKPAdd} {
			if !item.editKey(key, mods, true) {
				t.Fatal("zoom-in shortcut was not handled")
			}
		}
		for _, key := range []ebiten.Key{ebiten.KeyMinus, ebiten.KeyKPSubtract} {
			if !item.editKey(key, mods, false) {
				t.Fatal("zoom-out shortcut was not handled")
			}
		}
	}
	if steps != 0 || item.editSnapshot() != before || item.CanUndo() {
		t.Fatal("zoom changed the draft, selection, or undo history")
	}
	for _, mods := range []inputkeys.Modifiers{{}, {Control: true, Alt: true}} {
		if item.editKey(ebiten.KeyEqual, mods, false) {
			t.Fatal("plain/AltGr key was consumed as zoom")
		}
	}
	item.OnTextZoom = nil
	if item.editKey(ebiten.KeyEqual, inputkeys.Modifiers{Control: true}, false) {
		t.Fatal("ordinary input consumed zoom without a handler")
	}
}

func TestTextZoomWheelConsumesOnlyEditorShortcutScroll(t *testing.T) {
	item := editingFixture(t, strings.Repeat("line\n", 100), true)
	oldCapture := keyboardInputCaptured
	t.Cleanup(func() { keyboardInputCaptured = oldCapture })
	keyboardInputCaptured = false
	steps := 0
	item.OnTextZoom = func(delta int) { steps += delta }
	items, pos := []*itemData{item}, point{X: 80, Y: 100}
	ctrl := inputkeys.Modifiers{Control: true}
	for _, delta := range []float32{0.5, 0.5, -2} {
		if !scrollEditable(items, pos, point{Y: delta}, ctrl) {
			t.Fatal("zoom wheel was not consumed")
		}
	}
	if steps != -1 || item.editor().scroll != (point{}) {
		t.Fatalf("zoom also scrolled or lost fractional steps: %d, %+v", steps, item.editor().scroll)
	}
	if scrollEditable(items, point{}, point{Y: 1}, ctrl) || steps != -1 {
		t.Fatal("wheel outside the editor zoomed it")
	}
	if !scrollEditable(items, pos, point{Y: -1}, inputkeys.Modifiers{}) || item.editor().scroll.Y <= 0 || steps != -1 {
		t.Fatal("normal scrolling changed")
	}
	keyboardInputCaptured = true
	before := item.editor().scroll.Y
	scrollEditable(items, pos, point{Y: -1}, ctrl)
	if steps != -1 || item.editor().scroll.Y <= before {
		t.Fatal("captured modifier caused zoom")
	}
	item.Disabled = true
	keyboardInputCaptured = false
	if scrollEditable(items, pos, point{Y: 1}, ctrl) || steps != -1 {
		t.Fatal("disabled editor handled wheel")
	}
}
