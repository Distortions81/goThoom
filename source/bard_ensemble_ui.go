package main

import (
	"fmt"
	"strings"
	"time"

	"gothoom/eui"
	scriptapi "gt2"
)

func (p *bardPanel) showEnsembleWindow() {
	share := &p.sharing
	if share.win != nil {
		share.win.BringForward()
		return
	}
	win := eui.NewWindow()
	share.win = win
	win.Title = "Duet / Trio"
	win.Closable, win.Movable, win.Resizable, win.NoScroll = true, true, true, true
	win.Size = eui.Point{X: 520, Y: 500}
	win.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)
	root := eui.NewColumn()
	body := eui.NewColumn()
	body.Fixed, body.Scrollable = true, true
	root.AddItem(body)
	openedSession := p.session
	var openedGeneration uint64
	if openedSession != nil {
		openedGeneration = bardConnectionGeneration(openedSession)
	}
	body.AddItem(eui.NewWrappedLabel("Choose one partner for a duet or two for a trio. Each performer lists the other players in Play with.", 480))
	if tune := p.tune(); tune != nil {
		body.AddItem(eui.NewWrappedLabel("Song: "+tune.Name, 480))
	}
	body.AddItem(p.part)
	p.partners.Invisible, p.partnerRow.Invisible = false, false
	body.AddItem(p.partnerRow)
	if share.receive == nil {
		box, handler := eui.NewCheckbox()
		share.receive = box
		box.Text = "Receive parts from partners"
		box.Size = eui.Point{X: 460, Y: 26}
		handler.Handle = func(event eui.UIEvent) {
			if event.Type == eui.EventCheckboxChanged {
				p.setReceiveParts(event.Checked)
			}
		}
		box.SetTooltip("On by default. Save parts only from full partner names in Play with: up to 8 KiB per part and 16 saved parts before receiving pauses. Reconnecting or closing Bard turns it off. You still choose Play in Game.")
		p.setReceiveParts(true)
	}
	body.AddItem(share.receive)
	body.AddItem(eui.NewWrappedLabel("To receive parts, enter full partner names or use Choose players. Incoming parts are saved as new tunes; playback stays under your control.", 480))
	assignments := eui.NewColumn()
	body.AddItem(assignments)
	var previousAssignments []int
	for _, choice := range share.assignments {
		previousAssignments = append(previousAssignments, choice.Selected)
	}
	share.assignments = nil
	for i := 0; i < 2; i++ {
		choice, _ := eui.NewDropdown()
		choice.Label = fmt.Sprintf("Part for partner %d", i+1)
		choice.Size = eui.Point{X: 460, Y: 28}
		share.assignments = append(share.assignments, choice)
		assignments.AddItem(choice)
	}
	// Assignments name parts from this saved score; sending rechecks the file.
	tune := p.tune()
	path, value := "", ""
	var score bardScore
	if tune != nil {
		path = tune.Path
		var err error
		value, err = readBardTune(path)
		if err == nil {
			score, err = parseBardScore(value, bardLegacyInstrument(path))
		}
		if err != nil {
			p.setError(err)
		}
	}
	for i, choice := range share.assignments {
		choice.Options = []string{"Don't send"}
		for _, part := range score.Parts {
			choice.Options = append(choice.Options, part.Name+" · "+classicInstrumentNames[part.Instrument])
		}
		var others []int
		for n, part := range score.Parts {
			if part.Name != p.selectedPart {
				others = append(others, n+1)
			}
		}
		if i < len(others) {
			choice.Selected = others[i]
		}
		if path == share.assignmentPath && value == share.assignmentValue && i < len(previousAssignments) {
			choice.Selected = previousAssignments[i]
		}
	}
	share.assignmentPath, share.assignmentValue = path, value
	share.send = eui.NewActionButton("Send Parts", func() {
		session := p.session
		if !p.win.IsOpen() || share.win != win || !win.IsOpen() || session == nil || session != openedSession || !session.transport.connectedGeneration(openedGeneration) {
			p.setError(fmt.Errorf("The character or connection changed. Close and reopen Duet / Trio to send parts."))
			return
		}
		if p.selected != path {
			p.setError(fmt.Errorf("The selected song changed. Close and reopen Duet / Trio to assign its parts."))
			return
		}
		current, err := readBardTune(path)
		if err != nil {
			p.setError(err)
			return
		}
		if current != value {
			p.setError(fmt.Errorf("The saved song changed. Close and reopen Duet / Trio to assign its parts."))
			return
		}
		names, err := bardSharingPartners(p.partners.Text, session.characterName())
		if err != nil {
			p.setError(err)
			return
		}
		var commands []string
		for i, name := range names {
			selected := share.assignments[i].Selected
			if selected == 0 {
				continue
			}
			if selected < 1 || selected > len(score.Parts) {
				p.setError(fmt.Errorf("Choose a part for %s.", name))
				return
			}
			title := score.Title
			if title == "" && tune != nil {
				title = tune.Name
			}
			messages, err := bardPartMessages(title, score.Parts[selected-1])
			if err != nil {
				p.setError(err)
				return
			}
			for _, message := range messages {
				commands = append(commands, "/thinkto "+bardPlayerKey(name)+" "+message)
			}
		}
		if len(commands) == 0 {
			p.setError(fmt.Errorf("Choose at least one part to send."))
			return
		}
		if len(share.tickets) > 0 {
			p.setError(fmt.Errorf("Wait for sending to finish, or cancel it first."))
			return
		}
		share.sendSession, share.sendGeneration = session, openedGeneration
		share.tickets = queueBardSessionCommands(session, share.sendGeneration, commands)
		if len(share.tickets) == 0 {
			p.setError(fmt.Errorf("The connection changed; try again."))
			return
		}
		p.setStatus(fmt.Sprintf("Sending music parts in %d private messages.", len(commands)), false)
		p.updateBardSharing()
	})
	share.cancelSend = eui.NewActionButton("Cancel Sending", func() {
		p.cancelPartSending()
		p.setStatus("Canceled unsent music messages.", false)
		p.updateBardSharing()
	})
	body.AddItem(eui.NewWrappedLabel("Send Parts uses private sunstone messages. Equip working sunstones. Other clients can copy the numbered segments in order, including their comments. goThoom can assemble them automatically.", 480))
	closeButton := eui.NewActionButton("Close", win.Close)
	for _, button := range []*eui.ItemData{share.send, share.cancelSend, closeButton} {
		button.Size.X = 1
	}
	root.AddItem(eui.NewRow(share.send, share.cancelSend, closeButton))
	share.status = eui.NewWrappedLabel(p.statusMessage, 480)
	root.AddItem(share.status)
	win.OnClose = func() {
		if p.partnerPicker != nil {
			p.partnerPicker.Close()
		}
		win.RemoveWindow()
		share.win = nil
		share.status = nil
	}
	win.AddItem(root)
	win.OnResize = func() {
		eui.LayoutWindowBody(win, root, body)
		width := root.Size.X - 12
		for _, item := range body.Contents {
			if item != p.partnerRow {
				item.Size.X = width
			}
		}
		for _, choice := range share.assignments {
			choice.Size.X = width
		}
		p.partners.Size.X = max(100, width-p.choosePartners.GetSize().X/eui.UIScale()-12)
		p.choosePartners.Position.Y = p.partners.GetSize().Y/eui.UIScale() - p.partners.Size.Y + p.partners.Position.Y
		p.partnerRow.Size.Y = 0
		eui.LayoutWindowBody(win, root, body)
		win.Refresh()
	}
	p.updateBardSharing()
	win.OnResize()
	win.MarkOpen()
}

