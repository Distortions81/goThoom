package eui

import (
	"slices"
	"strings"
	"time"
	"unicode"

	"gothoom/internal/inputkeys"

	"github.com/go-text/typesetting/segmenter"
	"github.com/hajimehoshi/ebiten/v2"
)

type textEditSnapshot struct {
	text                string
	cursor, anchor, end int
}

type textEditState struct {
	known              string
	initialized        bool
	undo, redo         []textEditSnapshot
	group              string
	changed            time.Time
	caretReset         time.Time
	caretOn            bool
	scroll             point
	followCaret        bool
	preferredX         float32
	hasPreferredX      bool
	lastClick          time.Time
	lastClickPos       point
	clickCount         int
	dragStart, dragEnd int
	scrollDrag         dragType
	scrollGrab         float32
	layout             *editTextLayout
}

func (item *itemData) editText() string {
	if item.HideText {
		return item.SecretText
	}
	return item.Text
}

func (item *itemData) editor() *textEditState {
	if item.textEdit == nil {
		item.textEdit = &textEditState{}
	}
	s := item.textEdit
	value := item.editText()
	if !s.initialized || s.known != value {
		// Direct application assignments start a new document/history.
		*s = textEditState{initialized: true, known: value, followCaret: true, caretOn: true}
	}
	n := len([]rune(value))
	item.CursorPos = max(0, min(item.CursorPos, n))
	item.SelectStart = max(0, min(item.SelectStart, n))
	item.SelectEnd = max(0, min(item.SelectEnd, n))
	return s
}

func (item *itemData) editSnapshot() textEditSnapshot {
	return textEditSnapshot{item.editText(), item.CursorPos, item.SelectStart, item.SelectEnd}
}

func (item *itemData) editSelection() (int, int) {
	item.editor()
	return min(item.SelectStart, item.SelectEnd), max(item.SelectStart, item.SelectEnd)
}

func (item *itemData) resetCaret() {
	s := item.editor()
	s.caretReset, s.caretOn, s.followCaret = updateNow, true, true
	if s.caretReset.IsZero() {
		s.caretReset = time.Now()
	}
	item.markDirty()
}

func (item *itemData) editSelect(anchor, end int) {
	if selectedTextItem != nil && selectedTextItem != item {
		clearTextSelection()
	}
	n := len([]rune(item.editText()))
	item.SelectStart = max(0, min(anchor, n))
	item.SelectEnd = max(0, min(end, n))
	item.CursorPos = item.SelectEnd
	selectedTextItem = item
	item.resetCaret()
}

func (item *itemData) editMove(pos int, extend bool) {
	s := item.editor()
	s.group = ""
	anchor := pos
	if extend {
		anchor = item.SelectStart
		if item.SelectStart == item.SelectEnd {
			anchor = item.CursorPos
		}
	}
	item.editSelect(anchor, pos)
}

// Bound retained snapshots by count and bytes. Passwords never retain history.
func trimTextHistory(history []textEditSnapshot) []textEditSnapshot {
	bytes, start := 0, len(history)
	for start > 0 && len(history)-start < 100 {
		cost := len(history[start-1].text)
		if bytes+cost > 2<<20 {
			break
		}
		bytes += cost
		start--
	}
	if start > 0 {
		history = slices.Clone(history[start:])
	}
	return history
}

func (item *itemData) setEditedText(value string) {
	if item.HideText {
		item.SecretText = value
		item.Text = strings.Repeat("*", len([]rune(value)))
	} else {
		item.Text = value
	}
	if item.TextPtr != nil {
		// NewInput's default TextPtr points to the display string.
		if !item.HideText || item.TextPtr != &item.Text {
			*item.TextPtr = value
		}
	}
	item.textEdit.known = value
	item.textEdit.layout = nil
}

func (item *itemData) emitTextChanged() {
	item.resetCaret()
	if item.Handler != nil {
		item.Handler.Emit(UIEvent{Item: item, Type: EventInputChanged, Text: item.Text})
	}
}

