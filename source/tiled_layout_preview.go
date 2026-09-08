package main

import (
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"gothoom/eui"
)

// Preview proportions are deliberately fixed so arrangements remain comparable.
// These cells describe topology only; live divider sizes are left untouched.
type tiledPreviewPane struct {
	name       string
	x, y, w, h float64
}

func tiledPreviewPanes(layout TiledLayout, combined, stacked, inventoryLeft, consoleFirst, gameLeft bool) []tiledPreviewPane {
	grids := [7][3]string{
		{"IGP", "IGP", "CGH"},
		{"GIP", "GIP", "GMM"},
		{"IGP", "IGP", "IMP"},
		{"IMP", "IGP", "IGP"},
		{"ICP", "IGP", "IHP"},
		{"IGP", "IGP", "MMM"},
		{"MMM", "IGP", "IGP"},
	}
	if layout < TiledLayoutCenter || int(layout) >= len(grids) {
		return nil
	}
	grid := grids[layout]
	widths := [3]float64{.25, .5, .25}
	if layout == TiledLayoutSide {
		widths = [3]float64{.5, .25, .25}
	}
	// Swap semantic roles before removing the Chat area in combined mode.
	for row, cells := range grid {
		b := []byte(cells)
		for col, cell := range b {
			if !inventoryLeft {
				if cell == 'I' {
					b[col] = 'P'
				}
				if cell == 'P' {
					b[col] = 'I'
				}
			}
			if !consoleFirst {
				if cell == 'C' {
					b[col] = 'H'
				}
				if cell == 'H' {
					b[col] = 'C'
				}
			}
		}
		grid[row] = string(b)
	}
	if combined {
		for row, cells := range grid {
			b := []byte(cells)
			for col, cell := range b {
				if cell == 'H' {
					if layout == TiledLayoutCenter {
						b[col] = grid[0][col]
					} else {
						b[col] = 'G'
					}
				}
			}
			grid[row] = string(b)
		}
	}
	type bounds struct{ x0, y0, x1, y1 float64 }
	areas := map[byte]bounds{}
	for row, cells := range grid {
		x := 0.0
		for col, cell := range []byte(cells) {
			next := x + widths[col]
			a, ok := areas[cell]
			if !ok {
				a = bounds{x, float64(row) / 3, next, float64(row+1) / 3}
			} else {
				a.x0 = math.Min(a.x0, x)
				a.x1 = max(a.x1, next)
				a.y0 = math.Min(a.y0, float64(row)/3)
				a.y1 = max(a.y1, float64(row+1)/3)
			}
			areas[cell] = a
			x = next
		}
	}
	var panes []tiledPreviewPane
	add := func(name string, x, y, w, h float64) {
		if layout == TiledLayoutSide && !gameLeft {
			if name == "Game" {
				x = .5
			} else {
				x -= .5
			}
		}
		panes = append(panes, tiledPreviewPane{name, x, y, w, h})
	}
	for _, cell := range []byte("GIPCHM") {
		a, ok := areas[cell]
		if !ok {
			continue
		}
		x, y, w, h := a.x0, a.y0, a.x1-a.x0, a.y1-a.y0
		name := map[byte]string{'G': "Game", 'I': "Inv", 'P': "Players", 'C': "Console", 'H': "Chat"}[cell]
		if cell == 'M' {
			if combined {
				add("Chat +\nConsole", x, y, w, h)
				continue
			}
			first, second := "Console", "Chat"
			if !consoleFirst {
				first, second = second, first
			}
			if stacked {
				add(first, x, y, w, h/2)
				add(second, x, y+h/2, w, h/2)
			} else {
				add(first, x, y, w/2, h)
				add(second, x+w/2, y, w/2, h)
			}
		} else {
			if cell == 'C' && combined {
				name = "Chat +\nConsole"
			}
			add(name, x, y, w, h)
		}
	}
	return panes
}

