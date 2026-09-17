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

type legacyMacroEditor struct {
	doc                 *legacyMacroDocument
	win                 *eui.WindowData
	root, input, status *eui.ItemData
	saveButton          *eui.ItemData
	footer              *eui.ItemData
	actions             []*eui.ItemData
	confirm             *eui.WindowData
	discard             bool
	message             string
	scale               float32
}

var legacyMacroEditorFont *text.GoTextFaceSource

var legacyMacroEditors = map[string]*legacyMacroEditor{}
var legacyMacroQuitPrompt *eui.WindowData
var legacyMacroQuitRequested bool

func openLegacyMacroEditor(entry legacyMacroLibraryEntry) *legacyMacroEditor {
	doc, err := loadLegacyMacroDocument(entry.Path)
	if err != nil {
		legacyMacroLibraryReport("open macro editor: " + err.Error())
		return nil
	}
	if existing := legacyMacroEditors[doc.path]; existing != nil {
		existing.win.MarkOpen()
		existing.focus()
		return existing
	}
	if legacyMacroEditorFont == nil {
		legacyMacroEditorFont, err = text.NewGoTextFaceSource(bytes.NewReader(gomono.TTF))
		if err != nil {
			legacyMacroLibraryReport("load editor font: " + err.Error())
			return nil
		}
	}
	ed := &legacyMacroEditor{doc: doc, win: eui.NewWindow(), root: eui.NewColumn(), status: eui.NewLabel("")}
	win := ed.win
	win.Title = "Edit Macro — " + filepath.Base(doc.path)
	win.Closable, win.Movable, win.Resizable, win.NoScroll = true, true, true, true
	win.Size = eui.Point{X: 780, Y: 540}
	win.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)
	intro := eui.NewLabel("Edits apply to every character using this file. Save & Reload also reloads the selected session's enabled macros.")
	intro.Size = eui.Point{X: 740, Y: 42}
	intro.ConstrainToSize = true
	ed.root.AddItem(intro)
	input, events := eui.NewTextArea()
	ed.input = input
	input.Text, input.AcceptTab = doc.savedText, true
	input.FontSize = 14
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
	buttons := eui.NewColumn()
	ed.footer = buttons
	addButton := func(label string, action func()) *eui.ItemData {
		button := eui.NewActionButton(label, action)
		button.Size = eui.Point{X: 100, Y: 28}
		ed.actions = append(ed.actions, button)
		return button
	}
	addButton("Check", func() { ed.check() })
	ed.saveButton = addButton("Save", func() { ed.save(false) })
	ed.saveButton.SetTooltip(inputkeys.ShortcutLabel() + "+S saves this file.")
	addButton("Save & Reload", func() { ed.save(true) }).Size.X = 144
	addButton("Close", win.Close)
	ed.root.AddItem(buttons)
	win.AddItem(ed.root)
	win.OnResize = ed.layout
	win.BeforeClose = ed.beforeClose
	win.OnClose = func() {
		eui.ClearFocus(input)
		delete(legacyMacroEditors, doc.path)
		if ed.confirm != nil {
			ed.confirm.Close()
			ed.confirm = nil
		}
		win.RemoveWindow()
	}
	legacyMacroEditors[doc.path] = ed
	win.AddWindow(false)
	win.MarkOpen()
	ed.layout()
	ed.focus()
	return ed
}

// Return to the draft without moving its cursor or dropping its selection.
func (ed *legacyMacroEditor) focus() {
	cursor, anchor, end := ed.input.CursorPos, ed.input.SelectStart, ed.input.SelectEnd
	eui.Focus(ed.input)
	ed.input.CursorPos, ed.input.SelectStart, ed.input.SelectEnd = cursor, anchor, end
	ed.input.Dirty, ed.win.Dirty = true, true
}

func (ed *legacyMacroEditor) layout() {
	ed.scale = eui.UIScale()
	if legacyMacroEditorFont != nil {
		ed.input.Face = &text.GoTextFace{Source: legacyMacroEditorFont, Size: float64(ed.input.FontSize*ed.scale + 2)}
	}
	eui.LayoutWindowBody(ed.win, ed.root, ed.input)
	// Keep actions reachable when a window or a high-DPI display is narrow.
	var rows []*eui.ItemData
	row := eui.NewRow()
	used := float32(0)
	for _, button := range ed.actions {
		width := button.GetSize().X/ed.scale + button.Position.X
		if used > 0 && used+width > ed.root.Size.X-ed.footer.Position.X {
			rows = append(rows, row)
			row, used = eui.NewRow(), 0
		}
		row.AddItem(button)
		used += width
	}
	rows = append(rows, row)
	ed.footer.SetItems(rows)
	ed.footer.Size.Y = 0
	intro := ed.root.Contents[0]
	face := &text.GoTextFace{Source: eui.FontSource(), Size: float64(intro.FontSize*ed.scale + 2)}
	_, lines := eui.WrapText("Edits apply to every character using this file. Save & Reload also reloads the selected session's enabled macros.", face, float64(intro.Size.X*ed.scale))
	intro.Text = strings.Join(lines, "\n")
	intro.Size.Y = float32(len(lines)) * 18
	eui.LayoutWindowBody(ed.win, ed.root, ed.input)
	ed.refreshStatus()
	ed.win.Refresh()
}

