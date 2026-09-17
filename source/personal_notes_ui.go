package main

import (
	"fmt"
	"sort"
	"strings"

	"gothoom/eui"
)

type personalNotesPanel struct {
	win                                        *eui.WindowData
	root, list, player, scope, status, filters *eui.ItemData
	query                                      string
	notes                                      []*personalNote
}

var personalNotes *personalNotesPanel
var personalNoteDetailsWindows = map[string]*eui.WindowData{}

func showPersonalNotes() {
	if isWASM {
		consoleMessage("[notes] Personal notes are available in the desktop client.")
		return
	}
	if personalNotes != nil {
		personalNotes.reload()
		personalNotes.win.MarkOpen()
		personalNotes.win.BringForward()
		return
	}
	panel := &personalNotesPanel{win: eui.NewWindow(), root: eui.NewColumn()}
	personalNotes = panel
	win := panel.win
	win.Title = "Personal Notes"
	win.Closable, win.Movable, win.Resizable, win.NoScroll = true, true, true, true
	win.Size = eui.Point{X: 660, Y: 500}
	win.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)
	win.OnClose = func() {
		win.RemoveWindow()
		if personalNotes == panel {
			personalNotes = nil
		}
	}
	win.Searchable = true
	win.OnSearch = func(query string) { panel.query = query; panel.refreshList() }
	newButton := eui.NewActionButton("New Note", func() { openPersonalNoteDetails(nil, panel.selectedPlayer()) })
	setMaterialButtonIcon(newButton, "add")
	panel.root.AddItem(eui.NewRow(newButton, eui.NewActionButton("Refresh", panel.reload)))
	panel.player, _ = eui.NewDropdown()
	panel.player.Label = "Player"
	panel.player.Size = eui.Point{X: 250, Y: 28}
	panel.player.Handler.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventDropdownSelected {
			panel.refreshList()
		}
	}
	panel.scope, _ = eui.NewDropdown()
	panel.scope.Label = "Show"
	panel.scope.Options = []string{"Global + Player", "Global only", "Player only", "All notes"}
	panel.scope.Size = eui.Point{X: 200, Y: 28}
	panel.scope.Handler.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventDropdownSelected {
			panel.refreshList()
		}
	}
	panel.filters = eui.NewRow(panel.player, panel.scope)
	panel.root.AddItem(panel.filters)
	panel.root.AddItem(eui.NewWrappedLabel("Search subjects and tags with the titlebar magnifier.", 600))
	panel.list = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true, Fixed: true}
	panel.root.AddItem(panel.list)
	panel.status = eui.NewWrappedLabel("", 600)
	panel.root.AddItem(panel.status)
	win.AddItem(panel.root)
	win.OnResize = panel.refreshList
	win.AddWindow(false)
	panel.reload()
	win.MarkOpen()
}

func (panel *personalNotesPanel) selectedPlayer() string {
	if panel.player.Selected > 0 && panel.player.Selected < len(panel.player.Options) {
		return panel.player.Options[panel.player.Selected]
	}
	return ""
}

func (panel *personalNotesPanel) reload() {
	selected := panel.selectedPlayer()
	if selected == "" {
		selected = strings.TrimSpace(scriptManagerCharacter(selectedAppSession()))
	}
	if selected == "" {
		selected = strings.TrimSpace(gs.LastCharacter)
	}
	notes, err := listPersonalNotes()
	panel.notes = notes
	names := map[string]string{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name != "" {
			names[strings.ToLower(name)] = name
		}
	}
	add(selected)
	for _, character := range characters {
		add(character.Name)
	}
	for _, note := range notes {
		add(note.Player)
	}
	options := []string{}
	for _, name := range names {
		options = append(options, name)
	}
	sort.Slice(options, func(i, j int) bool { return strings.ToLower(options[i]) < strings.ToLower(options[j]) })
	panel.player.Options = append([]string{"No player selected"}, options...)
	panel.player.Selected = 0
	for i, name := range panel.player.Options {
		if i > 0 && strings.EqualFold(name, selected) {
			panel.player.Selected = i
			break
		}
	}
	panel.status.SetWrappedText("")
	if err != nil {
		panel.status.SetWrappedText("Some notes could not be loaded. Details are in Console.")
		consoleMessage("[notes] " + err.Error())
	}
	panel.refreshList()
}

