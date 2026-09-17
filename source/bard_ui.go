package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/skratchdot/open-golang/open"
	"gothoom/eui"
)

type bardActionGroup struct {
	column  *eui.ItemData
	buttons []*eui.ItemData
}

type bardPanel struct {
	groups                                                                                            []bardActionGroup
	win                                                                                               *eui.WindowData
	root, list, details, instrument, status, edit, previewButton, playButton, stopButton, stopPreview *eui.ItemData
	tunes                                                                                             []bardTune
	selected                                                                                          string
	query                                                                                             string
	session                                                                                           *Session
	revision                                                                                          uint64
	connected                                                                                         bool
	preview                                                                                           *bardPreview
	performance                                                                                       *bardPerformance
}

var bardWindow *bardPanel

func showBardWindow() {
	if isWASM {
		consoleMessage("[bard] Tunes are available in the desktop client.")
		return
	}
	if bardWindow != nil {
		bardWindow.reload()
		bardWindow.win.MarkOpen()
		bardWindow.win.BringForward()
		return
	}
	p := &bardPanel{win: eui.NewWindow(), root: eui.NewColumn()}
	bardWindow = p
	win := p.win
	win.Title = "Bard"
	win.Closable, win.Movable, win.Resizable, win.NoScroll = true, true, true, true
	win.Size = eui.Point{X: 640, Y: 520}
	win.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)
	win.OnClose = func() {
		p.preview.stop()
		p.performance.stop()
		win.RemoveWindow()
		if bardWindow == p {
			bardWindow = nil
		}
	}
	win.Searchable = true
	win.OnSearch = func(query string) { p.query = query; p.refreshList() }
	newButton := eui.NewActionButton("New", func() { p.newTune() })
	setMaterialButtonIcon(newButton, "add")
	p.addActions(newButton, eui.NewActionButton("Import", p.importTune), eui.NewActionButton("Open Folder", func() { p.setError(open.Run(bardTunesDir())) }), eui.NewActionButton("Refresh", p.reload))
	p.list = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true, Fixed: true}
	p.root.AddItem(p.list)
	p.details = eui.NewWrappedLabel("Select a tune or create one.", 580)
	p.root.AddItem(p.details)
	p.instrument, _ = eui.NewDropdown()
	p.instrument.Label = "Instrument"
	p.instrument.Size = eui.Point{X: 360, Y: 28}
	p.instrument.Handler.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventDropdownSelected {
			if tune := p.tune(); tune != nil {
				index := p.instrument.Selected
				if err := saveBardInstrument(*tune, index); err != nil {
					p.setError(err)
					p.refreshSelection()
					return
				}
				tune.Instrument = index
				p.preview.stop()
				p.preview = nil
				p.refreshSelection()
				p.refreshList()
			}
		}
	}
	p.root.AddItem(p.instrument)
	p.edit = eui.NewActionButton("Edit", p.editTune)
	setMaterialButtonIcon(p.edit, "edit")
	p.previewButton = eui.NewActionButton("Preview", p.startPreview)
	p.previewButton.SetTooltip("Listen locally using the selected instrument. Does not perform in-game.")
	p.stopPreview = eui.NewActionButton("Stop Preview", func() { p.preview.stop(); p.preview = nil; p.refreshSelection() })
	p.addActions(p.edit, p.previewButton, p.stopPreview)
	p.playButton = eui.NewActionButton("Play in Game", p.play)
	p.playButton.SetTooltip("Equip the selected inventory instrument and perform the saved tune in the selected session.")
	p.stopButton = eui.NewActionButton("Stop Playing", func() { p.performance.stop(); p.performance = nil; p.refreshSelection() })
	p.addActions(p.playButton, p.stopButton)
	p.status = eui.NewWrappedLabel("", 580)
	p.status.Invisible = true
	p.root.AddItem(p.status)
	win.AddItem(p.root)
	win.OnResize = p.refreshList
	win.AddWindow(false)
	p.reload()
	win.MarkOpen()
}
func (p *bardPanel) tune() *bardTune {
	for i := range p.tunes {
		if p.tunes[i].Path == p.selected {
			return &p.tunes[i]
		}
	}
	return nil
}
func (p *bardPanel) setError(err error) {
	message := ""
	if err != nil {
		message = err.Error()
	}
	p.status.SetWrappedText(message)
	p.status.Invisible = message == ""
	p.layout()
}
func (p *bardPanel) reload() {
	tunes, err := listBardTunes()
	if err != nil {
		p.setError(err)
		return
	}
	p.tunes = tunes
	p.setError(nil)
	p.refreshList()
	p.refreshSelection()
}
func (p *bardPanel) addActions(buttons ...*eui.ItemData) {
	column := eui.NewColumn()
	p.root.AddItem(column)
	p.groups = append(p.groups, bardActionGroup{column, buttons})
}
func (p *bardPanel) layout() {
	eui.LayoutWindowBody(p.win, p.root, p.list)
	for _, group := range p.groups {
		row := eui.NewRow()
		rows := []*eui.ItemData{}
		used := float32(0)
		for _, button := range group.buttons {
			width := button.GetSize().X/eui.UIScale() + button.Position.X
			if used > 0 && used+width > p.root.Size.X-group.column.Position.X {
				rows = append(rows, row)
				row = eui.NewRow()
				used = 0
			}
			row.AddItem(button)
			used += width
		}
		rows = append(rows, row)
		group.column.SetItems(rows)
		group.column.Size.Y = 0
	}
	eui.LayoutWindowBody(p.win, p.root, p.list)
	width := savedDataContentWidth(p.list.Size.X)
	p.details.Size.X = width
	p.status.Size.X = width
	p.instrument.Size.X = width
	if width > 360 {
		p.instrument.Size.X = 360
	}
	p.win.Refresh()
}
func (p *bardPanel) refreshList() {
	p.layout()
	p.list.SetItems(nil)
	width := savedDataContentWidth(p.list.Size.X)
	for _, tune := range p.tunes {
		if !strings.Contains(strings.ToLower(tune.Name), strings.ToLower(p.query)) {
			continue
		}
		tune := tune
		label := tune.Name + "  ·  " + classicInstrumentNames[tune.Instrument]
		button := eui.NewActionButton(label, func() { p.selected = tune.Path; p.setError(nil); p.refreshSelection(); p.refreshList() })
		button.Size = eui.Point{X: width, Y: 30}
		button.SelectionIndicator = tune.Path == p.selected
		button.ConstrainToSize = true
		p.list.AddItem(button)
	}
	if len(p.list.Contents) == 0 {
		p.list.AddItem(eui.NewWrappedLabel("No tunes found. Create a tune or import a CL tune text file.", width))
	}
	p.win.Refresh()
}
func (p *bardPanel) refreshSelection() {
	session := selectedAppSession()
	p.session = session
	p.connected = session != nil && session.transport.connected()
	if session != nil {
		p.revision = session.inventory.revision.Load()
	}
	p.instrument.Options = nil
	for i, name := range classicInstrumentNames {
		label := name + " (preview only)"
		if item, ok := bardOwnedInstrument(session, i); ok {
			label = item.Name + " (inventory)"
		}
		p.instrument.Options = append(p.instrument.Options, label)
	}
	tune := p.tune()
	none := tune == nil
	p.instrument.Disabled = none
	p.edit.Disabled = none
	p.previewButton.Disabled = none
	p.playButton.Disabled = true
	p.stopPreview.Disabled = p.preview == nil
	p.stopButton.Disabled = p.performance == nil
	if none {
		p.details.SetWrappedText("Select a tune or create one.")
	} else {
		p.details.SetWrappedText(tune.Name)
		p.instrument.Selected = tune.Instrument
		_, owned := bardOwnedInstrument(session, tune.Instrument)
		p.playButton.Disabled = !p.connected || !owned
		tip := "Equip the instrument and perform the saved tune in the selected session."
		if !p.connected {
			tip = "Connect a character to perform. Preview is local."
		} else if !owned {
			tip = "Take this instrument out of its case and into inventory to perform."
		}
		p.playButton.SetTooltip(tip)
	}
	p.layout()
}
func (p *bardPanel) selectedText() (string, error) {
	tune := p.tune()
	if tune == nil {
		return "", fmt.Errorf("Select a tune first.")
	}
	return readBardTune(tune.Path)
}
func (p *bardPanel) editTune() {
	tune := p.tune()
	if tune == nil {
		return
	}
	if _, err := readBardTune(tune.Path); err != nil {
		p.setError(err)
		return
	}
	path := tune.Path
	ed := openTextFileEditor(path, sourceEditorOptions{kind: "Tune", highlight: highlightBardTune, check: func(value string) error {
		index := bardSavedInstrument(path)
		_, err := validateBardTune(value, index)
		return err
	}, afterSave: func() {
		if bardWindow != nil {
			bardWindow.reload()
		}
	}})
	if ed != nil {
		// Draft preview belongs to the same tool and never saves or sends commands.
		found := false
		for _, action := range ed.actions {
			if action.Text == "Preview" {
				found = true
			}
		}
		if !found {
			button := eui.NewActionButton("Preview", func() {
				index := bardSavedInstrument(path)
				if bardWindow == nil {
					showBardWindow()
				}
				bardWindow.selected = path
				bardWindow.refreshList()
				bardWindow.previewText(ed.input.Text, index)
			})
			button.Size = eui.Point{X: 80, Y: 24}
			ed.actions = append(ed.actions, button, eui.NewActionButton("Stop Preview", func() {
				if bardWindow != nil {
					bardWindow.preview.stop()
					bardWindow.preview = nil
					bardWindow.refreshSelection()
				}
			}))
			ed.layout()
		}
	}
}
func (p *bardPanel) previewText(value string, index int) {
	p.preview.stop()
	p.preview = nil
	preview, err := startBardPreview(value, index, func(err error) { p.preview = nil; p.setError(err); p.refreshSelection() })
	p.preview = preview
	p.setError(err)
	p.refreshSelection()
}
func (p *bardPanel) startPreview() {
	value, err := p.selectedText()
	if err == nil {
		p.previewText(value, p.tune().Instrument)
	} else {
		p.setError(err)
	}
}
func (p *bardPanel) play() {
	value, err := p.selectedText()
	if err != nil {
		p.setError(err)
		return
	}
	// Validate completely before stopping a previous performance or equipping.
	if _, err = validateBardTune(value, p.tune().Instrument); err != nil {
		p.setError(err)
		return
	}
	if _, err = bardTuneCommands(value); err != nil {
		p.setError(err)
		return
	}
	p.preview.stop()
	p.preview = nil
	p.performance.stop()
	p.performance = nil
	p.performance, err = startBardPerformance(selectedAppSession(), value, p.tune().Instrument)
	p.setError(err)
	p.refreshSelection()
}
func (p *bardPanel) importTune() {
	path, err := pickBardTuneFile()
	if err != nil {
		p.setError(err)
		return
	}
	if path == "" {
		return
	}
	value, err := readBardTune(path)
	if err != nil {
		p.setError(err)
		return
	}
	tune, err := createBardTune(filepath.Base(path), value)
	if err != nil {
		p.setError(err)
		return
	}
	p.selected = tune.Path
	p.reload()
	p.editTune()
}
func (p *bardPanel) newTune() {
	win := eui.NewWindow()
	win.Title = "New Tune"
	win.Closable, win.Movable, win.AutoSize = true, true, true
	win.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)
	win.OnClose = win.RemoveWindow
	name, _ := eui.NewInput()
	name.Label = "Tune name"
	name.Size = eui.Point{X: 360, Y: 28}
	status := eui.NewWrappedLabel("", 360)
	create := eui.NewActionButton("Create & Edit", func() {
		tune, err := createBardTune(name.Text, "")
		if err != nil {
			status.SetWrappedText(err.Error())
			win.Refresh()
			return
		}
		p.selected = tune.Path
		p.reload()
		win.Close()
		p.editTune()
	})
	win.AddItem(eui.NewColumn(name, status, eui.NewRow(create, eui.NewActionButton("Cancel", win.Close))))
	win.DefaultButton = create
	win.AddWindow(false)
	win.MarkOpen()
	eui.Focus(name)
}
func updateBardWindow() {
	p := bardWindow
	if p == nil {
		return
	}
	session := selectedAppSession()
	if p.performance != nil {
		if p.performance.session != session {
			p.performance.stop()
			p.performance = nil
		} else if err := p.performance.update(time.Now()); err != nil {
			p.performance = nil
			p.setError(err)
			p.refreshSelection()
		}
	}
	if p.performance != nil && !p.performance.finishAt.IsZero() && time.Now().After(p.performance.finishAt) {
		p.performance = nil
		p.refreshSelection()
	}
	connected := session != nil && session.transport.connected()
	if session != p.session || connected != p.connected || (session != nil && session.inventory.revision.Load() != p.revision) {
		if session != p.session {
			p.preview.stop()
			p.preview = nil
		}
		p.refreshSelection()
	}
}