func (ed *legacyMacroEditor) dirty() bool { return ed.doc.changed(ed.input.Text) }

func (ed *legacyMacroEditor) find(query string, next, backward bool) {
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

func (ed *legacyMacroEditor) refreshStatus() {
	dirty := ed.dirty()
	ed.win.Title = "Edit Macro — " + filepath.Base(ed.doc.path)
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

func (ed *legacyMacroEditor) setStatus(message string) { ed.message = message; ed.refreshStatus() }

func (ed *legacyMacroEditor) check() bool {
	program := ed.doc.check(ed.input.Text)
	if len(program.Diagnostics) == 0 {
		ed.setStatus("No macro syntax errors found.")
		return true
	}
	for _, diagnostic := range program.Diagnostics {
		legacyMacroLibraryReport(diagnostic.Error())
	}
	first := program.Diagnostics[0]
	ed.setStatus(fmt.Sprintf("Check found %d issue(s). %s:%d:%d: %s. Details are in Console.", len(program.Diagnostics), filepath.Base(first.Location.Path), first.Location.Line, first.Location.Column, first.Message))
	return false
}

func (ed *legacyMacroEditor) save(reload bool) bool {
	if reload && !ed.check() {
		return false
	}
	if err := ed.doc.save(ed.input.Text); err != nil {
		ed.setStatus("Not saved: " + err.Error())
		return false
	}
	message := "Saved. Use Save & Reload to apply it to this session's enabled macros."
	if reload {
		if err := legacyMacroLibrarySession().loadLegacyMacrosForCharacter(legacyMacroLibraryCurrentCharacter()); err != nil {
			message = "Saved; reload reported an error: " + err.Error()
			legacyMacroLibraryReport(message)
		} else {
			message = "Saved; selected session's enabled macros reloaded."
		}
	}
	refreshLegacyMacroLibraryWindow()
	ed.setStatus(message)
	return true
}

func (ed *legacyMacroEditor) beforeClose() bool {
	if ed.discard || !ed.dirty() {
		return true
	}
	if ed.confirm != nil && ed.confirm.IsOpen() {
		ed.confirm.BringForward()
		return false
	}
	ed.confirm = eui.ShowPopup("Unsaved macro changes", "Save changes to "+filepath.Base(ed.doc.path)+"?", []eui.PopupButton{
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

func updateLegacyMacroEditors() {
	for _, ed := range legacyMacroEditors {
		if ed.scale != eui.UIScale() {
			ed.layout()
		}
		if ed.win.IsOpen() && ed.input.Focused && inputkeys.Current().Shortcut() && inpututil.IsKeyJustPressed(ebiten.KeyS) {
			ed.save(false)
		}
	}
}

func dirtyLegacyMacroEditors() []*legacyMacroEditor {
	var dirty []*legacyMacroEditor
	for _, ed := range legacyMacroEditors {
		if ed.dirty() {
			dirty = append(dirty, ed)
		}
	}
	sort.Slice(dirty, func(i, j int) bool { return dirty[i].doc.path < dirty[j].doc.path })
	return dirty
}

// Returns true when the continuation is deferred for an unsaved-changes choice.
// The popup is nonmodal, so Save All reads the current drafts when clicked.
func confirmLegacyMacroEditorQuit(quit func()) bool {
	if len(dirtyLegacyMacroEditors()) == 0 {
		return false
	}
	if legacyMacroQuitPrompt != nil && legacyMacroQuitPrompt.IsOpen() {
		legacyMacroQuitPrompt.BringForward()
		return true
	}
	legacyMacroQuitPrompt = eui.ShowPopup("Unsaved macro changes", "Save your macro changes before quitting?", []eui.PopupButton{
		{Text: "Keep Editing", Action: func() { legacyMacroQuitPrompt = nil }},
		{Text: "Discard & Quit", Width: 144, Action: func() { legacyMacroQuitPrompt = nil; quit() }},
		{Text: "Save All & Quit", Width: 144, Action: func() {
			legacyMacroQuitPrompt = nil
			for _, ed := range dirtyLegacyMacroEditors() {
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
