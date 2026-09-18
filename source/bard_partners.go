package main

import (
	"fmt"
	"sort"
	"strings"

	"gothoom/eui"
)

type bardVisiblePlayer struct {
	name            string
	distanceSquared int64
}

func bardVisiblePlayers(session *Session) []bardVisiblePlayer {
	if session == nil || session.draw == nil || !session.transport.connected() {
		return nil
	}
	selfIndex, selfName := session.playerIndexSnapshot(), session.characterName()
	session.draw.mu.Lock()
	defer session.draw.mu.Unlock()
	self, ok := session.draw.current.mobiles[selfIndex]
	if !ok || self.Persist {
		return nil
	}
	var players []bardVisiblePlayer
	seen := map[string]bool{}
	for index, mobile := range session.draw.current.mobiles {
		desc, ok := session.draw.current.descriptors[index]
		if index == selfIndex || !ok || desc.Type != kDescPlayer || desc.Name == "" ||
			strings.EqualFold(desc.Name, selfName) || !mobileActuallyVisible(mobile, desc) {
			continue
		}
		if _, err := bardWithOptions([]string{desc.Name}); err != nil {
			continue
		}
		key := strings.ToLower(desc.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		dx, dy := int64(mobile.H)-int64(self.H), int64(mobile.V)-int64(self.V)
		players = append(players, bardVisiblePlayer{desc.Name, dx*dx + dy*dy})
	}
	sort.Slice(players, func(i, j int) bool {
		if players[i].distanceSquared != players[j].distanceSquared {
			return players[i].distanceSquared < players[j].distanceSquared
		}
		return strings.ToLower(players[i].name) < strings.ToLower(players[j].name)
	})
	return players
}

func (p *bardPanel) showPartnerPicker() {
	if p.partnerPicker != nil {
		p.partnerPicker.BringForward()
		return
	}
	session := selectedAppSession()
	if session == nil {
		p.setError(fmt.Errorf("Select a character first."))
		return
	}
	generation, character, original := bardConnectionGeneration(session), session.characterName(), p.partners.Text
	win := eui.NewWindow()
	p.partnerPicker = win
	win.Title = "Choose Players to Play With"
	win.Closable, win.Movable, win.Resizable, win.NoScroll = true, true, true, true
	win.Size = eui.Point{X: 420, Y: 360}
	win.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)
	win.OnClose = func() {
		win.RemoveWindow()
		if p.partnerPicker == win {
			p.partnerPicker = nil
		}
	}
	root := eui.NewColumn()
	root.AddItem(eui.NewWrappedLabel("Visible players, nearest first. Choose up to two partners.", 390))
	list := eui.NewColumn()
	list.Fixed, list.Scrollable, list.Outlined = true, true, true
	list.Border, list.OutlineColor = 1, eui.ColorGray
	root.AddItem(list)
	var boxes []*eui.ItemData
	var names []string
	selected := map[string]bool{}
	for _, name := range strings.Split(original, ",") {
		if name = strings.TrimSpace(name); name != "" {
			selected[strings.ToLower(name)] = true
		}
	}
	count := eui.NewLabel("")
	root.AddItem(count)
	var update func()
	add := func(name, caption string, checked bool) {
		box, events := eui.NewCheckbox()
		box.Text, box.Checked = caption, checked
		box.Size = eui.Point{X: 380, Y: 26}
		box.Position = eui.Point{X: 5, Y: 4}
		box.ConstrainToSize = true
		box.SetTooltip(caption)
		events.Handle = func(event eui.UIEvent) {
			if event.Type == eui.EventCheckboxChanged {
				update()
			}
		}
		boxes, names = append(boxes, box), append(names, name)
		list.AddItem(box)
	}
	visible := bardVisiblePlayers(session)
	for _, player := range visible {
		key := strings.ToLower(player.name)
		add(player.name, player.name, selected[key])
		delete(selected, key)
	}
	// Keep existing typed names, including prefixes and absent players, until
	// the user explicitly unchecks them. Cancel never changes the field.
	for _, name := range strings.Split(original, ",") {
		name = strings.TrimSpace(name)
		key := strings.ToLower(name)
		if selected[key] {
			add(name, name+" (entered name; not listed above)", true)
			delete(selected, key)
		}
	}
	if len(visible) == 0 {
		list.AddItem(eui.NewWrappedLabel("No other players are visible. You can also enter names in Play with.", 380))
	}
	ok := eui.NewActionButton("OK", func() {
		if !p.win.IsOpen() || selectedAppSession() != session ||
			bardConnectionGeneration(session) != generation || session.characterName() != character || p.partners.Text != original {
			p.setError(fmt.Errorf("The character or partner names changed. Choose players again."))
			win.Close()
			return
		}
		var chosen []string
		for i, box := range boxes {
			if box.Checked {
				chosen = append(chosen, names[i])
			}
		}
		if len(chosen) > 2 {
			return
		}
		p.partners.Text = strings.Join(chosen, ", ")
		p.partners.Dirty = true
		p.setError(nil)
		win.Close()
	})
	root.AddItem(eui.NewRow(eui.NewActionButton("Cancel", win.Close), ok))
	update = func() {
		n := 0
		for _, box := range boxes {
			if box.Checked {
				n++
			}
		}
		count.Text = fmt.Sprintf("%d of 2 partners selected", n)
		for _, box := range boxes {
			box.Disabled = n >= 2 && !box.Checked
			box.Dirty = true
		}
		ok.Disabled = n > 2
		win.Refresh()
	}
	win.AddItem(root)
	win.OnResize = func() {
		eui.LayoutWindowBody(win, root, list)
		for _, box := range boxes {
			box.Size.X = savedDataContentWidth(list.Size.X)
		}
		win.Refresh()
	}
	update()
	win.OnResize()
	win.MarkOpen()
}