func (item *itemData) editReplace(start, end int, insert, group string) {
	s := item.editor()
	runes := []rune(item.editText())
	start, end = max(0, min(start, len(runes))), max(0, min(end, len(runes)))
	if start > end {
		start, end = end, start
	}
	value := string(runes[:start]) + insert + string(runes[end:])
	if value == item.editText() {
		return
	}
	if !item.HideText {
		if group == "" || group != s.group || updateNow.Sub(s.changed) > time.Second || len(s.redo) > 0 {
			s.undo = trimTextHistory(append(s.undo, item.editSnapshot()))
		}
		s.redo = nil
	}
	s.group, s.changed, s.hasPreferredX = group, updateNow, false
	item.setEditedText(value)
	pos := start + len([]rune(insert))
	item.editSelect(pos, pos)
	item.emitTextChanged()
}

func (item *itemData) editInsert(value, group string) {
	value = strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n")
	value = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			if item.Multiline {
				return r
			}
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
	if value == "" {
		return
	}
	start, end := item.editSelection()
	if start == end {
		start, end = item.CursorPos, item.CursorPos
	} else {
		group = ""
	}
	item.editReplace(start, end, value, group)
}

func (item *itemData) editUndo(redo bool) {
	s := item.editor()
	if item.HideText {
		return
	}
	from, to := &s.undo, &s.redo
	if redo {
		from, to = to, from
	}
	if len(*from) == 0 {
		return
	}
	snapshot := (*from)[len(*from)-1]
	*to = trimTextHistory(append(*to, item.editSnapshot()))
	*from = (*from)[:len(*from)-1]
	s.group, s.hasPreferredX = "", false
	item.setEditedText(snapshot.text)
	item.CursorPos, item.SelectStart, item.SelectEnd = snapshot.cursor, snapshot.anchor, snapshot.end
	selectedTextItem = item
	item.emitTextChanged()
}

func graphemeStops(value string) []int {
	var seg segmenter.Segmenter
	seg.Init([]rune(value))
	it := seg.GraphemeIterator()
	stops := []int{0}
	for it.Next() {
		g := it.Grapheme()
		stops = append(stops, g.Offset+len(g.Text))
	}
	return stops
}

func editCharBoundary(value string, pos, direction int) int {
	stops := graphemeStops(value)
	if direction < 0 {
		for i := len(stops) - 1; i >= 0; i-- {
			if stops[i] < pos {
				return stops[i]
			}
		}
		return 0
	}
	for _, p := range stops {
		if p > pos {
			return p
		}
	}
	return stops[len(stops)-1]
}

func editWordClass(r rune) int {
	if unicode.IsSpace(r) {
		return 0
	}
	if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) || r == '_' {
		return 1
	}
	return 2
}

func editWordBoundary(value string, pos, direction int, mac bool) int {
	r := []rune(value)
	pos = max(0, min(pos, len(r)))
	if direction < 0 {
		for pos > 0 && editWordClass(r[pos-1]) == 0 {
			pos--
		}
		if pos > 0 {
			class := editWordClass(r[pos-1])
			for pos > 0 && editWordClass(r[pos-1]) == class {
				pos--
			}
		}
	} else {
		if mac {
			for pos < len(r) && editWordClass(r[pos]) == 0 {
				pos++
			}
		}
		if pos < len(r) {
			class := editWordClass(r[pos])
			for pos < len(r) && editWordClass(r[pos]) == class {
				pos++
			}
		}
		if !mac {
			for pos < len(r) && editWordClass(r[pos]) == 0 {
				pos++
			}
		}
	}
	return pos
}

func editLineBounds(value string, pos int) (int, int) {
	r := []rune(value)
	start, end := max(0, min(pos, len(r))), max(0, min(pos, len(r)))
	for start > 0 && r[start-1] != '\n' {
		start--
	}
	for end < len(r) && r[end] != '\n' {
		end++
	}
	return start, end
}

