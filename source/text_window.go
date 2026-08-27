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
)

func alternateRowColor() eui.Color { return eui.SubtleAlternateRowColor() }

func restoreAlternateTextRow(row *eui.ItemData, index int) {
	row.Filled = gs.AlternateRowBackgrounds && index%2 == 1
	if row.Filled {
		row.Color = alternateRowColor()
	} else {
		row.Color = eui.Color{}
	}
}

// makeTextWindow creates a standardized text window with optional input bar.
func makeTextWindow(title string, hz eui.HZone, vz eui.VZone, withInput bool) (*eui.WindowData, *eui.ItemData, *eui.ItemData) {
	win := eui.NewWindow()
	win.Size = eui.Point{X: 410, Y: 450}
	win.Title = title
	win.Closable = true
	win.Resizable = true
	win.Movable = true
	win.SetZone(hz, vz)
	// Only the inner list should scroll; disable window scrollbars to avoid overlap
	win.NoScroll = true

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	win.AddItem(flow)

	list := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true, Fixed: true}
	flow.AddItem(list)

	var input *eui.ItemData
	if withInput {
		input = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true, Scrollable: true}
		flow.AddItem(input)
	}

	win.AddWindow(false)
	return win, list, input
}

// sizeTextWindowList fits a scrolling list below any fixed sibling rows, such
// as the docked toolbar. Those rows occupy part of the window's client area and
// must not be included in the list's viewport height.
func sizeTextWindowList(list *eui.ItemData, clientW, clientH float32) {
	if list == nil {
		return
	}
	scale := eui.UIScale()
	if scale <= 0 {
		scale = 1
	}
	clientW /= scale
	clientH /= scale
	extraH := float32(0)
	if list.Parent != nil {
		list.Parent.Size.X = clientW
		list.Parent.Size.Y = clientH
		for _, item := range list.Parent.Contents {
			if item == list {
				continue
			}
			item.Size.X = clientW
			extraH += item.Size.Y
		}
	}
	list.Size.X = clientW
	list.Size.Y = max(0, clientH-extraH)
}

// newTextWindow wraps makeTextWindow and assigns a resize handler that
// invokes the provided update callback.
func newTextWindow(name string, hz eui.HZone, vz eui.VZone, hasInput bool, update func()) (*eui.WindowData, *eui.ItemData, *eui.ItemData) {
	win, list, input := makeTextWindow(name, hz, vz, hasInput)
	if update != nil {
		win.OnResize = func() {
			update()
			if win != nil {
				win.Refresh()
			}
		}
	}
	return win, list, input
}

