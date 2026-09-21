package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	sharing                                                                                           bardSharing
	ensembleButton                                                                                    *eui.ItemData
	storage                                                                                           *bardCaseOperation
	storeButton                                                                                       *eui.ItemData
	moreButton, moreActions, deleteButton                                                             *eui.ItemData
	groups                                                                                            []bardActionGroup
	win                                                                                               *eui.WindowData
	root, list, details, instrument, status, edit, previewButton, playButton, stopButton, stopPreview *eui.ItemData
	part, previewPart, partners, sortOrder, tag                                                       *eui.ItemData
	statusFrame                                                                                       *eui.ItemData
	playConfirm                                                                                       *eui.WindowData
	partnerPicker                                                                                     *eui.WindowData
	partnerRow, choosePartners                                                                        *eui.ItemData
	statusMessage                                                                                     string
	statusText                                                                                        string
	statusProblem                                                                                     bool
	selectedPart                                                                                      string
	tunes                                                                                             []bardTune
	selected                                                                                          string
	query                                                                                             string
	session                                                                                           *Session
	revision                                                                                          uint64
	generation                                                                                        uint64
	connected                                                                                         bool
	preview                                                                                           *bardPreview
	performance                                                                                       *bardPerformance
}

var bardWindow *bardPanel
var bardPanels = make(map[*Session]*bardPanel)

