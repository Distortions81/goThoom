package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"gothoom/eui"
	"gothoom/internal/inputkeys"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/gomono"
)

type sourceEditor struct {
	options                sourceEditorOptions
	doc                    *sourceDocument
	win                    *eui.WindowData
	root, input, status    *eui.ItemData
	saveButton             *eui.ItemData
	undoButton, redoButton *eui.ItemData
	toolbar                *eui.ItemData
	actions                []*eui.ItemData
	confirm                *eui.WindowData
	discard                bool
	message                string
	scale                  float32
	highlightColors        eui.SyntaxColors
}

var sourceEditorFont *text.GoTextFaceSource

var sourceEditors = map[string]*sourceEditor{}
var sourceEditorQuitPrompt *eui.WindowData
var sourceEditorQuitRequested bool

type sourceEditorOptions struct {
	kind, reloadTooltip, checkedMessage, savedMessage string
	check                                             func(string) error
	reload                                            func() (string, error)
	afterSave                                         func()
	highlight                                         func(string, eui.SyntaxColors) []eui.TextColorSpan
	format                                            func(string) (string, error)
	formatPosition                                    func(before, after string, position int, trailing bool) int
	lint                                              func(string) []string
}

func openSourceEditor(doc *sourceDocument, options sourceEditorOptions) *sourceEditor {
	if existing := sourceEditors[doc.path]; existing != nil {
		existing.win.MarkOpen()
		existing.focus()
		return existing
	}
	if sourceEditorFont == nil {
		var err error
		sourceEditorFont, err = text.NewGoTextFaceSource(bytes.NewReader(gomono.TTF))
		if err != nil {
			consoleMessage("[editor] load font: " + err.Error())
			return nil
		}
	}
	ed := &sourceEditor{options: options, doc: doc, win: eui.NewWindow(), root: eui.NewColumn(), status: eui.NewLabel("")}
	win := ed.win
	win.Title = "Edit " + options.kind + " — " + filepath.Base(doc.path)
	win.Closable, win.Movable, win.Resizable, win.NoScroll = true, true, true, true
	win.Size = eui.Point{X: 780, Y: 540}
	win.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)
	ed.toolbar = eui.NewColumn()
	ed.root.AddItem(ed.toolbar)
	input, events := eui.NewTextArea()
	ed.input = input
	input.Text, input.AcceptTab = doc.savedText, true
	input.FontSize = 14
	if options.highlight != nil {
		ed.highlightColors = sourceEditorColors()
		input.SetTextHighlighter(func(value string) []eui.TextColorSpan {
			return options.highlight(value, sourceEditorColors())
		})
	}
	win.Searchable = true
	win.OnSearch = func(query string) { ed.find(query, false, false) }
	win.OnSearchNext = func(backward bool) { ed.find(win.SearchText, true, backward) }
	events.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventInputChanged {
			ed.message = ""
			ed.refreshStatus()
		}
	}
	ed.root.AddItem(input)
	ed.status.Size = eui.Point{X: 740, Y: 48}
	ed.status.ConstrainToSize = true
	ed.root.AddItem(ed.status)
	addButton := func(label string, action func()) *eui.ItemData {
		button := eui.NewActionButton(label, action)
		button.Size = eui.Point{X: 64, Y: 24}
		button.FontSize = 11
		ed.actions = append(ed.actions, button)
		return button
	}
	ed.undoButton = addButton("Undo", func() { input.Undo(); ed.focus(); ed.refreshStatus() })
	setMaterialButtonIcon(ed.undoButton, "undo")
	ed.undoButton.SetTooltip(inputkeys.ShortcutLabel() + "+Z undoes the last edit, including formatting.")
	ed.redoButton = addButton("Redo", func() { input.Redo(); ed.focus(); ed.refreshStatus() })
	setMaterialButtonIcon(ed.redoButton, "redo")
	ed.redoButton.SetTooltip(inputkeys.ShortcutLabel() + "+Shift+Z redoes the last undone edit.")
	checkButton := addButton("Check", func() { ed.check() })
	if options.lint != nil {
		checkButton.SetTooltip("Check syntax and lint warnings. Full diagnostics appear in Console.")
	}
	if options.format != nil {
		addButton("Format", func() { ed.format(); ed.focus() }).SetTooltip(inputkeys.ShortcutLabel() + "+Shift+I formats the draft. Saving also formats it.")
	}
	ed.saveButton = addButton("Save", func() { ed.save(false) })
	ed.saveButton.SetTooltip(inputkeys.ShortcutLabel() + "+S saves this file.")
	addButton("Save & Reload", func() { ed.save(true) }).SetTooltip(options.reloadTooltip)
	addButton("Settings", openSourceEditorSettings).SetTooltip("Choose theme or custom syntax colors for all source editors.")
	addButton("Close", win.Close)
	win.AddItem(ed.root)
	win.OnResize = ed.layout
	win.BeforeClose = ed.beforeClose
	win.OnClose = func() {
		eui.ClearFocus(input)
		delete(sourceEditors, doc.path)
		if ed.confirm != nil {
			ed.confirm.Close()
			ed.confirm = nil
		}
		win.RemoveWindow()
	}
	sourceEditors[doc.path] = ed
	if options.format != nil {
		ed.format()
	}
	win.AddWindow(false)
	win.MarkOpen()
	ed.layout()
	ed.focus()
	return ed
}

