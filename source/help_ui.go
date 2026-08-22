//go:build !test

package main

import (
	_ "embed"
	"math"
	"strings"

	"gothoom/eui"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

//go:embed data/help.txt
var helpText string

var helpWin *eui.WindowData
var helpList *eui.ItemData
var helpCols [3]*eui.ItemData
var helpLines []string

func initHelpUI() {
	if helpWin != nil {
		return
	}
	helpWin, helpList, _ = makeTextWindow("Help", eui.HZoneCenter, eui.VZoneMiddleTop, false)
	helpWin.Size = eui.Point{X: 1700, Y: 800}
	helpLines = strings.Split(strings.ReplaceAll(helpText, "\r\n", "\n"), "\n")
	helpList.FlowType = eui.FLOW_HORIZONTAL
	for i := 0; i < 3; i++ {
		col := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
		helpList.AddItem(col)
		helpCols[i] = col
	}
	helpWin.OnResize = func() { rebuildHelpColumns() }
}

func openHelpWindow(anchor *eui.ItemData) {
	if helpWin == nil {
		return
	}
	if anchor != nil {
		helpWin.MarkOpenNear(anchor)
	} else {
		helpWin.MarkOpen()
	}
	if helpWin.OnResize != nil {
		helpWin.OnResize()
	}
	helpWin.Refresh()
}

func toggleHelpWindow(anchor *eui.ItemData) {
	if helpWin == nil {
		return
	}
	if helpWin.IsOpen() {
		helpWin.Close()
		return
	}
	openHelpWindow(anchor)
}

// rebuildHelpColumns lays out help text into three columns sharing the same scroll.
func rebuildHelpColumns() {
	if helpWin == nil || helpList == nil || helpCols[0] == nil {
		return
	}
	clientW := helpWin.GetSize().X
	clientH := helpWin.GetSize().Y - helpWin.GetTitleSize()
	s := eui.UIScale()
	if helpWin.NoScale {
		s = 1
	}
	pad := (helpWin.Padding + helpWin.BorderPad) * s
	clientWAvail := clientW - 2*pad
	if clientWAvail < 0 {
		clientWAvail = 0
	}
	clientHAvail := clientH - 2*pad
	if clientHAvail < 0 {
		clientHAvail = 0
	}

	helpList.Scrollable = true
	colW := clientWAvail / 3
	for i := 0; i < 3; i++ {
		helpCols[i].Size.X = colW
		helpCols[i].Size.Y = clientHAvail
		helpCols[i].Scrollable = false
		helpCols[i].Contents = helpCols[i].Contents[:0]
	}

	// Font face for sizing
	ui := eui.UIScale()
	facePx := float64(float32(15)*ui) + 2
	var face text.Face
	if monoFaceSource != nil {
		face = &text.GoTextFace{Source: monoFaceSource, Size: facePx}
	} else if src := eui.FontSource(); src != nil {
		face = &text.GoTextFace{Source: src, Size: facePx}
	} else {
		face = &text.GoTextFace{Size: facePx}
	}
	metrics := face.Metrics()
	linePx := math.Ceil(metrics.HAscent + metrics.HDescent + 2)
	rowUnits := float32(linePx) / ui

	sb := eui.ScrollbarWidth()
	contentW := colW - sb
	if contentW < 0 {
		contentW = 0
	}
	wrapW := float64(contentW - 3*pad)

	type section struct {
		lines []string
		rows  int
	}
	var sections []section
	isDivider := func(s string) bool {
		s = strings.TrimSpace(s)
		if s == "" {
			return false
		}
		cnt := 0
		for _, r := range s {
			if r == '-' {
				cnt++
			} else {
				return false
			}
		}
		return cnt > 4
	}
	for i := 0; i < len(helpLines); {
		ln := helpLines[i]
		if i+1 < len(helpLines) && isDivider(helpLines[i+1]) {
			sec := []string{ln}
			i += 2
			for i < len(helpLines) {
				if i+1 < len(helpLines) && isDivider(helpLines[i+1]) {
					break
				}
				sec = append(sec, helpLines[i])
				i++
			}
			sections = append(sections, section{lines: sec})
			continue
		}
		var sec []string
		for i < len(helpLines) {
			if i+1 < len(helpLines) && isDivider(helpLines[i+1]) {
				break
			}
			sec = append(sec, helpLines[i])
			i++
		}
		if len(sec) > 0 {
			sections = append(sections, section{lines: sec})
		}
	}

	for i := range sections {
		rows := 0
		for _, l := range sections[i].lines {
			_, ls := wrapText(l, face, wrapW)
			if n := len(ls); n > 0 {
				rows += n
			} else {
				rows++
			}
		}
		sections[i].rows = rows
	}

	heights := [3]int{}
	capRows := int(float32(clientHAvail) / rowUnits)
	// helpers
	addLine := func(col int, txt string) int {
		_, ls := wrapText(txt, face, wrapW)
		wrapped := strings.Join(ls, "\n")
		// skip if divider (more than 4 dashes)
		tl := strings.TrimSpace(wrapped)
		dashCount := 0
		onlyDashes := true
		for _, r := range tl {
			if r == '-' {
				dashCount++
			} else {
				onlyDashes = false
				break
			}
		}
		if onlyDashes && dashCount > 4 {
			return 0
		}
		t, _ := eui.NewText()
		t.Text = wrapped
		t.FontSize = 15
		t.Face = face
		linesN := len(ls)
		if linesN < 1 {
			linesN = 1
		}
		t.Size = eui.Point{X: contentW, Y: rowUnits * float32(linesN)}
		helpCols[col].AddItem(t)
		return linesN
	}
	addSpacer := func(col int) {
		sp, _ := eui.NewText()
		sp.Text = ""
		sp.Size = eui.Point{X: contentW, Y: rowUnits * 0.5}
		helpCols[col].AddItem(sp)
		heights[col] += 1
	}

	for _, sec := range sections {
		// order columns by current fill (shortest first)
		order := []int{0, 1, 2}
		if heights[1] < heights[0] {
			order[0], order[1] = 1, 0
		}
		if heights[2] < heights[order[0]] {
			order[0], order[2] = 2, order[0]
		}
		if heights[2] < heights[order[1]] {
			order[1], order[2] = 2, order[1]
		}

		placed := false
		for _, idx := range order {
			remaining := capRows - heights[idx]
			if remaining >= sec.rows+1 { // fits with spacer
				for _, l := range sec.lines {
					used := addLine(idx, l)
					heights[idx] += used
				}
				addSpacer(idx)
				placed = true
				break
			}
		}
		if placed {
			continue
		}

		// split across columns if needed
		lines := append([]string(nil), sec.lines...)
		li := 0
		for _, idx := range order {
			remaining := capRows - heights[idx]
			for remaining > 1 && li < len(lines) {
				l := lines[li]
				_, ls := wrapText(l, face, wrapW)
				take := len(ls)
				if take < 1 {
					take = 1
				}
				if take > remaining-1 {
					take = remaining - 1
				}
				joined := strings.Join(ls[:take], "\n")
				used := addLine(idx, joined)
				heights[idx] += used
				remaining -= used
				if used < len(ls) {
					lines[li] = strings.Join(ls[used:], "\n")
				} else {
					li++
				}
			}
			addSpacer(idx)
			if li >= len(lines) {
				break
			}
		}
	}

	helpList.Size.X = clientWAvail
	helpList.Size.Y = clientHAvail
}
