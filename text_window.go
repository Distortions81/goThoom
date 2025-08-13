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

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	flow.Size = win.GetSize()
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

	for i, msg := range msgs {
		if i < len(list.Contents) {
			if list.Contents[i].Text != msg || list.Contents[i].FontSize != float32(fontSize) {
				list.Contents[i].Text = msg
				list.Contents[i].FontSize = float32(fontSize)
			}
		} else {
			t, _ := eui.NewText()
			t.Text = msg
			t.Size = eui.Point{X: 1000, Y: 24}
			list.PrependItem(t)
		}
	}
	if len(list.Contents) > len(msgs) {
		for i := len(msgs); i < len(list.Contents); i++ {
			list.Contents[i] = nil
		}
		list.Contents = list.Contents[:len(msgs)]
	}

	if input != nil {
		input.Size.Y = float32(fontSize) + 8
		if len(input.Contents) == 0 {
			t, _ := eui.NewText()
			t.Text = inputMsg
			t.FontSize = float32(fontSize)
			t.Size = eui.Point{X: 1000, Y: 24}
			input.AddItem(t)
		} else if input.Contents[0].Text != inputMsg || input.Contents[0].FontSize != float32(fontSize) {
			input.Contents[0].Text = inputMsg
			input.Contents[0].FontSize = float32(fontSize)
		}
	}

	if win != nil {
		list.Size.X = win.GetSize().X
		if input != nil {
			list.Size.Y = win.GetSize().Y - input.Size.Y
		} else {
			list.Size.Y = win.GetSize().Y
		}
		win.Refresh()
	}
}