// updateTextWindow refreshes a text window's content and optional input message.
// If faceSrc is nil the default font source is used.
func updateTextWindow(win *eui.WindowData, list, input *eui.ItemData, msgs []string, fontSize float64, inputMsg string, faceSrc *text.GoTextFaceSource, alternateRows bool) {
	if list == nil || win == nil {
		return
	}

	// Compute client area (window size minus title bar and padding).
	clientW := win.GetSize().X
	clientH := win.GetSize().Y - win.GetTitleSize()
	// Adjust for window padding/border so child flows fit within clip region.
	s := eui.UIScale()
	if win.NoScale {
		s = 1
	}
	pad := (win.Padding + win.BorderPad) * s
	clientWAvail := clientW - 2*pad
	if clientWAvail < 0 {
		clientWAvail = 0
	}
	clientHAvail := clientH - 2*pad
	if clientHAvail < 0 {
		clientHAvail = 0
	}

	// Compute a row height from actual font metrics (ascent+descent) to
	// avoid clipping at large sizes. Convert pixels to item units.
	ui := eui.UIScale()
	if ui <= 0 {
		ui = 1
	}
	// Match the render-time face size used by EUI text items
	// (EUI renders with size = fontSize*ui + 2). Using the same value here
	// ensures wrap measurements align with what actually gets drawn.
	facePx := float64(float32(fontSize)*ui) + 2
	var goFace *text.GoTextFace
	if faceSrc != nil {
		goFace = &text.GoTextFace{Source: faceSrc, Size: facePx}
	} else if src := eui.FontSource(); src != nil {
		goFace = &text.GoTextFace{Source: src, Size: facePx}
	} else {
		goFace = &text.GoTextFace{Size: facePx}
	}
	metrics := goFace.Metrics()
	linePx := math.Ceil(metrics.HAscent + metrics.HDescent + 2) // +2 px padding
	rowUnits := float32(linePx) / ui

	// Prepare wrapping parameters: use the same face for measurement.
	var face text.Face = goFace
	// Reserve a gutter for the vertical scrollbar so wrapped text and
	// hitboxes never encroach beneath it. This applies broadly to chat,
	// console, help, and about windows using this helper.
	sb := eui.ScrollbarWidth()
	contentW := clientWAvail - sb
	if contentW < 0 {
		contentW = 0
	}
	// Use the effective content width in pixels for wrapping.
	wrapWidthPx := float64(contentW - 3*pad)
	contentWUnits := contentW / ui
	clientWUnits := clientWAvail / ui
	clientHUnits := clientHAvail / ui

	for i, msg := range msgs {
		// Word-wrap the message to the available width.
		_, lines := wrapText(msg, face, wrapWidthPx)
		wrapped := strings.Join(lines, "\n")
		linesN := len(lines)
		if linesN < 1 {
			linesN = 1
		}
		if i < len(list.Contents) {
			if list.Contents[i].Text != wrapped || list.Contents[i].FontSize != float32(fontSize) {
				list.Contents[i].Text = wrapped
				list.Contents[i].FontSize = float32(fontSize)
			}
			list.Contents[i].Face = face
			list.Contents[i].Size.Y = rowUnits * float32(linesN)
			list.Contents[i].Size.X = contentWUnits
			list.Contents[i].SelectableText = true
			list.Contents[i].Filled = alternateRows && gs.AlternateRowBackgrounds && i%2 == 1
			list.Contents[i].Color = alternateRowColor()
			list.Contents[i].OnURLClick = func(url string) { _ = browser.OpenURL(url) }
		} else {
			t, _ := eui.NewText()
			t.Text = wrapped
			t.FontSize = float32(fontSize)
			t.Face = face
			t.Size = eui.Point{X: contentWUnits, Y: rowUnits * float32(linesN)}
			t.SelectableText = true
			t.Filled = alternateRows && gs.AlternateRowBackgrounds && i%2 == 1
			t.Color = alternateRowColor()
			t.OnURLClick = func(url string) { _ = browser.OpenURL(url) }
			// Append to maintain ordering with the msgs index
			list.AddItem(t)
		}
	}
	if len(list.Contents) > len(msgs) {
		for i := len(msgs); i < len(list.Contents); i++ {
			list.Contents[i] = nil
		}
		list.Contents = list.Contents[:len(msgs)]
	}

	var scrollInput bool
	if input != nil {
		scrollInput = input.ScrollAtBottom()
		// Soft-wrap the input message to the available width and grow the input area.
		_, inLines := wrapText(inputMsg, face, wrapWidthPx)
		wrappedIn := strings.Join(inLines, "\n")
		var miss []eui.TextSpan
		if inputMsg != "" && !strings.HasPrefix(inputMsg, "[") {
			if len(input.Contents) > 0 && input.Contents[0].Text == wrappedIn && !spellDirty {
				miss = input.Contents[0].Underlines
			} else if spellDirty {
				miss = findMisspellings(wrappedIn)
				spellDirty = false
			}
		}
		inLinesN := len(inLines)
		if inLinesN < 1 {
			inLinesN = 1
		}
		inputContentH := rowUnits * float32(inLinesN)
		maxInputH := clientHUnits / 2
		if inputContentH > maxInputH {
			input.Size.Y = maxInputH
			input.Scrollable = true
		} else {
			input.Size.Y = inputContentH
			input.Scrollable = false
		}
		input.Size.X = contentWUnits
		if len(input.Contents) == 0 {
			t, _ := eui.NewText()
			t.Text = wrappedIn
			t.FontSize = float32(fontSize)
			t.Face = face
			t.Size = eui.Point{X: contentWUnits, Y: inputContentH}
			t.Filled = true
			t.SelectableText = inputActive
			t.Underlines = miss
			input.AddItem(t)
		} else {
			if input.Contents[0].Text != wrappedIn || input.Contents[0].FontSize != float32(fontSize) {
				input.Contents[0].Text = wrappedIn
				input.Contents[0].FontSize = float32(fontSize)
			}
			input.Contents[0].Face = face
			input.Contents[0].Size.X = contentWUnits
			input.Contents[0].Size.Y = inputContentH
			input.Contents[0].SelectableText = inputActive
			input.Contents[0].Underlines = miss
		}
		if scrollInput {
			input.Scroll.Y = 1e9
		}
	}

	if input != nil && len(input.Contents) > 0 {
		t := input.Contents[0]
		if t.Text != "" && t.Focused {
			showSpellSuggestions(t)
		}
	}

	// Size the flow to the client area, and the list to fill above any bottom items and optional input.
	var extraH float32
	if list.Parent != nil {
		list.Parent.Size.X = clientWUnits
		list.Parent.Size.Y = clientHUnits
		for _, c := range list.Parent.Contents {
			if c != list && c != input {
				c.Size.X = clientWUnits
				extraH += c.Size.Y
			}
		}
	}
	list.Size.X = clientWUnits
	if input != nil {
		list.Size.Y = max(0, clientHUnits-input.Size.Y-extraH)
	} else {
		list.Size.Y = max(0, clientHUnits-extraH)
	}
	// Do not refresh here unconditionally; callers decide when to refresh.
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
	if list == nil {
		if win != nil {
			win.Refresh()
		}
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
			restoreAlternateTextRow(it, i)
		}
	}
	list.ScrollMarks = marks
	if win != nil {
		win.Refresh()
	}
}
