package main

import "gothoom/eui"

// Above/below variants share a choice. Keep the persisted layout values intact.
var tiledLayoutChoices = []struct {
	layout TiledLayout
	title  string
}{
	{TiledLayoutCenter, "Game centered"},
	{TiledLayoutSide, "Game on a side"},
	{TiledLayoutMessagesBelow, "Messages above or below"},
	{TiledLayoutFullMessagesBelow, "Full-width message row"},
	{TiledLayoutMessagesSplit, "Messages above and below"},
}

func tiledMessagesAbove() bool {
	return gs.TiledLayout == TiledLayoutMessagesAbove || gs.TiledLayout == TiledLayoutFullMessagesAbove ||
		(gs.TiledLayout == TiledLayoutMessagesSplit && gs.MessagesToConsole && gs.TiledConsoleLeft)
}

func tiledLayoutChoiceIndex() int {
	layout := gs.TiledLayout
	switch layout {
	case TiledLayoutMessagesAbove:
		layout = TiledLayoutMessagesBelow
	case TiledLayoutFullMessagesAbove:
		layout = TiledLayoutFullMessagesBelow
	case TiledLayoutMessagesSplit:
		if gs.MessagesToConsole {
			layout = TiledLayoutMessagesBelow
		}
	}
	for i, choice := range tiledLayoutChoices {
		if choice.layout == layout {
			return i
		}
	}
	return 0
}

func tiledLayoutChoice(index int) TiledLayout {
	if index < 0 || index >= len(tiledLayoutChoices) {
		return gs.TiledLayout
	}
	layout := tiledLayoutChoices[index].layout
	if tiledMessagesAbove() {
		switch layout {
		case TiledLayoutMessagesBelow:
			layout = TiledLayoutMessagesAbove
		case TiledLayoutFullMessagesBelow:
			layout = TiledLayoutFullMessagesAbove
		}
	}
	return layout
}

// Shared by the layout window and setup wizard so availability, labels, and
// callbacks do not drift between the two entry points.
func newTiledArrangementControls(width float32, changed func()) *eui.ItemData {
	root := eui.NewColumn()
	apply := func() {
		applyTiledWorkspaceLayout()
		refreshTiledArrangementControls(root.Contents)
		if changed != nil {
			changed()
		}
	}
	add := func(name, label, tip string, set func(bool)) {
		cb, events := eui.NewCheckbox()
		cb.Name, cb.Text = name, label
		cb.Size = eui.Point{X: width, Y: 24}
		cb.SetTooltip(tip)
		events.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventCheckboxChanged && !cb.Disabled && !cb.Invisible {
				set(ev.Checked)
				apply()
			}
		}
		root.AddItem(cb)
	}
	add("tiled-swap-game", "Swap game side", "Move the game to the right edge, keeping the other panes together on the left.", func(v bool) { gs.TiledGameLeft = !v })
	add("tiled-swap-lists", "Swap inventory / players list", "Exchange the two lists without moving the game or messages.", func(v bool) { gs.TiledInventoryLeft = !v })
	add("tiled-swap-messages", "Swap console and chat", "Exchange Console and Chat within their row, stack, or separate areas.", func(v bool) { gs.TiledConsoleLeft = !v })
	add("tiled-messages-above", "Messages above game", "Move the message row above the game. List and message order stay the same.", func(v bool) {
		if tiledLayoutChoiceIndex() == 3 {
			gs.TiledLayout = TiledLayoutFullMessagesBelow
			if v {
				gs.TiledLayout = TiledLayoutFullMessagesAbove
			}
		} else {
			gs.TiledLayout = TiledLayoutMessagesBelow
			if v {
				gs.TiledLayout = TiledLayoutMessagesAbove
			}
		}
	})
	split, splitEvents := eui.NewDropdown()
	split.Name, split.Label = "tiled-message-split", "Message split"
	split.Options = []string{"Side by side", "Stacked"}
	split.Size = eui.Point{X: width, Y: 24}
	split.SetTooltip("Place separate Chat and Console beside each other or stack them in their shared area. Drag the divider to resize them.")
	splitEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected && !split.Disabled && !split.Invisible && ev.Index >= 0 && ev.Index <= 1 {
			gs.TiledMessagesStacked = ev.Index == 1
			apply()
		}
	}
	root.AddItem(split)
	refreshTiledArrangementControls(root.Contents)
	return root
}

func refreshTiledArrangementControls(items []*eui.ItemData) {
	for _, it := range items {
		disabled := false
		switch it.Name {
		case "tiled-swap-game":
			it.Checked, it.Invisible = !gs.TiledGameLeft, false
			disabled = gs.TiledLayout != TiledLayoutSide
			it.SetTooltip("Move the game to the right edge, keeping the other panes together on the left.")
			if gs.TiledLayout != TiledLayoutSide {
				it.SetTooltip("Choose Game on a side to move the game between the left and right edges.")
			}
		case "tiled-swap-lists":
			it.Checked = !gs.TiledInventoryLeft
		case "tiled-swap-messages":
			it.Checked, it.Invisible = !gs.TiledConsoleLeft, false
			disabled = gs.MessagesToConsole && gs.TiledLayout != TiledLayoutCenter
			it.Text = "Swap console and chat"
			it.SetTooltip("Exchange Console and Chat within their row, stack, or separate areas.")
			if gs.MessagesToConsole {
				it.Text = "Combined messages on right"
				it.SetTooltip("Move the combined Chat and Console pane to the right side of the centered game.")
				if gs.TiledLayout != TiledLayoutCenter {
					it.SetTooltip("Choose Game centered to place combined messages in either side panel.")
				}
			}
		case "tiled-messages-above":
			index := tiledLayoutChoiceIndex()
			it.Checked = tiledMessagesAbove()
			disabled = index != 2 && index != 3
			it.SetTooltip("Move the message row above the game. List and message order stay the same.")
			if disabled {
				it.SetTooltip("Choose Messages above or below or Full-width message row to move the row above the game.")
			}
		case "tiled-message-split":
			it.Selected = 0
			if gs.TiledMessagesStacked {
				it.Selected = 1
			}
			disabled = gs.MessagesToConsole || !tiledPairedMessages()
			it.SetTooltip("Place separate Chat and Console beside each other or stack them in their shared area. Drag the divider to resize them.")
			if gs.MessagesToConsole {
				it.SetTooltip("Turn off Combine chat + console to split the message area into separate panes.")
			} else if disabled {
				it.SetTooltip("Choose a layout where Chat and Console share a message area to change its split direction.")
			}
		default:
			refreshTiledArrangementControls(it.Contents)
			continue
		}
		it.Invisible, it.Disabled = false, disabled
		it.Dirty = true
	}
}
