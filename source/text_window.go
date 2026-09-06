package main

import (
	"math"
	"strings"

	"gothoom/eui"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/pkg/browser"
)

var (
	cursorPosition  = ebiten.CursorPosition
	showContextMenu = eui.ShowContextMenu

	chatTextWrapCache      textWindowWrapCache
	consoleTextWrapCache   textWindowWrapCache
	aboutTextWrapCache     textWindowWrapCache
	changelogTextWrapCache textWindowWrapCache
)

type textWindowWrapCache = eui.TextWindowWrapCache

func alternateRowColor() eui.Color { return eui.SubtleAlternateRowColor() }

func alternatingTextRows(win *eui.WindowData) bool {
	if win == nil {
		return false
	}
	if win == chatWin {
		return gs.ChatAlternatingRowColors
	}
	if win == consoleWin {
		return gs.ConsoleAlternatingRowColors
	}
	return false
}

func restoreAlternateTextRow(row *eui.ItemData, index int, enabled bool) {
	row.Filled = enabled && index%2 == 1
	if row.Filled {
		row.Color = alternateRowColor()
	} else {
		row.Color = eui.Color{}
	}
}

// newTextWindow wraps eui.NewTextWindow and assigns a resize handler that
// invokes the provided update callback.
func newTextWindow(name string, hz eui.HZone, vz eui.VZone, hasInput bool, update func()) (*eui.WindowData, *eui.ItemData, *eui.ItemData) {
	win, list, input := eui.NewTextWindow(name, hz, vz, hasInput)
	if update != nil {
		win.OnResize = func() {
			update()
		}
	}
	return win, list, input
}

// updateTextWindow refreshes a text window's content and optional input message.
// If faceSrc is nil the default font source is used.
func updateTextWindow(win *eui.WindowData, list, input *eui.ItemData, msgs []string, fontSize float64, inputMsg string, faceSrc *text.GoTextFaceSource, alternateRows bool, wrapCache *textWindowWrapCache) {
	updateTextWindowFrom(win, list, input, msgs, fontSize, inputMsg, faceSrc, alternateRows, wrapCache, 0)
}

// updateTextWindowFrom supplies client input and spellchecking policy to EUI.
func updateTextWindowFrom(win *eui.WindowData, list, input *eui.ItemData, msgs []string, fontSize float64, inputMsg string, faceSrc *text.GoTextFaceSource, alternateRows bool, wrapCache *textWindowWrapCache, firstChanged int) {
	eui.UpdateTextWindow(win, list, input, msgs, eui.TextWindowOptions{
		FontSize: fontSize, FontSource: faceSrc, AlternateRows: alternateRows, FirstChanged: firstChanged,
		OnURLClick: func(url string) { _ = browser.OpenURL(url) },
		InputText:  inputMsg, InputEditable: inputActive,
		InputUnderlines: func(wrapped string) []eui.TextSpan {
			if inputMsg == "" || strings.HasPrefix(inputMsg, "[") {
				return nil
			}
			if len(input.Contents) > 0 && input.Contents[0].Text == wrapped && !spellDirty {
				return input.Contents[0].Underlines
			}
			if spellDirty {
				spellDirty = false
				return findMisspellings(wrapped)
			}
			return nil
		},
	}, wrapCache)
	if input != nil && len(input.Contents) > 0 {
		t := input.Contents[0]
		if t.Text != "" && t.Focused {
			showSpellSuggestions(t)
		}
	}
}

// showSpellSuggestions displays correction suggestions for misspelled words
// when hovering over underlined text. Selecting a suggestion replaces the
// word and updates the input text.
func showSpellSuggestions(t *eui.ItemData) {
	if t == nil || len(t.Underlines) == 0 || sc == nil {
		return
	}
	if t.Text == "" || t.ParentWindow == nil || !t.ParentWindow.IsOpen() {
		return
	}
	if eui.ContextMenusOpen() {
		return
	}
	mx, my := cursorPosition()
	x := float32(mx)
	y := float32(my)
	if x < t.DrawRect.X0 || x > t.DrawRect.X1 || y < t.DrawRect.Y0 || y > t.DrawRect.Y1 {
		return
	}
	if t.Face == nil {
		return
	}
	rs := []rune(t.Text)
	metrics := t.Face.Metrics()
	lineHeight := float32(math.Ceil(metrics.HAscent + metrics.HDescent + 2))
	for _, ul := range t.Underlines {
		if ul.Start < 0 || ul.End > len(rs) || ul.Start >= ul.End {
			continue
		}
		line := 0
		lineStart := 0
		for i := 0; i < ul.Start; i++ {
			if rs[i] == '\n' {
				line++
				lineStart = i + 1
			}
		}
		prefix := string(rs[lineStart:ul.Start])
		x0, _ := text.Measure(prefix, t.Face, 0)
		word := string(rs[ul.Start:ul.End])
		w, _ := text.Measure(word, t.Face, 0)
		top := t.DrawRect.Y0 + float32(line)*lineHeight
		bottom := top + lineHeight
		left := t.DrawRect.X0 + float32(x0)
		right := left + float32(w)
		if x >= left && x <= right && y >= top && y <= bottom {
			sugg := suggestCorrections(strings.ToLower(word), 5)
			if len(sugg) == 0 {
				return
			}
			showContextMenu(sugg, x, y, func(i int) {
				if i < 0 || i >= len(sugg) {
					return
				}
				replacement := sugg[i]
				rs := []rune(t.Text)
				newWrapped := string(rs[:ul.Start]) + replacement + string(rs[ul.End:])
				plain := strings.ReplaceAll(newWrapped, "\n", "")
				scriptSetInputText(plain)
				t.Text = newWrapped
				t.Underlines = findMisspellings(newWrapped)
				if t.ParentWindow != nil {
					t.ParentWindow.Refresh()
				}
				eui.CloseContextMenus()
			})
			return
		}
	}
}

// searchTextWindow highlights rows in the list containing the query string and
// adds markers to the scrollbar for quick navigation. It clears highlights when
// the query is empty.
func searchTextWindow(win *eui.WindowData, list *eui.ItemData, query string) {
	applyTextWindowSearch(list, query)
	if win != nil {
		win.Refresh()
	}
}

// applyTextWindowSearch updates search presentation without refreshing the
// window. Update callers use it to batch layout, style, and search into one
// final refresh.
func applyTextWindowSearch(list *eui.ItemData, query string) {
	if list == nil {
		return
	}

	q := strings.ToLower(query)
	total := len(list.Contents)
	var marks []float32
	accent := eui.AccentColor()
	for i, it := range list.Contents {
		it.Focused = false
		if q != "" && strings.Contains(strings.ToLower(it.Text), q) {
			it.Filled = true
			it.Color = accent
			marks = append(marks, float32(i)/float32(total))
		} else {
			restoreAlternateTextRow(it, i, alternatingTextRows(list.ParentWindow))
		}
	}
	list.ScrollMarks = marks
}
