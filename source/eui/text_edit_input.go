package eui

import (
	"context"
	"math"
	"time"

	"gothoom/internal/inputkeys"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	clipboard "golang.design/x/clipboard"
)

func textKeyRepeats(key ebiten.Key) bool {
	d := inpututil.KeyPressDuration(key)
	return d == 1 || d > 24 && (d-24)%3 == 0
}

func (item *itemData) updateTextEditing(chars []rune, mods inputkeys.Modifiers, shift bool) bool {
	item.editor()
	handled := false
	if mods.Shortcut() {
		for _, key := range []ebiten.Key{ebiten.KeyA, ebiten.KeyZ, ebiten.KeyY} {
			if inpututil.IsKeyJustPressed(key) {
				handled = item.editKey(key, mods, shift) || handled
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyX) {
			item.editCut(func(value string) error {
				_, err := clipboard.Write(context.Background(), clipboard.FmtText, []byte(value))
				return err
			})
			handled = true
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyV) {
			if value, err := clipboard.Read(context.Background(), clipboard.FmtText); err == nil {
				item.editInsert(string(value), "")
			}
			handled = true
		}
	}
	if len(chars) > 0 {
		item.editInsert(string(chars), "typing")
		handled = true
	}
	for _, key := range []ebiten.Key{ebiten.KeyArrowLeft, ebiten.KeyArrowRight, ebiten.KeyArrowUp, ebiten.KeyArrowDown, ebiten.KeyHome, ebiten.KeyEnd, ebiten.KeyPageUp, ebiten.KeyPageDown, ebiten.KeyBackspace, ebiten.KeyDelete, ebiten.KeyEnter, ebiten.KeyKPEnter, ebiten.KeyTab} {
		if textKeyRepeats(key) {
			handled = item.editKey(key, mods, shift) || handled
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		ClearFocus(item)
		handled = true
	}
	return handled
}

func (item *itemData) updateCaretBlink(now time.Time) {
	s := item.editor()
	on := s.caretReset.IsZero() || now.Sub(s.caretReset)%(time.Second) < 500*time.Millisecond
	if on != s.caretOn {
		s.caretOn = on
		item.markDirty()
	}
}

func (item *itemData) clickEditableText(mpos point, extend bool) {
	activeSearch = nil
	state := item.editor()
	pos := item.editCursorAt(mpos)
	state.group, state.hasPreferredX = "", false
	if extend {
		item.editMove(pos, true)
		state.clickCount = 0
	} else {
		if !state.lastClick.IsZero() && updateNow.Sub(state.lastClick) < 400*time.Millisecond && math.Hypot(float64(mpos.X-state.lastClickPos.X), float64(mpos.Y-state.lastClickPos.Y)) < float64(5*uiScale) {
			state.clickCount = state.clickCount%3 + 1
		} else {
			state.clickCount = 1
		}
		switch state.clickCount {
		case 2:
			r := []rune(item.editText())
			if pos == len(r) && pos > 0 {
				pos--
			}
			a, b := pos, pos
			if pos < len(r) {
				class := editWordClass(r[pos])
				for a > 0 && editWordClass(r[a-1]) == class && r[a-1] != '\n' {
					a--
				}
				for b < len(r) && editWordClass(r[b]) == class && r[b] != '\n' {
					b++
				}
			}
			item.editSelect(a, b)
		case 3:
			a, b := editLineBounds(item.editText(), pos)
			if b < len([]rune(item.editText())) {
				b++
			}
			item.editSelect(a, b)
		default:
			item.editMove(pos, false)
		}
	}
	state.lastClick, state.lastClickPos = updateNow, mpos
	state.dragStart, state.dragEnd = item.SelectStart, item.SelectEnd
	item.selecting = true
	item.Focused = true
	focusedItem = item
}

func (item *itemData) dragEditableText(mpos point) {
	viewport, _ := item.editGeometry()
	viewport = intersectRect(viewport, item.DrawRect)
	state := item.editor()
	// Scrolling uses a bounded per-tick step, so dragging far outside does not
	// skip uncontrollably through a document.
	step := item.editLayout().lineHeight / 3
	if mpos.X < viewport.X0 {
		state.scroll.X -= step
	}
	if mpos.X > viewport.X1 {
		state.scroll.X += step
	}
	if mpos.Y < viewport.Y0 {
		state.scroll.Y -= step
	}
	if mpos.Y > viewport.Y1 {
		state.scroll.Y += step
	}
	item.clampEditScroll(viewport)
	pos := item.editCursorAt(mpos)
	anchor := state.dragStart
	if state.clickCount > 1 {
		a, b := editLineBounds(item.editText(), pos)
		if state.clickCount == 2 {
			r := []rune(item.editText())
			a, b = pos, pos
			if pos < len(r) {
				class := editWordClass(r[pos])
				for a > 0 && editWordClass(r[a-1]) == class && r[a-1] != '\n' {
					a--
				}
				for b < len(r) && editWordClass(r[b]) == class && r[b] != '\n' {
					b++
				}
			}
		} else if b < len([]rune(item.editText())) {
			b++
		}
		if pos < state.dragStart {
			anchor, pos = state.dragEnd, a
		} else {
			pos = max(b, state.dragEnd)
		}
	}
	if item.SelectStart != anchor || item.SelectEnd != pos || state.scroll != (point{}) {
		item.editSelect(anchor, pos)
	}
	state.followCaret = false
}

// A failed clipboard write must leave the document intact.
func (item *itemData) editCut(write func(string) error) {
	if item.HideText {
		return
	}
	if value := item.SelectedText(); value != "" {
		if err := write(value); err == nil {
			a, b := item.editSelection()
			item.editReplace(a, b, "", "")
		}
	}
}