// Return to the draft without moving its cursor or dropping its selection.
func (ed *sourceEditor) focus() {
	cursor, anchor, end := ed.input.CursorPos, ed.input.SelectStart, ed.input.SelectEnd
	eui.Focus(ed.input)
	ed.input.CursorPos, ed.input.SelectStart, ed.input.SelectEnd = cursor, anchor, end
	ed.input.Dirty, ed.win.Dirty = true, true
}

func (ed *sourceEditor) layout() {
	ed.scale = eui.UIScale()
	if sourceEditorFont != nil {
		ed.input.Face = &text.GoTextFace{Source: sourceEditorFont, Size: float64(ed.input.FontSize*ed.scale + 2)}
	}
	eui.LayoutWindowBody(ed.win, ed.root, ed.input)
	// Keep actions reachable when a window or a high-DPI display is narrow.
	var rows []*eui.ItemData
	row := eui.NewRow()
	used := float32(0)
	for _, button := range ed.actions {
		width := button.GetSize().X/ed.scale + button.Position.X
		if used > 0 && used+width > ed.root.Size.X-ed.toolbar.Position.X {
			rows = append(rows, row)
			row, used = eui.NewRow(), 0
		}
		row.AddItem(button)
		used += width
	}
	rows = append(rows, row)
	ed.toolbar.SetItems(rows)
	ed.toolbar.Size.Y = 0
	eui.LayoutWindowBody(ed.win, ed.root, ed.input)
	ed.refreshStatus()
	ed.win.Refresh()
}

func (ed *sourceEditor) dirty() bool { return ed.doc.changed(ed.input.Text) }

func (ed *sourceEditor) find(query string, next, backward bool) {
	if query == "" {
		ed.setStatus("")
		return
	}
	start := 0
	if next {
		start = max(ed.input.SelectStart, ed.input.SelectEnd)
		if backward {
			start = min(ed.input.SelectStart, ed.input.SelectEnd) - 1
		}
	}
	if ed.input.FindText(query, start, backward) {
		ed.setStatus("Match selected. Enter / Shift+Enter finds the next / previous match; F3 also works while editing.")
	} else {
		ed.setStatus("No matches for " + fmt.Sprintf("%q", query) + ".")
	}
}