func (panel *personalNotesPanel) refreshList() {
	if panel.list == nil {
		return
	}
	eui.LayoutWindowBody(panel.win, panel.root, panel.list)
	panel.filters.FlowType = eui.FLOW_HORIZONTAL
	if panel.root.Size.X < 520 {
		panel.filters.FlowType = eui.FLOW_VERTICAL
	}
	panel.filters.Size.Y = 0
	eui.LayoutWindowBody(panel.win, panel.root, panel.list)
	width := savedDataContentWidth(panel.list.Size.X)
	panel.status.Size.X = width
	panel.list.SetItems(nil)
	scope := "Global + Player"
	if panel.scope.Selected >= 0 && panel.scope.Selected < len(panel.scope.Options) {
		scope = panel.scope.Options[panel.scope.Selected]
	}
	count := 0
	for _, note := range panel.notes {
		if !personalNoteMatches(note, panel.query, panel.selectedPlayer(), scope) {
			continue
		}
		row := eui.NewColumn()
		row.Size.X = width
		row.Filled, row.Color = count%2 == 1, eui.SubtleAlternateRowColor()
		row.AddItem(eui.NewWrappedLabel(note.Subject, width))
		summary := "Global"
		if note.Player != "" {
			summary = "Player: " + note.Player
		}
		if len(note.Tags) > 0 {
			summary += "   ·   " + strings.Join(note.Tags, ", ")
		}
		info := eui.NewWrappedLabel(summary, width)
		info.FontSize = 11
		row.AddItem(info)
		open := eui.NewActionButton("Open", func() { openPersonalNote(note) })
		details := eui.NewActionButton("Details", func() { openPersonalNoteDetails(note, panel.selectedPlayer()) })
		trash := eui.NewActionButton("Trash", func() {
			eui.ShowPopup("Move note to Trash", "Move “"+note.Subject+"” to Notes/Trash?", []eui.PopupButton{
				{Text: "Cancel"}, {Text: "Move to Trash", Action: func() {
					if err := trashPersonalNote(note); err != nil {
						panel.status.SetWrappedText(err.Error())
						panel.win.Refresh()
						return
					}
					panel.reload()
				}},
			})
		})
		for _, button := range []*eui.ItemData{open, details, trash} {
			button.Size = eui.Point{X: 80, Y: 24}
			button.FontSize = 11
		}
		row.AddItem(eui.NewRow(open, details, trash))
		panel.list.AddItem(row)
		count++
	}
	if count == 0 {
		panel.list.AddItem(eui.NewWrappedLabel("No matching notes. Create a note or change the player, scope, or search.", width))
	}
	panel.win.Refresh()
}

func openPersonalNoteDetails(note *personalNote, player string) *eui.WindowData {
	key := "new"
	if note != nil {
		key = note.id
	}
	if existing := personalNoteDetailsWindows[key]; existing != nil {
		existing.MarkOpen()
		existing.BringForward()
		return existing
	}
	win := eui.NewWindow()
	personalNoteDetailsWindows[key] = win
	win.Title = "New Note"
	win.Closable, win.Movable, win.AutoSize = true, true, true
	win.Resizable = false
	win.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)
	win.OnClose = func() { win.RemoveWindow(); delete(personalNoteDetailsWindows, key) }
	root := eui.NewColumn()
	subject, _ := eui.NewInput()
	subject.Label = "Subject"
	subject.Size = eui.Point{X: 420, Y: 28}
	tags, _ := eui.NewInput()
	tags.Label = "Tags (comma-separated)"
	tags.Size = subject.Size
	global, _ := eui.NewCheckbox()
	global.Text = "Global note"
	global.Checked = true
	global.Size = eui.Point{X: 420, Y: 26}
	owner, _ := eui.NewInput()
	owner.Label = "Player"
	owner.Text = player
	owner.Size = subject.Size
	owner.Disabled = true
	global.SetTooltip("Global notes are available with every character. Uncheck to assign this note to one player.")
	if note != nil {
		win.Title = "Note Details"
		subject.Text, tags.Text, owner.Text = note.Subject, strings.Join(note.Tags, ", "), note.Player
		global.Checked, owner.Disabled = note.Player == "", note.Player == ""
	}
	global.Handler.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventCheckboxChanged {
			global.Checked = event.Checked
			owner.Disabled = event.Checked
			win.Refresh()
		}
	}
	for _, item := range []*eui.ItemData{subject, tags, global, owner} {
		root.AddItem(item)
	}
	status := eui.NewWrappedLabel("", 420)
	root.AddItem(status)
	label := "Create & Open"
	if note != nil {
		label = "Save Details"
	}
	save := eui.NewActionButton(label, func() {
		player := ""
		if !global.Checked {
			player = strings.TrimSpace(owner.Text)
			if player == "" {
				status.SetWrappedText("Enter a player name or choose Global note.")
				win.Refresh()
				return
			}
		}
		var err error
		created := note == nil
		if created {
			note, err = createPersonalNote(subject.Text, tags.Text, player)
		} else {
			err = savePersonalNoteDetails(note, subject.Text, tags.Text, player)
		}
		if err != nil {
			status.SetWrappedText(fmt.Sprintf("Not saved: %v", err))
			win.Refresh()
			return
		}
		if personalNotes != nil {
			personalNotes.reload()
		}
		win.Close()
		if created {
			openPersonalNote(note)
		}
	})
	root.AddItem(eui.NewRow(save, eui.NewActionButton("Cancel", win.Close)))
	win.AddItem(root)
	win.DefaultButton = save
	win.AddWindow(false)
	win.MarkOpen()
	eui.Focus(subject)
	return win
}