func showBardWindow() {
	if isWASM {
		consoleMessage("[bard] Tunes are available in the desktop client.")
		return
	}
	session := selectedAppSession()
	if bardWindow != nil && bardWindow.session != session {
		bardWindow.hideForSessionSwitch()
		bardWindow = nil
	}
	if p := bardPanels[session]; p != nil {
		bardWindow = p
		message, problem := p.statusMessage, p.statusProblem
		bardWindow.reload()
		p.setStatus(message, problem)
		bardWindow.win.MarkOpen()
		bardWindow.win.BringForward()
		return
	}
	p := &bardPanel{win: eui.NewWindow(), root: eui.NewColumn(), session: session}
	bardWindow = p
	bardPanels[session] = p
	win := p.win
	win.Title = "Bard"
	win.Closable, win.Movable, win.Resizable, win.NoScroll = true, true, true, true
	win.Size = eui.Point{X: 640, Y: 700}
	win.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)
	win.OnClose = closeBardTools
	win.Searchable = true
	win.OnSearch = func(query string) { p.query = query; p.refreshList() }
	newButton := eui.NewActionButton("New", func() { p.newTune() })
	setMaterialButtonIcon(newButton, "add")
	helpButton := eui.NewActionButton("Help", showBardHelp)
	setMaterialButtonIcon(helpButton, "help")
	helpButton.SetTooltip("Read the Bard Tools guide: tunes, instruments, previews, performing, sharing parts, and notation.")
	p.edit = eui.NewActionButton("Edit", p.editTune)
	setMaterialButtonIcon(p.edit, "edit")
	p.moreButton = eui.NewActionButton("More…", func() {
		p.moreActions.Invisible = !p.moreActions.Invisible
		p.moreButton.Text = "More…"
		if !p.moreActions.Invisible {
			p.moreButton.Text = "Less"
		}
		p.layout()
	})
	p.moreButton.SetTooltip("Show or hide file actions and instrument storage.")
	p.addActions(newButton, eui.NewActionButton("Import", p.importTune), p.edit, p.moreButton, helpButton)
	p.deleteButton = eui.NewActionButton("Delete Song…", func() {
		if tune := p.tune(); tune != nil {
			p.confirmDeleteTune(*tune)
		}
	})
	setMaterialButtonIcon(p.deleteButton, "delete")
	p.deleteButton.SetTooltip("Permanently delete the selected song after confirmation.")
	p.storeButton = eui.NewActionButton("Put All Instruments Away", p.storeInstruments)
	p.addActions(eui.NewActionButton("Open Folder", func() { p.setError(open.Run(bardTunesDir())) }), eui.NewActionButton("Refresh", p.reload), p.deleteButton, p.storeButton)
	p.moreActions = p.groups[len(p.groups)-1].column
	p.moreActions.Invisible = true
	p.sortOrder, _ = eui.NewDropdown()
	p.sortOrder.Label = "Songs — sort by"
	p.sortOrder.Options = []string{"Title", "Composer", "Tags", "Part count"}
	p.sortOrder.Size = eui.Point{X: 160, Y: 28}
	p.sortOrder.Handler.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventDropdownSelected {
			p.refreshList()
		}
	}
	p.tag, _ = eui.NewDropdown()
	p.tag.Label = "Tag"
	p.tag.Options = []string{"All tags"}
	p.tag.Size = eui.Point{X: 180, Y: 28}
	p.tag.Handler.Handle = p.sortOrder.Handler.Handle
	p.addActions(p.sortOrder, p.tag)
	p.list = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true, Fixed: true}
	p.list.Outlined, p.list.Border, p.list.OutlineColor = true, 1, eui.ColorGray
	p.list.Position.Y = 4
	p.root.AddItem(p.list)
	p.details = eui.NewWrappedLabel("Select a song above to preview, edit, or play it.", 580)
	p.root.AddItem(p.details)
	p.part, _ = eui.NewDropdown()
	p.part.Label = "Your part"
	p.part.Size = eui.Point{X: 360, Y: 28}
	p.part.Handler.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventDropdownSelected {
			p.selectPart(p.part.Selected)
		}
	}
	p.instrument, _ = eui.NewDropdown()
	p.instrument.Label = "Instrument (saved to song)"
	p.instrument.SetTooltip("Changing this instrument immediately saves it in the song shared by all your characters. Save or close an unsaved editor draft first.")
	p.instrument.Size = eui.Point{X: 360, Y: 28}
	p.instrument.Handler.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventDropdownSelected {
			if tune := p.tune(); tune != nil {
				index := p.instrument.Selected
				if err := saveBardPartInstrument(*tune, p.part.Selected, index); err != nil {
					p.setError(err)
					p.refreshSelection()
					return
				}
				p.preview.stop()
				p.preview = nil
				p.reload()
				p.setStatus(fmt.Sprintf("Saved %s for %s in “%s”.", classicInstrumentNames[index], p.selectedPart, tune.Name), false)
			}
		}
	}
	p.addActions(p.part, p.instrument)
	p.previewButton = eui.NewActionButton("Preview", p.startPreview)
	p.previewButton.SetTooltip("Listen locally to every part together, using each part's instrument.")
	p.previewPart = eui.NewActionButton("Preview Part…", p.startPartPreview)
	p.previewPart.SetTooltip("Choose any part to listen to locally, without changing your ensemble assignment.")
	p.stopPreview = eui.NewActionButton("Stop Preview", p.endPreview)
	listenLabel := eui.NewLabel("Listen:")
	listenLabel.Position.Y = 7
	p.addActions(listenLabel, p.previewButton, p.previewPart, p.stopPreview)
	p.partners, _ = eui.NewInput()
	p.partners.Label = "Play with"
	p.partners.Size = eui.Point{X: 360, Y: 28}
	p.partners.CompleteText = func(value string) string { return bardPartnerCompletion(p.session, value) }
	p.partners.Handler.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventInputChanged {
			p.refreshSelection()
		}
	}
	p.partners.SetTooltip("Type a name and press Tab to accept the gray suggestion. Separate performers with commas. Each performer chooses their own part and names the others. Leave blank to play your part alone.")
	p.choosePartners = eui.NewActionButton("Choose players…", p.showPartnerPicker)
	p.choosePartners.SetTooltip("Choose up to two visible players, nearest first.")
	p.partnerRow = eui.NewRow(p.partners, p.choosePartners)
	p.ensembleButton = eui.NewActionButton("Duet / Trio…", p.showEnsembleWindow)
	p.ensembleButton.SetTooltip("Choose partners, assign and send music parts, and enable receiving from partners.")
	p.playButton = eui.NewActionButton("Play in Game", p.play)
	p.playButton.SetTooltip("Review the song, character, and instrument before confirming in-game playback.")
	p.stopButton = eui.NewActionButton("Stop Playing", func() {
		p.storage.cancel()
		p.storage = nil
		p.performance.stop()
		p.performance = nil
		p.setStatus("Stopped in-game playback.", false)
		p.refreshSelection()
	})
	performLabel := eui.NewLabel("Perform:")
	performLabel.Position.Y = 7
	p.addActions(performLabel, p.playButton, p.stopButton, p.ensembleButton)
	p.statusFrame, p.status = newStatusBar(580)
	p.root.AddItem(p.statusFrame)
	win.AddItem(p.root)
	win.OnResize = p.refreshList
	win.AddWindow(false)
	p.reload()
	win.MarkOpen()
}
func (p *bardPanel) selectPart(index int) {
	if tune := p.tune(); tune != nil && index >= 0 && index < len(tune.Score.Parts) {
		p.selectedPart = tune.Score.Parts[index].Name
		p.preview.stop()
		p.preview = nil
		p.setError(nil)
		p.refreshSelection()
	}
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
	if err != nil {
		p.setStatus(err.Error(), true)
		return
	}
	p.statusMessage, p.statusProblem = "", false
	p.refreshStatus()
}
func (p *bardPanel) setStatus(message string, problem bool) {
	p.statusMessage, p.statusProblem = message, problem
	p.refreshStatus()
}
func (p *bardPanel) refreshStatus() {
	message, problem := p.statusMessage, p.statusProblem
	good := message != "" && !problem
	if message == "" {
		switch {
		case p.storage != nil:
			message, good = "Putting instruments away. Stop Playing cancels remaining transfers.", true
		case p.performance != nil:
			good = true
			switch {
			case !p.performance.submitted:
				message = "Preparing the instrument for in-game playback."
			case p.performance.ensemble && !p.performance.startedAt.IsZero():
				message = "Playing with the ensemble. Stop Playing ends your performance."
			case p.performance.ensemble && !p.performance.musicSent:
				message = "Sending the selected part to the game."
			case p.performance.ensemble:
				message = "Waiting for the ensemble to start. Each performer must choose Play in Game."
			case p.performance.finishAt.IsZero():
				message = "Sending the selected part to the game."
			default:
				message = "Playing in game. Stop Playing ends the performance."
			}
		case p.preview != nil:
			message, good = "Previewing locally.", true
		case p.tune() == nil:
			message = "Select a song above."
		case p.tune().Err != nil:
			message, problem = p.tune().Err.Error(), true
		default:
			message = "Ready to preview the selected song."
		}
	}
	key := fmt.Sprintf("%t:%t:%s", good, problem, message)
	if p.statusText == key {
		return
	}
	p.statusText = key
	if p.sharing.status != nil {
		p.sharing.status.SetWrappedText(message)
	}
	setStatusBar(p.statusFrame, p.status, message, good, problem)
	p.win.Refresh()
}
func (p *bardPanel) reload() {
	tunes, err := listBardTunes()
	if err != nil {
		p.setError(err)
		return
	}
	p.tunes = tunes
	p.refreshTags()
	p.setError(nil)
	p.refreshList()
	p.refreshSelection()
}
func (p *bardPanel) addActions(buttons ...*eui.ItemData) {
	for _, button := range buttons {
		if button.ItemType == eui.ITEM_BUTTON {
			button.Size.X = 1 // Fit captions so the song list keeps room in narrow windows.
		}
	}
	column := eui.NewColumn()
	p.root.AddItem(column)
	p.groups = append(p.groups, bardActionGroup{column, buttons})
}
func (p *bardPanel) layout() {
	eui.LayoutWindowBody(p.win, p.root, p.list)
	// Keep the part and its saved instrument together when the window is wide
	// enough for both captions, and stack them in narrow windows.
	width := p.root.Size.X - 8
	p.part.Size.X, p.instrument.Size.X = width, width
	if !p.part.Invisible && width >= 560 {
		p.part.Size.X = (width - 8) / 2
		p.instrument.Size.X = (width - 8) / 2
	}
	for _, group := range p.groups {
		row := eui.NewRow()
		rows := []*eui.ItemData{}
		used := float32(0)
		for _, button := range group.buttons {
			if button.Invisible {
				continue
			}
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
	width = savedDataContentWidth(p.list.Size.X)
	p.details.Size.X = width
	p.status.Size.X = p.statusFrame.Size.X - 16
	eui.LayoutWindowBody(p.win, p.root, p.list)
	p.win.Refresh()
}
func (p *bardPanel) refreshList() {
	p.layout()
	p.list.SetItems(nil)
	width := savedDataContentWidth(p.list.Size.X)
	for _, tune := range p.filteredTunes() {
		tune := tune
		label := tune.Name + "  ·  " + classicInstrumentNames[tune.Instrument]
		if len(tune.Score.Parts) > 1 {
			label = fmt.Sprintf("%s  ·  %d parts", tune.Name, len(tune.Score.Parts))
		}
		button, events := eui.NewRadio()
		button.Text = label
		button.RadioGroup = "bard-song"
		button.Checked = tune.Path == p.selected
		button.SetTooltip("Select " + tune.Name + " to preview, edit, or play it.")
		events.Handle = func(event eui.UIEvent) {
			if event.Type != eui.EventRadioSelected {
				return
			}
			if p.selected != tune.Path {
				p.selectedPart = ""
			}
			p.selected = tune.Path
			p.setError(nil)
			p.refreshSelection()
			p.refreshList()
		}
		button.Size = eui.Point{X: width, Y: 30}
		button.ConstrainToSize = true
		row := eui.NewRow(button)
		row.Position = eui.Point{X: 4, Y: 4}
		row.Filled = button.Checked
		row.Color = eui.SubtleAlternateRowColor()
		p.list.AddItem(row)
	}
	if len(p.list.Contents) == 0 {
		message := "No tunes found. Create a tune or import a CL tune text file."
		if p.query != "" || p.tag.Selected > 0 {
			message = "No matching tunes. Change the search or tag filter."
		}
		p.list.AddItem(eui.NewWrappedLabel(message, width))
	}
	p.win.Refresh()
}

func (p *bardPanel) refreshTags() {
	selected := ""
	if p.tag.Selected > 0 && p.tag.Selected < len(p.tag.Options) {
		selected = p.tag.Options[p.tag.Selected]
	}
	tags := make(map[string]string)
	for _, tune := range p.tunes {
		for _, tag := range tune.Score.Tags {
			tags[strings.ToLower(tag)] = tag
		}
	}
	options := make([]string, 0, len(tags))
	for _, tag := range tags {
		options = append(options, tag)
	}
	sort.Slice(options, func(i, j int) bool { return strings.ToLower(options[i]) < strings.ToLower(options[j]) })
	p.tag.Options, p.tag.Selected = append([]string{"All tags"}, options...), 0
	for i, option := range p.tag.Options[1:] {
		if strings.EqualFold(option, selected) {
			p.tag.Selected = i + 1
		}
	}
}

func (p *bardPanel) filteredTunes() []bardTune {
	var tunes []bardTune
	for _, tune := range p.tunes {
		search := tune.Name + " " + tune.Score.Composer + " " + strings.Join(tune.Score.Tags, " ")
		for _, part := range tune.Score.Parts {
			search += " " + part.Name + " " + classicInstrumentNames[part.Instrument]
		}
		if !strings.Contains(strings.ToLower(search), strings.ToLower(p.query)) {
			continue
		}
		if p.tag.Selected > 0 {
			match := false
			for _, tag := range tune.Score.Tags {
				match = match || strings.EqualFold(tag, p.tag.Options[p.tag.Selected])
			}
			if !match {
				continue
			}
		}
		tunes = append(tunes, tune)
	}
	sort.SliceStable(tunes, func(i, j int) bool {
		a, b := tunes[i], tunes[j]
		key := func(tune bardTune) string {
			switch p.sortOrder.Selected {
			case 1:
				return strings.ToLower(tune.Score.Composer)
			case 2:
				return strings.ToLower(strings.Join(tune.Score.Tags, ", "))
			}
			return strings.ToLower(tune.Name)
		}
		if p.sortOrder.Selected == 3 && len(a.Score.Parts) != len(b.Score.Parts) {
			return len(a.Score.Parts) < len(b.Score.Parts)
		}
		if key(a) != key(b) {
			return key(a) < key(b)
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	return tunes
}
func (p *bardPanel) confirmDeleteTune(tune bardTune) *eui.WindowData {
	return eui.ShowPopup("Delete Tune", "Delete “"+tune.Name+"” from Tunes? This cannot be undone. Any open draft of this tune will also be discarded.", []eui.PopupButton{
		{Text: "Cancel"},
		{Text: "Delete", Color: &eui.ColorDarkRed, HoverColor: &eui.ColorRed, Action: func() {
			if err := os.Remove(tune.Path); err != nil {
				p.setError(err)
				return
			}
			path, _ := filepath.Abs(tune.Path)
			if ed := sourceEditors[path]; ed != nil {
				ed.discard = true
				ed.win.Close()
			}
			if p.selected == tune.Path {
				p.selected = ""
				p.preview.stop()
				p.preview = nil
				p.performance.stop()
				p.performance = nil
			}
			err := os.Remove(tune.Path + ".json")
			p.reload()
			if err != nil && !os.IsNotExist(err) {
				p.setError(fmt.Errorf("Tune deleted, but its instrument preference could not be removed: %w", err))
			}
		}},
	})
}
func (p *bardPanel) refreshSelection() {
	session := p.session
	title := "Bard"
	if session != nil && session.characterName() != "" {
		title += " — " + session.characterName()
	}
	p.win.Title = title
	p.connected = session != nil && session.transport.connected()
	if session != nil {
		p.revision = session.inventory.revision.Load()
		p.generation = bardConnectionGeneration(session)
	}
	_, hasCase := bardInstrumentCase(session)
	hasInstruments := false
	if session != nil {
		for _, item := range session.inventory.snapshot() {
			hasInstruments = hasInstruments || bardItemInstrument(item) >= 0
		}
	}
	p.storeButton.Disabled = !p.connected || !hasCase || !hasInstruments || p.storage != nil
	storeTip := "Stop playing and return all carried instruments to your instrument case."
	switch {
	case !p.connected:
		storeTip = "Connect a character to put instruments away."
	case !hasCase:
		storeTip = "Carry an instrument case to put instruments away."
	case p.storage != nil:
		storeTip = "Instruments are being put away. Stop Playing cancels remaining transfers."
	case !hasInstruments:
		storeTip = "No carried instruments to put away."
	}
	p.storeButton.SetTooltip(storeTip)
	p.instrument.Options = nil
	for i, name := range classicInstrumentNames {
		label := name + " (preview only)"
		if hasCase {
			label = name + " (try case)"
		}
		if item, ok := bardOwnedInstrument(session, i); ok {
			label = item.Name + " (inventory)"
		}
		p.instrument.Options = append(p.instrument.Options, label)
	}
	tune := p.tune()
	none := tune == nil
	p.instrument.Disabled = none
	p.edit.Disabled = none
	p.deleteButton.Disabled = none
	p.previewButton.Disabled = none
	p.playButton.Disabled = true
	p.stopPreview.Disabled = p.preview == nil
	p.stopButton.Disabled = p.performance == nil && p.storage == nil
	p.part.Options = nil
	p.part.Invisible, p.previewPart.Invisible = true, true
	p.instrument.Label = "Instrument (saved to song)"
	p.previewButton.Text = "Preview"
	if none {
		p.details.SetWrappedText("Select a song above to preview, edit, or play it.")
	} else {
		details := "Selected song: " + tune.Name
		if tune.Score.Composer != "" {
			details += " — " + tune.Score.Composer
		}
		if len(tune.Score.Tags) > 0 {
			details += "\nTags: " + strings.Join(tune.Score.Tags, ", ")
		}
		valid := tune.Err == nil && len(tune.Score.Parts) > 0
		p.instrument.Disabled, p.previewButton.Disabled = !valid, !valid
		index := tune.Instrument
		p.part.Selected = 0
		for i, part := range tune.Score.Parts {
			p.part.Options = append(p.part.Options, part.Name+" · "+classicInstrumentNames[part.Instrument])
			if part.Name == p.selectedPart {
				p.part.Selected = i
			}
		}
		if valid {
			part := tune.Score.Parts[p.part.Selected]
			p.selectedPart, index = part.Name, part.Instrument
		}
		if len(tune.Score.Parts) > 1 && valid {
			p.part.Invisible, p.previewPart.Invisible = false, false
			p.instrument.Label = "Part instrument (saved to song)"
			p.previewButton.Text = "Preview All"
		}
		if partners := strings.TrimSpace(p.partners.Text); partners != "" {
			details += "\nPlay with: " + partners
		}
		p.details.SetWrappedText(details)
		p.instrument.Selected = index
		_, owned := bardOwnedInstrument(session, index)
		p.playButton.Disabled = !valid || !p.connected || (!owned && !hasCase) || p.storage != nil
		tip := "Review the song, character, and instrument before confirming in-game playback."
		if !valid {
			tip = "Edit the tune to fix its metadata, then save it."
		} else if !p.connected {
			tip = "Connect a character to perform. Preview is local."
		} else if !owned && !hasCase {
			tip = "Carry this instrument or an instrument case to perform."
		} else if p.storage != nil {
			tip = "Wait for instruments to be put away, or cancel with Stop Playing."
		}
		p.playButton.SetTooltip(tip)
	}
	p.refreshStatus()
	p.layout()
	if part := p.sharing.part; part != nil {
		part.Options = append([]string(nil), p.part.Options...)
		part.Selected, part.Invisible = p.part.Selected, p.part.Invisible
	}
	if p.sharing.win != nil && p.sharing.win.OnResize != nil {
		p.refreshEnsembleAnalysis()
		p.sharing.win.OnResize()
	}
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
	ed := openTextFileEditor(path, sourceEditorOptions{kind: "Tune", persistentStatus: true, highlight: highlightBardTune, check: func(value string) error {
		score, err := parseBardScore(value, bardLegacyInstrument(path))
		if err != nil {
			return err
		}
		partners := ""
		if panel := bardPanels[selectedAppSession()]; panel != nil {
			partners = panel.partners.Text
		}
		return validateBardEnsembleScore(score, partners)
	}, lint: func(value string) []string {
		score, err := parseBardScore(value, bardLegacyInstrument(path))
		if err != nil {
			return nil
		}
		_, warnings := bardTimingSummary(score)
		return warnings
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
				index := bardLegacyInstrument(path)
				if bardWindow == nil {
					showBardWindow()
				}
				bardWindow.selected = path
				bardWindow.refreshList()
				bardWindow.previewText(ed.input.Text, index)
			})
			button.Size = eui.Point{X: 80, Y: 24}
			button.SetTooltip("Preview every part together using this unsaved draft.")
			ed.actions = append(ed.actions, button, eui.NewActionButton("Stop Preview", func() {
				if bardWindow != nil {
					bardWindow.endPreview()
				}
			}))
			ed.layout()
		}
	}
}
func (p *bardPanel) previewText(value string, index int) {
	score, err := parseBardScore(value, index)
	if err != nil {
		p.setError(err)
		return
	}
	p.previewScore(score, -1)
}
func (p *bardPanel) previewScore(score bardScore, part int) {
	p.preview.stop()
	p.preview = nil
	preview, err := startBardScorePreview(score, part, func(err error) {
		p.preview = nil
		if err != nil {
			p.setError(err)
		} else {
			p.setStatus("Preview finished.", false)
		}
		p.refreshSelection()
	})
	p.preview = preview
	p.setError(err)
	p.refreshSelection()
}
func (p *bardPanel) endPreview() {
	p.preview.stop()
	p.preview = nil
	p.setStatus("Preview stopped.", false)
	p.refreshSelection()
}
func (p *bardPanel) startPreview() {
	value, err := p.selectedText()
	if err == nil {
		p.previewText(value, bardLegacyInstrument(p.selected))
	} else {
		p.setError(err)
	}
}
func (p *bardPanel) startPartPreview() {
	value, err := p.selectedText()
	if err != nil {
		p.setError(err)
		return
	}
	score, err := parseBardScore(value, bardLegacyInstrument(p.selected))
	if err != nil {
		p.setError(err)
		return
	}
	options := make([]string, len(score.Parts))
	for i, part := range score.Parts {
		options[i] = part.Name + " · " + classicInstrumentNames[part.Instrument]
		if duration, err := bardPartDuration(part); err == nil {
			options[i] += fmt.Sprintf(" (%.2fs)", duration.Seconds())
		}
	}
	r := p.previewPart.DrawRect
	eui.ShowContextMenu(options, r.X0, r.Y1, func(index int) {
		p.previewScore(score, index)
	})
}
func (p *bardPanel) play() {
	value, err := p.selectedText()
	if err != nil {
		p.setError(err)
		return
	}
	score, err := parseBardScore(value, bardLegacyInstrument(p.selected))
	if err != nil {
		p.setError(err)
		return
	}
	// Validate completely before stopping a previous performance or equipping.
	if _, err = validateBardScore(score); err != nil {
		p.setError(err)
		return
	}
	index, err := bardSelectedPart(score, p.selectedPart)
	if err != nil {
		p.setError(err)
		return
	}
	part := score.Parts[index]
	var partners []string
	if strings.TrimSpace(p.partners.Text) != "" {
		partners = strings.Split(p.partners.Text, ",")
	}
	if _, err = bardEnsembleCommands(part.Text, partners); err != nil {
		p.setError(err)
		return
	}
	session := p.session
	if session == nil || !session.transport.connected() {
		p.setError(fmt.Errorf("Connect a character to perform. Preview is local."))
		return
	}
	if !bardCanPrepareInstrument(session, part.Instrument) {
		p.setError(fmt.Errorf("Carry %s or an instrument case to perform.", classicInstrumentNames[part.Instrument]))
		return
	}
	p.showPlayConfirmation(bardPlayRequest{
		session: session, generation: bardConnectionGeneration(session), character: session.characterName(),
		path: p.selected, value: value, title: p.tune().Name, part: part, partners: partners,
	})
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
	tune, err := importBardTune(path)
	if err != nil {
		p.setError(err)
		return
	}
	p.selected = tune.Path
	p.selectedPart = ""
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
		value := bardMetadata("title", strings.TrimSpace(name.Text)) + "\n<@instrument: Lucky Lyra>\n<@tags:>\n\n<Write your tune below.>\n"
		tune, err := createBardTune(name.Text, value)
		if err != nil {
			status.SetWrappedText(err.Error())
			win.Refresh()
			return
		}
		p.selected = tune.Path
		p.selectedPart = ""
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
func (p *bardPanel) storeInstruments() {
	if p.storage != nil {
		return
	}
	session := p.session
	if session == nil || !session.transport.connected() {
		p.setError(fmt.Errorf("Connect a character to put instruments away."))
		return
	}
	if _, ok := bardInstrumentCase(session); !ok {
		p.setError(fmt.Errorf("Carry an instrument case to put instruments away."))
		return
	}
	p.performance.stop()
	p.performance = nil
	var err error
	p.storage, err = startBardCaseOperation(session, -1)
	p.setError(err)
	p.refreshSelection()
}

// The visible panel follows the selected tab; hidden panels keep their own
// selections, transfers, and performances running on their original session.
func (p *bardPanel) hideForSessionSwitch() {
	if p.win.IsOpen() {
		// Close transient menus and release drawing resources without ending the
		// session's work. The panel can reopen when this tab is selected again.
		onClose := p.win.OnClose
		p.win.OnClose = nil
		p.win.Close()
		p.win.OnClose = onClose
	}
	if p.playConfirm != nil {
		p.playConfirm.Close()
	}
	if p.partnerPicker != nil {
		p.partnerPicker.Close()
	}
	if p.sharing.win != nil {
		p.sharing.win.Close()
	}
	p.preview.stop()
	p.preview = nil
}

func (p *bardPanel) registered() bool {
	return p != nil && bardPanels[p.session] == p
}

func (p *bardPanel) closeSessionPanel() {
	delete(bardPanels, p.session)
	p.hideForSessionSwitch()
	p.cancelPartSending()
	p.storage.cancel()
	p.performance.stop()
	p.storage, p.performance = nil, nil
	p.win.RemoveWindow()
}

func closeBardTools() {
	for _, p := range bardPanels {
		p.closeSessionPanel()
	}
	bardWindow = nil
}

func updateBardWindow() {
	if bardWindow == nil {
		return
	}
	// Removed tabs cannot keep work alive or transfer it to a reused slot.
	for session, p := range bardPanels {
		current, present := appSessions.session(session.ID())
		if !present || current != session {
			p.closeSessionPanel()
		}
	}
	if !bardWindow.registered() || bardWindow.session != selectedAppSession() {
		previous := bardWindow.win
		showBardWindow()
		bardWindow.win.Size, bardWindow.win.Position = previous.Size, previous.Position
		bardWindow.refreshList()
	}
	for _, p := range bardPanels {
		p.updateSession()
	}
}

func (p *bardPanel) updateSession() {
	p.updateBardSharing()
	session := p.session
	if p.storage != nil {
		if err := p.storage.update(time.Now()); err != nil {
			p.storage = nil
			p.setError(err)
		} else if p.storage.done {
			p.storage = nil
			p.setStatus("All carried instruments put away.", false)
		}
		if p.storage == nil {
			p.refreshSelection()
		}
	}
	if p.performance != nil {
		if err := p.performance.update(time.Now()); err != nil {
			p.performance.clearMusicWatch()
			p.performance = nil
			p.setError(err)
			p.refreshSelection()
		}
	}
	if p.performance != nil && !p.performance.finishAt.IsZero() && time.Now().After(p.performance.finishAt) {
		message := p.performance.endedMessage
		if message == "" {
			message = "In-game performance finished."
		}
		p.performance.clearMusicWatch()
		p.performance = nil
		p.setStatus(message, false)
		p.refreshSelection()
	}
	connected := session != nil && session.transport.connected()
	if connected != p.connected || (session != nil && (session.inventory.revision.Load() != p.revision || bardConnectionGeneration(session) != p.generation)) {
		p.refreshSelection()
	}
	p.refreshStatus()
}