func (ed *sourceEditor) refreshStatus() {
	ed.refreshHistoryButtons()
	dirty := ed.dirty()
	ed.win.Title = "Edit " + ed.options.kind + " — " + filepath.Base(ed.doc.path)
	if dirty {
		ed.win.Title += " *"
	}
	ed.saveButton.Disabled = !dirty
	ed.saveButton.Dirty = true
	message := ed.message
	if message == "" {
		message = "Saved."
		if dirty {
			message = "Unsaved changes."
		}
	}
	face := &text.GoTextFace{Source: eui.FontSource(), Size: float64(ed.status.FontSize*eui.UIScale() + 2)}
	_, lines := eui.WrapText(message, face, float64(max(1, ed.status.Size.X*eui.UIScale())))
	if len(lines) > 2 {
		lines = append(lines[:1], lines[1]+"…")
	}
	ed.status.Text = strings.Join(lines, "\n")
	ed.status.SetTooltip(message)
	ed.status.Dirty = true
	ed.win.Dirty = true
}

func (ed *sourceEditor) refreshHistoryButtons() {
	update := func(button *eui.ItemData, enabled bool) {
		if button != nil && button.Disabled == enabled {
			button.Disabled = !enabled
			button.Dirty, ed.win.Dirty = true, true
		}
	}
	update(ed.undoButton, ed.input.CanUndo())
	update(ed.redoButton, ed.input.CanRedo())
}

func (ed *sourceEditor) setStatus(message string) { ed.message = message; ed.refreshStatus() }

func (ed *sourceEditor) check() bool {
	if err := ed.options.check(ed.input.Text); err != nil {
		consoleMessage("[editor] " + err.Error())
		ed.setStatus("Check failed: " + err.Error())
		return false
	}
	ed.setStatus(ed.options.checkedMessage)
	if ed.options.lint != nil {
		warnings := ed.options.lint(ed.input.Text)
		for _, warning := range warnings {
			consoleMessage("[editor] lint: " + warning)
		}
		if len(warnings) > 0 {
			ed.setStatus(fmt.Sprintf("No syntax errors; %d lint warning(s). %s. Details are in Console.", len(warnings), warnings[0]))
		}
	}
	return true
}

func (ed *sourceEditor) format() bool {
	if ed.options.format == nil {
		return true
	}
	before := ed.input.Text
	value, err := ed.options.format(before)
	if err != nil {
		ed.setStatus("Not formatted: " + err.Error())
		return false
	}
	if value == before {
		ed.setStatus("Already formatted.")
		return true
	}
	cursor, start, end := ed.input.CursorPos, ed.input.SelectStart, ed.input.SelectEnd
	remap := func(position int, trailing bool) int {
		if ed.options.formatPosition != nil {
			return ed.options.formatPosition(before, value, position, trailing)
		}
		return remapFormattedPosition(before, value, position)
	}
	ed.input.ReplaceText(value)
	// Keep selection edges on their own side of newly inserted whitespace.
	// A collapsed selection follows the caret, without selecting new spaces.
	ed.input.CursorPos = remap(cursor, start == end || cursor != min(start, end))
	ed.input.SelectStart = remap(start, start >= end)
	ed.input.SelectEnd = remap(end, end >= start)
	ed.setStatus("Formatted. Undo restores the previous draft.")
	return true
}

// Formatting changes only indentation and trailing whitespace, not line order.
func remapFormattedPosition(before, after string, position int) int {
	old := []rune(before)
	position = max(0, min(position, len(old)))
	line := strings.Count(string(old[:position]), "\n")
	oldLines, newLines := strings.Split(before, "\n"), strings.Split(after, "\n")
	if line >= len(newLines) {
		return len([]rune(after))
	}
	offset, oldOffset := 0, 0
	for i := 0; i < line; i++ {
		offset += len([]rune(newLines[i])) + 1
		oldOffset += len([]rune(oldLines[i])) + 1
	}
	oldIndent := len([]rune(oldLines[line])) - len([]rune(strings.TrimLeft(oldLines[line], " \t")))
	newIndent := len([]rune(newLines[line])) - len([]rune(strings.TrimLeft(newLines[line], " \t")))
	column := position - oldOffset
	if column >= oldIndent {
		column += newIndent - oldIndent
	} else {
		column = min(column, newIndent)
	}
	return offset + max(0, min(column, len([]rune(newLines[line]))))
}