func drawTiledPreview(dst *ebiten.Image, layout TiledLayout, selected bool) {
	dst.Fill(color.RGBA{12, 26, 37, 255})
	width, height := float32(dst.Bounds().Dx()), float32(dst.Bounds().Dy())
	border := color.RGBA{76, 96, 110, 255}
	if selected {
		border = color.RGBA{100, 222, 193, 255}
	}
	vector.FillRect(dst, 0, 0, width, height, border, false)
	vector.FillRect(dst, 4, 4, width-8, height-8, color.RGBA{12, 26, 37, 255}, false)
	face := *mainFont.(*text.GoTextFace)
	face.Size = 18
	for _, p := range tiledPreviewPanes(layout, gs.MessagesToConsole, gs.TiledMessagesStacked, gs.TiledInventoryLeft, gs.TiledConsoleLeft, gs.TiledGameLeft) {
		x, y := 8+float32(p.x)*(width-16), 8+float32(p.y)*(height-16)
		w, h := float32(p.w)*(width-16)-4, float32(p.h)*(height-16)-4
		fill := color.RGBA{37, 57, 72, 255}
		switch p.name {
		case "Game":
			fill = color.RGBA{18, 68, 63, 255}
		case "Console", "Chat +\nConsole":
			fill = color.RGBA{66, 54, 34, 255}
		case "Chat":
			fill = color.RGBA{48, 46, 80, 255}
		}
		vector.FillRect(dst, x, y, w, h, fill, false)
		// Keep pane labels within their small schematic rectangles.
		labelFace := face
		for _, line := range strings.Split(p.name, "\n") {
			tw, _ := text.Measure(line, &labelFace, 0)
			if tw > float64(w-6) {
				labelFace.Size *= float64(w-6) / tw
			}
		}
		op := &text.DrawOptions{}
		op.PrimaryAlign = text.AlignCenter
		op.SecondaryAlign = text.AlignCenter
		op.LineSpacing = labelFace.Size
		op.GeoM.Translate(float64(x+w/2), float64(y+h/2))
		op.ColorScale.ScaleWithColor(color.RGBA{235, 246, 249, 255})
		text.Draw(dst, p.name, &labelFace, op)
	}
}

// Locate items through the current window so recreating it cannot leave stale
// callbacks or preview images attached to a previous window instance.
func refreshTiledLayoutPreviews() {
	for _, win := range []*eui.WindowData{tileLayoutWin, setupWizardWin} {
		if win != nil {
			refreshTiledPreviewItems(win.Contents)
		}
	}
}

func refreshTiledPreviewItems(items []*eui.ItemData) {
	refreshTiledArrangementControls(items)
	var visit func([]*eui.ItemData)
	visit = func(items []*eui.ItemData) {
		for _, it := range items {
			if it.Name == "tiled-layout-card" {
				it.Invisible = gs.MessagesToConsole && it.Selected == 4
				it.Dirty = true
			}
			if it.Name == "tiled-layout-preview" && it.Image != nil {
				layout := tiledLayoutChoice(it.Selected)
				it.Checked = it.Selected == tiledLayoutChoiceIndex()
				drawTiledPreview(it.Image, layout, it.Checked)
				it.Dirty = true
			}
			visit(it.Contents)
		}
	}
	visit(items)
}

func newTiledLayoutGallery(selectLayout func(TiledLayout), columns int) *eui.ItemData {
	const cardWidth float32 = 172
	gallery := eui.NewSection("Choose a layout", float32(columns)*cardWidth+float32(columns-1)*8)
	var row *eui.ItemData
	for index, choice := range tiledLayoutChoices {
		if index%columns == 0 {
			row = eui.NewRow()
			if index > 0 {
				row.Position.Y = 8
			}
			gallery.AddItem(row)
		}
		layout := tiledLayoutChoice(index)
		button, events := eui.NewButton()
		button.Name = "tiled-layout-preview"
		button.Selected = index
		button.Size = eui.Point{X: cardWidth, Y: 88}
		button.Image = ebiten.NewImage(332, 160)
		button.SmoothImage = true
		button.Checked = index == tiledLayoutChoiceIndex()
		drawTiledPreview(button.Image, layout, button.Checked)
		events.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick && !(gs.MessagesToConsole && choice.layout == TiledLayoutMessagesSplit) {
				selectLayout(tiledLayoutChoice(index))
			}
		}
		caption := eui.NewLabel(choice.title)
		caption.FontSize = 11
		caption.Size = eui.Point{X: cardWidth, Y: 32}
		card := eui.NewColumn(button, caption)
		card.Name, card.Selected = "tiled-layout-card", index
		card.Invisible = gs.MessagesToConsole && choice.layout == TiledLayoutMessagesSplit
		if index%columns != 0 {
			card.Position.X = 8
		}
		row.AddItem(card)
	}
	return gallery
}
