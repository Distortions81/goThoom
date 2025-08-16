package main

import (
	"math"
	"strings"

	"gothoom/eui"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

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

	// Compute a row height from actual font metrics (ascent+descent) to
	// avoid clipping at large sizes. Convert pixels to item units.
	ui := eui.UIScale()
	facePx := float64(float32(fontSize) * ui)
	var goFace *text.GoTextFace
	if src := eui.FontSource(); src != nil {
		goFace = &text.GoTextFace{Source: src, Size: facePx}
	} else {
		goFace = &text.GoTextFace{Size: facePx}
	}
	metrics := goFace.Metrics()
	linePx := math.Ceil(metrics.HAscent + metrics.HDescent + 2) // +2 px padding
	rowUnits := float32(linePx) / ui

	// Prepare wrapping parameters: use the same face for measurement.
	var face text.Face = goFace
	// list.Size.X is in item units; convert to pixels for measurement.
	wrapWidthPx := float64(list.Size.X * ui)

	for i, msg := range msgs {
		// Word-wrap the message to the available width.
		_, lines := wrapText(msg, face, wrapWidthPx)
		wrapped := strings.Join(lines, "\n")
		linesN := len(lines)
		if linesN < 1 {
			linesN = 1
		}
		if i < len(list.Contents) {
			if list.Contents[i].Text != wrapped || list.Contents[i].FontSize != float32(fontSize) {
				list.Contents[i].Text = wrapped
				list.Contents[i].FontSize = float32(fontSize)
			}
			list.Contents[i].Size.Y = rowUnits * float32(linesN)
			list.Contents[i].Size.X = clientWAvail
		} else {
			t, _ := eui.NewText()
			t.Text = wrapped
			t.FontSize = float32(fontSize)
			t.Size = eui.Point{X: clientWAvail, Y: rowUnits * float32(linesN)}
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
		// Soft-wrap the input message to the available width and grow the input area.
		_, inLines := wrapText(inputMsg, face, wrapWidthPx)
		wrappedIn := strings.Join(inLines, "\n")
		inLinesN := len(inLines)
		if inLinesN < 1 {
			inLinesN = 1
		}
		input.Size.X = clientWAvail
		input.Size.Y = rowUnits * float32(inLinesN)
		if len(input.Contents) == 0 {
			t, _ := eui.NewText()
			t.Text = wrappedIn
			t.FontSize = float32(fontSize)
			t.Size = eui.Point{X: clientWAvail, Y: rowUnits * float32(inLinesN)}
			input.AddItem(t)
		} else {
			if input.Contents[0].Text != wrappedIn || input.Contents[0].FontSize != float32(fontSize) {
				input.Contents[0].Text = wrappedIn
				input.Contents[0].FontSize = float32(fontSize)
			}
			input.Contents[0].Size.X = clientWAvail
			input.Contents[0].Size.Y = rowUnits * float32(inLinesN)
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
		// Do not refresh here unconditionally; callers decide when to refresh.
	}
}