func (ed *sourceEditor) save(reload bool) bool {
	if reload && !ed.check() {
		return false
	}
	formatMessage := ""
	if !ed.format() {
		formatMessage = " Saved without formatting: " + strings.TrimPrefix(ed.message, "Not formatted: ")
	}
	if err := ed.doc.save(ed.input.Text); err != nil {
		ed.setStatus("Not saved: " + err.Error())
		return false
	}
	message := ed.options.savedMessage
	if reload {
		result, err := ed.options.reload()
		if err != nil {
			message = "Saved; reload reported an error: " + err.Error()
			consoleMessage("[editor] " + message)
		} else {
			message = result
		}
	}
	if ed.options.afterSave != nil {
		ed.options.afterSave()
	}
	ed.setStatus(message + formatMessage)
	return true
}

func (ed *sourceEditor) beforeClose() bool {
	if ed.discard || !ed.dirty() {
		return true
	}
	if ed.confirm != nil && ed.confirm.IsOpen() {
		ed.confirm.BringForward()
		return false
	}
	ed.confirm = eui.ShowPopup("Unsaved editor changes", "Save changes to "+filepath.Base(ed.doc.path)+"?", []eui.PopupButton{
		{Text: "Keep Editing", Action: func() { ed.confirm = nil; ed.focus() }},
		{Text: "Discard", Action: func() { ed.confirm = nil; ed.discard = true; ed.win.Close() }},
		{Text: "Save & Close", Action: func() {
			ed.confirm = nil
			if ed.save(false) {
				ed.win.Close()
			}
		}},
	})
	return false
}

func updateSourceEditors() {
	refreshSourceEditorSettings()
	for _, ed := range sourceEditors {
		ed.refreshHistoryButtons()
		ed.refreshHighlighting()
		if ed.scale != eui.UIScale() {
			ed.layout()
		}
		if ed.win.IsOpen() && ed.input.Focused && inputkeys.Current().Shortcut() && inpututil.IsKeyJustPressed(ebiten.KeyS) {
			ed.save(false)
		}
		if ed.win.IsOpen() && ed.input.Focused && inputkeys.Current().Shortcut() && eui.ShiftPressed && inpututil.IsKeyJustPressed(ebiten.KeyI) && ed.options.format != nil {
			ed.format()
		}
	}
}

func (ed *sourceEditor) refreshHighlighting() {
	if ed.options.highlight != nil && ed.highlightColors != sourceEditorColors() {
		ed.highlightColors = sourceEditorColors()
		ed.input.RefreshTextHighlighting()
	}
}

func dirtySourceEditors() []*sourceEditor {
	var dirty []*sourceEditor
	for _, ed := range sourceEditors {
		if ed.dirty() {
			dirty = append(dirty, ed)
		}
	}
	sort.Slice(dirty, func(i, j int) bool { return dirty[i].doc.path < dirty[j].doc.path })
	return dirty
}

// Returns true when the continuation is deferred for an unsaved-changes choice.
// The popup is nonmodal, so Save All reads the current drafts when clicked.
func confirmSourceEditorQuit(quit func()) bool {
	if len(dirtySourceEditors()) == 0 {
		return false
	}
	if sourceEditorQuitPrompt != nil && sourceEditorQuitPrompt.IsOpen() {
		sourceEditorQuitPrompt.BringForward()
		return true
	}
	sourceEditorQuitPrompt = eui.ShowPopup("Unsaved editor changes", "Save your macro and script changes before quitting?", []eui.PopupButton{
		{Text: "Keep Editing", Action: func() { sourceEditorQuitPrompt = nil }},
		{Text: "Discard & Quit", Width: 144, Action: func() { sourceEditorQuitPrompt = nil; quit() }},
		{Text: "Save All & Quit", Width: 144, Action: func() {
			sourceEditorQuitPrompt = nil
			for _, ed := range dirtySourceEditors() {
				if !ed.save(false) {
					ed.win.BringForward()
					return
				}
			}
			quit()
		}},
	})
	return true
}