func (item *itemData) editIndent(outdent bool) {
	start, end := item.editSelection()
	oldCursor, hadSelection := item.CursorPos, start != end
	if start == end && !outdent {
		item.editInsert("\t", "")
		return
	}
	r := []rune(item.editText())
	if start == end {
		start, end = item.CursorPos, item.CursorPos
	}
	start, _ = editLineBounds(string(r), start)
	// A selection ending at the beginning of a line does not indent that line.
	if end > start && r[end-1] == '\n' {
		end--
	}
	_, end = editLineBounds(string(r), end)
	lines := strings.Split(string(r[start:end]), "\n")
	for i, line := range lines {
		if !outdent {
			lines[i] = "\t" + line
			continue
		}
		if strings.HasPrefix(line, "\t") {
			lines[i] = line[1:]
		} else {
			n := 0
			for n < min(4, len(line)) && line[n] == ' ' {
				n++
			}
			lines[i] = line[n:]
		}
	}
	insert := strings.Join(lines, "\n")
	item.editReplace(start, end, insert, "")
	if hadSelection {
		item.editSelect(start, start+len([]rune(insert)))
	} else {
		pos := max(start, oldCursor-(end-start-len([]rune(insert))))
		item.editSelect(pos, pos)
	}
}

// editKey is independent of physical key polling, making platform shortcut and
// selection behavior testable without synthesizing OS input.
func (item *itemData) editKey(key ebiten.Key, mods inputkeys.Modifiers, shift bool) bool {
	if !itemHandlesTextEditing(item) {
		return false
	}
	s := item.editor()
	if mods.Shortcut() {
		switch key {
		case ebiten.KeyA:
			s.group = ""
			item.editSelect(0, len([]rune(item.editText())))
			return true
		case ebiten.KeyZ:
			item.editUndo(shift)
			return true
		case ebiten.KeyY:
			item.editUndo(true)
			return true
		}
	}
	pos := item.CursorPos
	start, end := item.editSelection()
	direction := 1
	if key == ebiten.KeyArrowLeft || key == ebiten.KeyBackspace || key == ebiten.KeyArrowUp || key == ebiten.KeyPageUp {
		direction = -1
	}
	boundary := func() int {
		if mods.Line() {
			a, b := editLineBounds(item.editText(), pos)
			if direction < 0 {
				return a
			}
			return b
		}
		if mods.Word() {
			return editWordBoundary(item.editText(), pos, direction, mods.Mac)
		}
		return editCharBoundary(item.editText(), pos, direction)
	}
	switch key {
	case ebiten.KeyArrowLeft, ebiten.KeyArrowRight:
		s.hasPreferredX = false
		if !shift && start != end {
			pos = end
			if direction < 0 {
				pos = start
			}
		} else {
			pos = boundary()
		}
		item.editMove(pos, shift)
	case ebiten.KeyHome, ebiten.KeyEnd:
		s.hasPreferredX = false
		a, b := editLineBounds(item.editText(), pos)
		if mods.Shortcut() {
			a, b = 0, len([]rune(item.editText()))
		}
		pos = b
		if key == ebiten.KeyHome {
			pos = a
		}
		item.editMove(pos, shift)
	case ebiten.KeyArrowUp, ebiten.KeyArrowDown, ebiten.KeyPageUp, ebiten.KeyPageDown:
		if !item.Multiline {
			return false
		}
		if mods.Line() {
			pos = len([]rune(item.editText()))
			if direction < 0 {
				pos = 0
			}
			item.editMove(pos, shift)
		} else {
			item.editVertical(direction, key == ebiten.KeyPageUp || key == ebiten.KeyPageDown, shift)
		}
	case ebiten.KeyBackspace, ebiten.KeyDelete:
		group := ""
		if start == end {
			start, end = pos, boundary()
			group = "delete"
			if direction < 0 {
				group = "backspace"
			}
		}
		item.editReplace(start, end, "", group)
	case ebiten.KeyEnter, ebiten.KeyKPEnter:
		if !item.Multiline {
			return false
		}
		item.editInsert("\n", "")
	case ebiten.KeyTab:
		if !item.Multiline || !item.AcceptTab || mods.Control || mods.Meta {
			return false
		}
		item.editIndent(shift)
	default:
		return false
	}
	return true
}