func (p *bardPanel) cancelPartSending() {
	for _, ticket := range p.sharing.tickets {
		ticket.Cancel()
	}
	p.sharing.tickets = nil
}

func (p *bardPanel) updateBardSharing() {
	share := &p.sharing
	session := p.session
	for key, incoming := range share.incoming {
		if time.Now().After(incoming.expires) {
			delete(share.incoming, key)
			p.setStatus("A music part did not arrive completely. Ask your partner to resend it.", true)
		}
	}
	if share.receive != nil && share.receive.Checked && (share.session == nil || session != share.session || !share.session.transport.connectedGeneration(share.generation)) {
		p.setReceiveParts(false)
	}
	if len(share.tickets) > 0 {
		if session != share.sendSession || !share.sendSession.transport.connectedGeneration(share.sendGeneration) {
			p.cancelPartSending()
			p.setStatus("Part sending canceled because the character or connection changed.", true)
		} else {
			pending, failed := false, false
			for _, ticket := range share.tickets {
				state := ticket.Status().State
				pending = pending || state == scriptapi.CommandQueued
				failed = failed || state == scriptapi.CommandCancelled || state == scriptapi.CommandRejected
			}
			if failed {
				p.cancelPartSending()
				p.setStatus("Part sending was interrupted. Ask your partners which parts arrived.", true)
			} else if !pending {
				share.tickets = nil
				p.setStatus("Part messages sent. Partners with receiving enabled can select Play in Game.", false)
			}
		}
	}
	if share.win == nil {
		return
	}
	share.receive.Disabled = session == nil || !session.transport.connected()
	names := strings.Split(p.partners.Text, ",")
	for i, choice := range share.assignments {
		label := fmt.Sprintf("Part for partner %d", i+1)
		if i < len(names) && strings.TrimSpace(names[i]) != "" {
			label = "Part for " + strings.TrimSpace(names[i])
		}
		if choice.Label != label {
			choice.Label = label
			share.win.Refresh()
		}
		choice.Disabled = i >= len(names) || strings.TrimSpace(names[i]) == "" || len(share.tickets) > 0
	}
	share.send.Disabled = p.tune() == nil || session == nil || !session.transport.connected() || len(share.tickets) > 0
	share.cancelSend.Disabled = len(share.tickets) == 0
}
