package main

import "gothoom/eui"

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
		input = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
		input.Color = eui.ColorVeryDarkGray
		flow.AddItem(input)
	}

	win.AddWindow(false)
	return win, list, input
}

// updateTextWindow refreshes a text window's content and optional input message.
func updateTextWindow(win *eui.WindowData, list, input *eui.ItemData, msgs []string, fontSize float64, inputMsg string) {
	if list == nil {
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

	// Compute a row height that matches the rendered text height at the
	// current UI scale to avoid clipping.
	ui := eui.UIScale()
	rowUnits := (float32(fontSize)*ui + 4) / ui

	for i, msg := range msgs {
		if i < len(list.Contents) {
			if list.Contents[i].Text != msg || list.Contents[i].FontSize != float32(fontSize) {
				list.Contents[i].Text = msg
				list.Contents[i].FontSize = float32(fontSize)
			}
			list.Contents[i].Size.Y = rowUnits
		} else {
			t, _ := eui.NewText()
			t.Text = msg
			t.FontSize = float32(fontSize)
			t.Size = eui.Point{X: 1000, Y: rowUnits}
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

	if input != nil {
		input.Size.X = clientWAvail
		input.Size.Y = rowUnits
		if len(input.Contents) == 0 {
			t, _ := eui.NewText()
			t.Text = inputMsg
			t.FontSize = float32(fontSize)
			t.Size = eui.Point{X: clientWAvail, Y: rowUnits}
			input.AddItem(t)
		} else {
			if input.Contents[0].Text != inputMsg || input.Contents[0].FontSize != float32(fontSize) {
				input.Contents[0].Text = inputMsg
				input.Contents[0].FontSize = float32(fontSize)
			}
			input.Contents[0].Size.X = clientWAvail
			input.Contents[0].Size.Y = rowUnits
		}
	}

	if win != nil {
		// Size the flow to the client area, and the list to fill above the input.
		if list.Parent != nil {
			list.Parent.Size.X = clientWAvail
			list.Parent.Size.Y = clientHAvail
		}
		list.Size.X = clientWAvail
		if input != nil {
			list.Size.Y = clientHAvail - input.Size.Y
		} else {
			list.Size.Y = clientHAvail
		}
		win.Refresh()
	}
}
