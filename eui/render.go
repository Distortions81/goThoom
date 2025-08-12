package eui

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const shadowAlphaDivisor = 16

type dropdownRender struct {
	item   *itemData
	offset point
	clip   rect
}

var pendingDropdowns []dropdownRender
var dumpDone bool

// Draw renders the UI to the provided screen image.
// Call this from your Ebiten Draw function.
func Draw(screen *ebiten.Image) {

	pendingDropdowns = pendingDropdowns[:0]

	for _, win := range windows {
		if !win.Open {
			continue
		}

		win.Draw(screen)
	}

	for _, dr := range pendingDropdowns {
		drawDropdownOptions(dr.item, dr.offset, dr.clip, screen)
	}

	drawFPS(screen)

	if DumpMode && !dumpDone {
		if err := DumpCachedImages(); err != nil {
			panic(err)
		}
		dumpDone = true
		os.Exit(0)
	}
	if TreeMode && !dumpDone {
		if err := DumpTree(); err != nil {
			panic(err)
		}
		dumpDone = true
		os.Exit(0)
	}
}

func (win *windowData) Draw(screen *ebiten.Image) {
	win.drawBG(screen)
	win.drawItems(screen)
	win.drawScrollbars(screen)
	titleArea := screen.SubImage(win.getTitleRect().getRectangle()).(*ebiten.Image)
	win.drawWinTitle(titleArea)
	windowArea := screen.SubImage(win.getWinRect().getRectangle()).(*ebiten.Image)
	win.drawBorder(windowArea)
	win.drawDebug(screen)
}

func (win *windowData) drawBG(screen *ebiten.Image) {
	if win.ShadowSize > 0 && win.ShadowColor.A > 0 {
		rr := roundRect{
			Size:     win.GetSize(),
			Position: win.getPosition(),
			Fillet:   win.Fillet,
			Filled:   true,
			Color:    win.ShadowColor,
		}
		drawDropShadow(screen, &rr, win.ShadowSize, win.ShadowColor)
	}
	r := rect{
		X0: win.getPosition().X + win.BorderPad,
		Y0: win.getPosition().Y + win.BorderPad,
		X1: win.getPosition().X + win.GetSize().X - win.BorderPad,
		Y1: win.getPosition().Y + win.GetSize().Y - win.BorderPad,
	}
	drawRoundRect(screen, &roundRect{
		Size:     point{X: r.X1 - r.X0, Y: r.Y1 - r.Y0},
		Position: point{X: r.X0, Y: r.Y0},
		Fillet:   win.Fillet,
		Filled:   true,
		Color:    win.Theme.Window.BGColor,
	})
}

func (win *windowData) drawWinTitle(screen *ebiten.Image) {
	// Window Title
	if win.TitleHeight > 0 {
		screen.Fill(win.Theme.Window.TitleBGColor)

		textSize := ((win.GetTitleSize()) / 2)
		face := textFace(textSize)

		skipTitleText := false
		textWidth, textHeight := text.Measure(win.Title, face, 0)
		if textWidth > float64(win.GetSize().X) ||
			textHeight > float64(win.GetTitleSize()) {
			skipTitleText = true
		}

		//Title text
		if !skipTitleText {
			loo := text.LayoutOptions{
				LineSpacing:    0, //No multi-line titles
				PrimaryAlign:   text.AlignStart,
				SecondaryAlign: text.AlignCenter,
			}
			tdop := ebiten.DrawImageOptions{}
			tdop.GeoM.Translate(float64(win.getPosition().X+((win.GetTitleSize())/4)),
				float64(win.getPosition().Y+((win.GetTitleSize())/2)))

			top := &text.DrawOptions{DrawImageOptions: tdop, LayoutOptions: loo}

			top.ColorScale.ScaleWithColor(win.Theme.Window.TitleTextColor)
			buf := strings.ReplaceAll(win.Title, "\n", "") //Remove newline
			buf = strings.ReplaceAll(buf, "\r", "")        //Remove return
			text.Draw(screen, buf, face, top)
		} else {
			textWidth = 0
		}

		//Close X
		var buttonsWidth float32 = 0
		if win.Closable {
			var xpad float32 = (win.GetTitleSize()) / 3.0
			color := win.Theme.Window.TitleColor
			// fill background for close area if configured
			if win.Theme.Window.CloseBGColor.A > 0 {
				r := win.xRect()
				closeArea := screen.SubImage(r.getRectangle()).(*ebiten.Image)
				closeArea.Fill(win.Theme.Window.CloseBGColor)
			}
			xThick := 1 * uiScale
			if win.HoverClose {
				color = win.Theme.Window.HoverTitleColor
				win.HoverClose = false
			}
			strokeLine(screen,
				win.getPosition().X+win.GetSize().X-(win.GetTitleSize())+xpad,
				win.getPosition().Y+xpad,

				win.getPosition().X+win.GetSize().X-xpad,
				win.getPosition().Y+(win.GetTitleSize())-xpad,
				xThick, color, true)
			strokeLine(screen,
				win.getPosition().X+win.GetSize().X-xpad,
				win.getPosition().Y+xpad,

				win.getPosition().X+win.GetSize().X-(win.GetTitleSize())+xpad,
				win.getPosition().Y+(win.GetTitleSize())-xpad,
				xThick, color, true)

			buttonsWidth += (win.GetTitleSize())
		}

		//Dragbar
		if win.Movable && win.ShowDragbar {
			var xThick float32 = 1
			xColor := win.Theme.Window.DragbarColor
			if win.HoverDragbar {
				xColor = win.Theme.Window.HoverTitleColor
				win.HoverDragbar = false
			}
			dpad := (win.GetTitleSize()) / 5
			spacing := win.DragbarSpacing
			if spacing <= 0 {
				spacing = 5
			}
			for x := textWidth + float64((win.GetTitleSize())/1.5); x < float64(win.GetSize().X-buttonsWidth); x = x + float64(uiScale*spacing) {
				strokeLine(screen,
					win.getPosition().X+float32(x), win.getPosition().Y+dpad,
					win.getPosition().X+float32(x), win.getPosition().Y+(win.GetTitleSize())-dpad,
					xThick, xColor, false)
			}
		}
	}
}

func (win *windowData) drawBorder(screen *ebiten.Image) {
	//Draw borders
	if win.Outlined && win.Border > 0 {
		FrameColor := win.Theme.Window.BorderColor
		if activeWindow == win {
			FrameColor = win.Theme.Window.ActiveColor
		} else if win.Hovered {
			FrameColor = win.Theme.Window.HoverColor
			win.Hovered = false
		}
		drawRoundRect(screen, &roundRect{
			Size:     win.GetSize(),
			Position: win.getPosition(),
			Fillet:   win.Fillet,
			Filled:   false,
			Border:   win.Border,
			Color:    FrameColor,
		})
	}
}

func (win *windowData) drawScrollbars(screen *ebiten.Image) {
	if win.NoScroll {
		return
	}
	pad := (win.Padding + win.BorderPad) * uiScale
	req := win.contentBounds()
	avail := point{
		X: win.GetSize().X - 2*pad,
		Y: win.GetSize().Y - win.GetTitleSize() - 2*pad,
	}
	if req.Y > avail.Y {
		barH := avail.Y * avail.Y / req.Y
		maxScroll := req.Y - avail.Y
		pos := float32(0)
		if maxScroll > 0 {
			pos = (win.Scroll.Y / maxScroll) * (avail.Y - barH)
		}
		sbW := currentStyle.BorderPad.Slider * 2
		drawRoundRect(screen, &roundRect{
			Size:     point{X: sbW, Y: barH},
			Position: point{X: win.getPosition().X + win.GetSize().X - win.BorderPad - sbW, Y: win.getPosition().Y + win.GetTitleSize() + win.BorderPad + pos},
			Fillet:   currentStyle.Fillet.Slider,
			Filled:   true,
			Color:    win.Theme.Window.ActiveColor,
		})
	}
	if req.X > avail.X {
		barW := avail.X * avail.X / req.X
		maxScroll := req.X - avail.X
		pos := float32(0)
		if maxScroll > 0 {
			pos = (win.Scroll.X / maxScroll) * (avail.X - barW)
		}
		sbW := currentStyle.BorderPad.Slider * 2
		drawRoundRect(screen, &roundRect{
			Size:     point{X: barW, Y: sbW},
			Position: point{X: win.getPosition().X + win.BorderPad + pos, Y: win.getPosition().Y + win.GetSize().Y - win.BorderPad - sbW},
			Fillet:   currentStyle.Fillet.Slider,
			Filled:   true,
			Color:    win.Theme.Window.ActiveColor,
		})
	}
}

func (win *windowData) drawItems(screen *ebiten.Image) {
	pad := (win.Padding + win.BorderPad) * uiScale
	winPos := pointAdd(win.getPosition(), point{X: pad, Y: win.GetTitleSize() + pad})
	winPos = pointSub(winPos, win.Scroll)
	clip := win.getMainRect()

	for _, item := range win.Contents {
		itemPos := pointAdd(winPos, item.getPosition(win))

		if item.ItemType == ITEM_FLOW {
			item.drawFlows(win, nil, itemPos, clip, screen)
		} else {
			item.drawItem(nil, itemPos, clip, screen)
		}
	}
}

func (item *itemData) drawFlows(win *windowData, parent *itemData, offset point, clip rect, screen *ebiten.Image) {
	// Store the drawn rectangle for input handling
	itemRect := rect{
		X0: offset.X,
		Y0: offset.Y,
		X1: offset.X + item.GetSize().X,
		Y1: offset.Y + item.GetSize().Y,
	}
	item.DrawRect = intersectRect(itemRect, clip)

	if item.DrawRect.X1 <= item.DrawRect.X0 || item.DrawRect.Y1 <= item.DrawRect.Y0 {
		return
	}
	subImg := screen.SubImage(item.DrawRect.getRectangle()).(*ebiten.Image)
	style := item.themeStyle()

	var activeContents []*itemData
	drawOffset := pointSub(offset, item.Scroll)

	if len(item.Tabs) > 0 {
		if item.ActiveTab >= len(item.Tabs) {
			item.ActiveTab = 0
		}

		tabHeight := float32(defaultTabHeight) * uiScale
		if th := item.FontSize*uiScale + 4; th > tabHeight {
			tabHeight = th
		}
		textSize := (item.FontSize * uiScale) + 2
		x := offset.X
		spacing := float32(4) * uiScale
		for i, tab := range item.Tabs {
			face := textFace(textSize)
			tw, _ := text.Measure(tab.Name, face, 0)
			w := float32(tw) + 8
			if w < float32(defaultTabWidth)*uiScale {
				w = float32(defaultTabWidth) * uiScale
			}
			col := style.Color
			if time.Since(tab.Clicked) < clickFlash {
				col = style.ClickColor
			} else if i == item.ActiveTab {
				if !item.ActiveOutline {
					col = style.SelectedColor
				}
			} else if tab.Hovered {
				col = style.HoverColor
			}
			tab.Hovered = false
			if item.Filled {
				drawTabShape(subImg,
					point{X: x, Y: offset.Y},
					point{X: w, Y: tabHeight},
					col,
					item.Fillet*uiScale,
					item.BorderPad*uiScale,
				)
			}
			if item.Outlined || !item.Filled {
				border := item.Border * uiScale
				if border <= 0 {
					border = 1 * uiScale
				}
				strokeTabShape(subImg,
					point{X: x, Y: offset.Y},
					point{X: w, Y: tabHeight},
					style.OutlineColor,
					item.Fillet*uiScale,
					item.BorderPad*uiScale,
					border,
				)
			}
			if item.ActiveOutline && i == item.ActiveTab {
				strokeTabTop(subImg,
					point{X: x, Y: offset.Y},
					point{X: w, Y: tabHeight},
					style.ClickColor,
					item.Fillet*uiScale,
					item.BorderPad*uiScale,
					3*uiScale,
				)
			}
			loo := text.LayoutOptions{PrimaryAlign: text.AlignCenter, SecondaryAlign: text.AlignCenter}
			dop := ebiten.DrawImageOptions{}
			dop.GeoM.Translate(float64(x+w/2), float64(offset.Y+tabHeight/2))
			dto := &text.DrawOptions{DrawImageOptions: dop, LayoutOptions: loo}
			dto.ColorScale.ScaleWithColor(style.TextColor)
			text.Draw(subImg, tab.Name, face, dto)
			tab.DrawRect = rect{X0: x, Y0: offset.Y, X1: x + w, Y1: offset.Y + tabHeight}
			x += w + spacing
		}
		drawOffset = pointAdd(drawOffset, point{Y: tabHeight})
		drawFilledRect(subImg,
			offset.X,
			offset.Y+tabHeight-3*uiScale,
			item.GetSize().X,
			3*uiScale,
			style.SelectedColor,
			false)
		strokeRect(subImg,
			offset.X,
			offset.Y+tabHeight,
			item.GetSize().X,
			item.GetSize().Y-tabHeight,
			1,
			style.OutlineColor,
			false)
		activeContents = item.Tabs[item.ActiveTab].Contents
	} else {
		activeContents = item.Contents
	}

	var flowOffset point

	for _, subItem := range activeContents {

		if subItem.ItemType == ITEM_FLOW {
			flowPos := pointAdd(drawOffset, item.GetPos())
			flowOff := pointAdd(flowPos, flowOffset)
			itemPos := pointAdd(flowOff, subItem.GetPos())
			subItem.drawFlows(win, item, itemPos, item.DrawRect, screen)
		} else {
			flowOff := pointAdd(drawOffset, flowOffset)

			if subItem.PinTo != PIN_TOP_LEFT {
				pad := (win.Padding + win.BorderPad) * uiScale
				objOff := pointAdd(win.getPosition(), point{X: pad, Y: win.GetTitleSize() + pad})
				objOff = pointSub(objOff, win.Scroll)
				objOff = pointAdd(objOff, subItem.getPosition(win))
				subItem.drawItem(item, objOff, win.getMainRect(), screen)
			} else {
				objOff := flowOff
				if parent != nil && parent.ItemType == ITEM_FLOW {
					objOff = pointAdd(objOff, subItem.GetPos())
				}
				subItem.drawItem(item, objOff, item.DrawRect, screen)
			}
		}

		if item.ItemType == ITEM_FLOW {
			if item.FlowType == FLOW_HORIZONTAL {
				flowOffset = pointAdd(flowOffset, point{X: subItem.GetSize().X, Y: 0})
				flowOffset = pointAdd(flowOffset, point{X: subItem.GetPos().X})
			} else if item.FlowType == FLOW_VERTICAL {
				flowOffset = pointAdd(flowOffset, point{X: 0, Y: subItem.GetSize().Y})
				flowOffset = pointAdd(flowOffset, point{Y: subItem.GetPos().Y})
			}
		}
	}

	if item.Scrollable {
		req := item.contentBounds()
		size := item.GetSize()
		if item.FlowType == FLOW_VERTICAL && req.Y > size.Y {
			barH := size.Y * size.Y / req.Y
			maxScroll := req.Y - size.Y
			pos := float32(0)
			if maxScroll > 0 {
				pos = (item.Scroll.Y / maxScroll) * (size.Y - barH)
			}
			col := NewColor(96, 96, 96, 192)
			sbW := currentStyle.BorderPad.Slider * 2
			drawFilledRect(subImg, item.DrawRect.X1-sbW, item.DrawRect.Y0+pos, sbW, barH, col.ToRGBA(), false)
		} else if item.FlowType == FLOW_HORIZONTAL && req.X > size.X {
			barW := size.X * size.X / req.X
			maxScroll := req.X - size.X
			pos := float32(0)
			if maxScroll > 0 {
				pos = (item.Scroll.X / maxScroll) * (size.X - barW)
			}
			col := NewColor(96, 96, 96, 192)
			sbW := currentStyle.BorderPad.Slider * 2
			drawFilledRect(subImg, item.DrawRect.X0+pos, item.DrawRect.Y1-sbW, barW, sbW, col.ToRGBA(), false)
		}
	}

	if DebugMode {
		strokeRect(subImg,
			item.DrawRect.X0,
			item.DrawRect.Y0,
			item.DrawRect.X1-item.DrawRect.X0,
			item.DrawRect.Y1-item.DrawRect.Y0,
			1,
			Color{G: 255},
			false)

		midX := (item.DrawRect.X0 + item.DrawRect.X1) / 2
		midY := (item.DrawRect.Y0 + item.DrawRect.Y1) / 2
		margin := float32(4) * uiScale
		col := Color{B: 255, A: 255}

		switch item.FlowType {
		case FLOW_HORIZONTAL:
			drawArrow(subImg, item.DrawRect.X0+margin, midY, item.DrawRect.X1-margin, midY, 1, col)
		case FLOW_VERTICAL:
			drawArrow(subImg, midX, item.DrawRect.Y0+margin, midX, item.DrawRect.Y1-margin, 1, col)
		case FLOW_HORIZONTAL_REV:
			drawArrow(subImg, item.DrawRect.X1-margin, midY, item.DrawRect.X0+margin, midY, 1, col)
		case FLOW_VERTICAL_REV:
			drawArrow(subImg, midX, item.DrawRect.Y1-margin, midX, item.DrawRect.Y0+margin, 1, col)
		}
	}
}

func (item *itemData) drawItemInternal(parent *itemData, offset point, clip rect, screen *ebiten.Image) {

	if parent == nil {
		parent = item
	}
	maxSize := item.GetSize()
	if item.Size.X > parent.Size.X {
		maxSize.X = parent.GetSize().X
	}
	if item.Size.Y > parent.Size.Y {
		maxSize.Y = parent.GetSize().Y
	}

	itemRect := rect{
		X0: offset.X,
		Y0: offset.Y,
		X1: offset.X + maxSize.X,
		Y1: offset.Y + maxSize.Y,
	}
	item.DrawRect = intersectRect(itemRect, clip)
	if item.DrawRect.X1 <= item.DrawRect.X0 || item.DrawRect.Y1 <= item.DrawRect.Y0 {
		return
	}
	subImg := screen.SubImage(item.DrawRect.getRectangle()).(*ebiten.Image)
	style := item.themeStyle()

	if item.Label != "" {
		textSize := (item.FontSize * uiScale) + 2
		face := textFace(textSize)
		loo := text.LayoutOptions{PrimaryAlign: text.AlignStart, SecondaryAlign: text.AlignCenter}
		tdop := ebiten.DrawImageOptions{}
		tdop.GeoM.Translate(float64(offset.X), float64(offset.Y+textSize/2))
		top := &text.DrawOptions{DrawImageOptions: tdop, LayoutOptions: loo}
		if style != nil {
			top.ColorScale.ScaleWithColor(style.TextColor)
		}
		text.Draw(subImg, item.Label, face, top)
		offset.Y += textSize + currentStyle.TextPadding*uiScale
		maxSize.Y -= textSize + currentStyle.TextPadding*uiScale
		if maxSize.Y < 0 {
			maxSize.Y = 0
		}
	}

	if item.ItemType == ITEM_CHECKBOX {

		bThick := item.Border * uiScale
		itemColor := style.Color
		bColor := style.OutlineColor
		if item.Checked {
			itemColor = style.ClickColor
			bColor = style.Color
		} else if item.Hovered {
			item.Hovered = false
			itemColor = style.HoverColor
		}
		auxSize := pointScaleMul(item.AuxSize)
		if item.Filled {
			drawRoundRect(subImg, &roundRect{
				Size:     auxSize,
				Position: offset,
				Fillet:   item.Fillet,
				Filled:   true,
				Color:    itemColor,
			})
		}
		drawRoundRect(subImg, &roundRect{
			Size:     auxSize,
			Position: offset,
			Fillet:   item.Fillet,
			Filled:   false,
			Color:    bColor,
			Border:   bThick,
		})

		if item.Checked {
			cThick := 2 * uiScale
			margin := auxSize.X * 0.25

			start := point{X: offset.X + margin, Y: offset.Y + auxSize.Y*0.55}
			mid := point{X: offset.X + auxSize.X*0.45, Y: offset.Y + auxSize.Y - margin}
			end := point{X: offset.X + auxSize.X - margin, Y: offset.Y + margin}

			drawCheckmark(subImg, start, mid, end, cThick, style.TextColor)
		}

		textSize := (item.FontSize * uiScale) + 2
		face := textFace(textSize)
		loo := text.LayoutOptions{
			LineSpacing:    1.2,
			PrimaryAlign:   text.AlignStart,
			SecondaryAlign: text.AlignCenter,
		}
		tdop := ebiten.DrawImageOptions{}
		tdop.GeoM.Translate(
			float64(offset.X+auxSize.X+item.AuxSpace),
			float64(offset.Y+(auxSize.Y/2)),
		)
		top := &text.DrawOptions{DrawImageOptions: tdop, LayoutOptions: loo}
		top.ColorScale.ScaleWithColor(style.TextColor)
		text.Draw(subImg, item.Text, face, top)

	} else if item.ItemType == ITEM_RADIO {

		bThick := item.Border * uiScale
		itemColor := style.Color
		bColor := style.OutlineColor
		if item.Checked {
			itemColor = style.ClickColor
			bColor = style.OutlineColor
		} else if item.Hovered {
			item.Hovered = false
			itemColor = style.HoverColor
		}
		auxSize := pointScaleMul(item.AuxSize)
		if item.Filled {
			drawRoundRect(subImg, &roundRect{
				Size:     auxSize,
				Position: offset,
				Fillet:   auxSize.X / 2,
				Filled:   true,
				Color:    itemColor,
			})
		}
		drawRoundRect(subImg, &roundRect{
			Size:     auxSize,
			Position: offset,
			Fillet:   auxSize.X / 2,
			Filled:   false,
			Color:    bColor,
			Border:   bThick,
		})
		if item.Checked {
			inner := auxSize.X / 2.5
			drawRoundRect(subImg, &roundRect{
				Size:     point{X: inner, Y: inner},
				Position: point{X: offset.X + (auxSize.X-inner)/2, Y: offset.Y + (auxSize.Y-inner)/2},
				Fillet:   inner / 2,
				Filled:   true,
				Color:    style.TextColor,
			})
		}

		textSize := (item.FontSize * uiScale) + 2
		face := textFace(textSize)
		loo := text.LayoutOptions{
			LineSpacing:    1.2,
			PrimaryAlign:   text.AlignStart,
			SecondaryAlign: text.AlignCenter,
		}
		tdop := ebiten.DrawImageOptions{}
		tdop.GeoM.Translate(
			float64(offset.X+auxSize.X+item.AuxSpace),
			float64(offset.Y+(auxSize.Y/2)),
		)
		top := &text.DrawOptions{DrawImageOptions: tdop, LayoutOptions: loo}
		top.ColorScale.ScaleWithColor(style.TextColor)
		text.Draw(subImg, item.Text, face, top)

	} else if item.ItemType == ITEM_BUTTON {

		if item.Image != nil {
			sop := &ebiten.DrawImageOptions{}
			sop.GeoM.Scale(float64(maxSize.X)/float64(item.Image.Bounds().Dx()),
				float64(maxSize.Y)/float64(item.Image.Bounds().Dy()))
			sop.GeoM.Translate(float64(offset.X), float64(offset.Y))
			subImg.DrawImage(item.Image, sop)
		} else {
			itemColor := style.Color
			if time.Since(item.Clicked) < clickFlash {
				itemColor = style.ClickColor
			} else if item.Hovered {
				item.Hovered = false
				itemColor = style.HoverColor
			}
			if item.Filled {
				drawRoundRect(subImg, &roundRect{
					Size:     maxSize,
					Position: offset,
					Fillet:   item.Fillet,
					Filled:   true,
					Color:    itemColor,
				})
			}
		}

		textSize := (item.FontSize * uiScale) + 2
		face := textFace(textSize)
		loo := text.LayoutOptions{
			LineSpacing:    0,
			PrimaryAlign:   text.AlignCenter,
			SecondaryAlign: text.AlignCenter,
		}
		tdop := ebiten.DrawImageOptions{}
		tdop.GeoM.Translate(
			float64(offset.X+((maxSize.X)/2)),
			float64(offset.Y+((maxSize.Y)/2)))
		top := &text.DrawOptions{DrawImageOptions: tdop, LayoutOptions: loo}
		top.ColorScale.ScaleWithColor(style.TextColor)
		text.Draw(subImg, item.Text, face, top)

		//Text
	} else if item.ItemType == ITEM_INPUT {

		itemColor := style.Color
		if item.Focused {
			itemColor = style.ClickColor
		} else if item.Hovered {
			item.Hovered = false
			itemColor = style.HoverColor
		}

		if item.Filled {
			drawRoundRect(subImg, &roundRect{
				Size:     maxSize,
				Position: offset,
				Fillet:   item.Fillet,
				Filled:   true,
				Color:    itemColor,
			})
		}

		textSize := (item.FontSize * uiScale) + 2
		face := textFace(textSize)
		loo := text.LayoutOptions{
			LineSpacing:    0,
			PrimaryAlign:   text.AlignStart,
			SecondaryAlign: text.AlignCenter,
		}
		tdop := ebiten.DrawImageOptions{}
		tdop.GeoM.Translate(
			float64(offset.X+item.BorderPad+item.Padding+currentStyle.TextPadding*uiScale),
			float64(offset.Y+((maxSize.Y)/2)),
		)
		top := &text.DrawOptions{DrawImageOptions: tdop, LayoutOptions: loo}
		top.ColorScale.ScaleWithColor(style.TextColor)
		text.Draw(subImg, item.Text, face, top)

		if item.Focused {
			width, _ := text.Measure(item.Text, face, 0)
			cx := offset.X + item.BorderPad + item.Padding + currentStyle.TextPadding*uiScale + float32(width)
			strokeLine(subImg,
				cx, offset.Y+2,
				cx, offset.Y+maxSize.Y-2,
				1, style.TextColor, false)
		}

	} else if item.ItemType == ITEM_SLIDER {

		itemColor := style.Color
		if item.Hovered {
			item.Hovered = false
			itemColor = style.HoverColor
		}

		// Prepare value text and measure the largest value label so the
		// slider track remains consistent length
		// Use a constant max label width so all sliders have the
		// same track length regardless of their numeric range.
		valueText := fmt.Sprintf("%.2f", item.Value)
		maxLabel := sliderMaxLabel
		if item.IntOnly {
			// Pad the integer value so the value field width matches
			// the float slider which reserves space for two decimal
			// places.
			width := len(maxLabel)
			valueText = fmt.Sprintf("%*d", width, int(item.Value))
		}

		textSize := (item.FontSize * uiScale) + 2
		face := textFace(textSize)
		maxW, _ := text.Measure(maxLabel, face, 0)

		gap := currentStyle.SliderValueGap
		knobW := item.AuxSize.X * uiScale
		knobH := item.AuxSize.Y * uiScale
		trackWidth := maxSize.X - knobW - gap - float32(maxW)
		if trackWidth < 0 {
			trackWidth = 0
		}

		trackStart := offset.X + knobW/2
		trackY := offset.Y + maxSize.Y/2

		ratio := 0.0
		if item.MaxValue > item.MinValue {
			ratio = float64((item.Value - item.MinValue) / (item.MaxValue - item.MinValue))
		}
		if ratio < 0 {
			ratio = 0
		} else if ratio > 1 {
			ratio = 1
		}
		knobCenter := trackStart + float32(ratio)*trackWidth
		filledCol := style.SelectedColor
		strokeLine(subImg, trackStart, trackY, knobCenter, trackY, 2*uiScale, filledCol, true)
		strokeLine(subImg, knobCenter, trackY, trackStart+trackWidth, trackY, 2*uiScale, itemColor, true)
		knobRect := point{X: knobCenter - knobW/2, Y: offset.Y + (maxSize.Y-knobH)/2}
		drawRoundRect(subImg, &roundRect{
			Size:     pointScaleMul(item.AuxSize),
			Position: knobRect,
			Fillet:   item.Fillet,
			Filled:   true,
			Color:    style.Color,
		})
		drawRoundRect(subImg, &roundRect{
			Size:     pointScaleMul(item.AuxSize),
			Position: knobRect,
			Fillet:   item.Fillet,
			Filled:   false,
			Border:   1 * uiScale,
			Color:    style.OutlineColor,
		})

		// value text drawn to the right of the slider track
		loo := text.LayoutOptions{LineSpacing: 1.2, PrimaryAlign: text.AlignStart, SecondaryAlign: text.AlignCenter}
		tdop := ebiten.DrawImageOptions{}
		tdop.GeoM.Translate(
			float64(trackStart+trackWidth+gap),
			float64(offset.Y+(maxSize.Y/2)),
		)
		top := &text.DrawOptions{DrawImageOptions: tdop, LayoutOptions: loo}
		top.ColorScale.ScaleWithColor(style.TextColor)
		text.Draw(subImg, valueText, face, top)

	} else if item.ItemType == ITEM_DROPDOWN {

		itemColor := style.Color
		if item.Open {
			itemColor = style.SelectedColor
		} else if item.Hovered {
			item.Hovered = false
			itemColor = style.HoverColor
		}

		if item.Filled {
			drawRoundRect(subImg, &roundRect{
				Size:     maxSize,
				Position: offset,
				Fillet:   item.Fillet,
				Filled:   true,
				Color:    itemColor,
			})
		}

		textSize := (item.FontSize * uiScale) + 2
		face := textFace(textSize)
		loo := text.LayoutOptions{PrimaryAlign: text.AlignStart, SecondaryAlign: text.AlignCenter}
		tdop := ebiten.DrawImageOptions{}
		tdop.GeoM.Translate(float64(offset.X+item.BorderPad+item.Padding+currentStyle.TextPadding*uiScale), float64(offset.Y+maxSize.Y/2))
		top := &text.DrawOptions{DrawImageOptions: tdop, LayoutOptions: loo}
		top.ColorScale.ScaleWithColor(style.TextColor)
		label := item.Text
		if item.Selected >= 0 && item.Selected < len(item.Options) {
			label = item.Options[item.Selected]
		}
		text.Draw(subImg, label, face, top)

		arrow := maxSize.Y * 0.4
		drawTriangle(subImg,
			point{X: offset.X + maxSize.X - arrow - item.BorderPad - item.Padding - currentStyle.DropdownArrowPad,
				Y: offset.Y + (maxSize.Y-arrow)/2},
			arrow,
			style.TextColor)

	} else if item.ItemType == ITEM_COLORWHEEL {

		wheelSize := maxSize.Y
		if wheelSize > maxSize.X {
			wheelSize = maxSize.X
		}

		if item.Image == nil || item.Image.Bounds().Dx() != int(wheelSize) {
			item.Image = colorWheelImage(int(wheelSize))
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(offset.X), float64(offset.Y))
		subImg.DrawImage(item.Image, op)

		h, _, v, _ := rgbaToHSVA(color.RGBA(item.WheelColor))
		radius := wheelSize / 2
		cx := offset.X + radius
		cy := offset.Y + radius
		px := cx + float32(math.Cos(h*math.Pi/180))*radius*float32(v)
		py := cy + float32(math.Sin(h*math.Pi/180))*radius*float32(v)
		vector.DrawFilledCircle(subImg, px, py, 4*uiScale, color.Black, true)
		vector.DrawFilledCircle(subImg, px, py, 2*uiScale, color.White, true)

		sw := wheelSize / 5
		if sw < 10*uiScale {
			sw = 10 * uiScale
		}
		sx := offset.X + wheelSize + 4*uiScale
		sy := offset.Y + maxSize.Y - sw - 4*uiScale
		drawFilledRect(subImg, sx, sy, sw, sw, color.RGBA(item.WheelColor), true)
		strokeRect(subImg, sx, sy, sw, sw, 1, color.Black, true)

	} else if item.ItemType == ITEM_TEXT {

		textSize := (item.FontSize * uiScale) + 2
		face := textFace(textSize)
		loo := text.LayoutOptions{
			LineSpacing:    float64(textSize) * 1.2,
			PrimaryAlign:   text.AlignStart,
			SecondaryAlign: text.AlignStart,
		}
		tdop := ebiten.DrawImageOptions{}
		tdop.GeoM.Translate(
			float64(offset.X),
			float64(offset.Y))

		top := &text.DrawOptions{DrawImageOptions: tdop, LayoutOptions: loo}
		top.ColorScale.ScaleWithColor(style.TextColor)
		text.Draw(subImg, item.Text, face, top)
	}

	if item.Outlined && item.Border > 0 && item.ItemType != ITEM_CHECKBOX && item.ItemType != ITEM_RADIO {
		drawRoundRect(subImg, &roundRect{
			Size:     maxSize,
			Position: offset,
			Fillet:   item.Fillet,
			Filled:   false,
			Color:    style.OutlineColor,
			Border:   item.Border * uiScale,
		})
	}

	if DebugMode {
		strokeRect(subImg,
			item.DrawRect.X0,
			item.DrawRect.Y0,
			item.DrawRect.X1-item.DrawRect.X0,
			item.DrawRect.Y1-item.DrawRect.Y0,
			1, color.RGBA{R: 128}, false)
	}

}

func (item *itemData) drawItem(parent *itemData, offset point, clip rect, screen *ebiten.Image) {
	if item.ItemType != ITEM_FLOW {

		if parent == nil {
			parent = item
		}
		maxSize := item.GetSize()
		if item.Size.X > parent.Size.X {
			maxSize.X = parent.GetSize().X
		}
		if item.Size.Y > parent.Size.Y {
			maxSize.Y = parent.GetSize().Y
		}

		itemRect := rect{X0: offset.X, Y0: offset.Y, X1: offset.X + maxSize.X, Y1: offset.Y + maxSize.Y}
		item.DrawRect = intersectRect(itemRect, clip)
		if item.DrawRect.X1 <= item.DrawRect.X0 || item.DrawRect.Y1 <= item.DrawRect.Y0 {
			return
		}

		src := image.Rect(int(item.DrawRect.X0-offset.X), int(item.DrawRect.Y0-offset.Y), int(item.DrawRect.X1-offset.X), int(item.DrawRect.Y1-offset.Y))
		if item.Render == nil {
			return // optionally log missing render
		}
		sub := item.Render.SubImage(src).(*ebiten.Image)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(item.DrawRect.X0), float64(item.DrawRect.Y0))
		screen.DrawImage(sub, op)

		if item.ItemType == ITEM_DROPDOWN && item.Open {
			dropOff := offset
			if item.Label != "" {
				textSize := (item.FontSize * uiScale) + 2
				dropOff.Y += textSize + currentStyle.TextPadding*uiScale
			}
			screenClip := rect{X0: 0, Y0: 0, X1: float32(screenWidth), Y1: float32(screenHeight)}
			pendingDropdowns = append(pendingDropdowns, dropdownRender{item: item, offset: dropOff, clip: screenClip})
		}

		if DebugMode {
			strokeRect(screen, item.DrawRect.X0, item.DrawRect.Y0, item.DrawRect.X1-item.DrawRect.X0, item.DrawRect.Y1-item.DrawRect.Y0, 1, color.RGBA{R: 128}, false)
		}
		return
	}

	item.drawItemInternal(parent, offset, clip, screen)
}

func drawDropdownOptions(item *itemData, offset point, clip rect, screen *ebiten.Image) {
	maxSize := item.GetSize()
	optionH := maxSize.Y
	drawRect, visible := dropdownOpenRect(item, offset)
	startY := drawRect.Y0
	first := int(item.Scroll.Y / optionH)
	offY := startY - (item.Scroll.Y - float32(first)*optionH)
	textSize := (item.FontSize * uiScale) + 2
	face := textFace(textSize)
	loo := text.LayoutOptions{PrimaryAlign: text.AlignStart, SecondaryAlign: text.AlignCenter}

	if item.ShadowSize > 0 && item.ShadowColor.A > 0 {
		rr := roundRect{
			Size:     point{X: drawRect.X1 - drawRect.X0, Y: drawRect.Y1 - drawRect.Y0},
			Position: point{X: drawRect.X0, Y: drawRect.Y0},
			Fillet:   item.Fillet,
			Filled:   true,
			Color:    item.ShadowColor,
		}
		drawDropShadow(screen, &rr, item.ShadowSize, item.ShadowColor)
	}
	visibleRect := intersectRect(drawRect, clip)
	if visibleRect.X1 <= visibleRect.X0 || visibleRect.Y1 <= visibleRect.Y0 {
		return
	}
	subImg := screen.SubImage(visibleRect.getRectangle()).(*ebiten.Image)
	style := item.themeStyle()
	drawFilledRect(subImg,
		visibleRect.X0,
		visibleRect.Y0,
		visibleRect.X1-visibleRect.X0,
		visibleRect.Y1-visibleRect.Y0,
		style.Color, false)
	for i := first; i < first+visible && i < len(item.Options); i++ {
		y := offY + float32(i-first)*optionH
		if i == item.Selected || i == item.HoverIndex {
			col := style.SelectedColor
			if i == item.HoverIndex && i != item.Selected {
				col = style.HoverColor
			}
			drawRoundRect(subImg, &roundRect{Size: maxSize, Position: point{X: offset.X, Y: y}, Fillet: item.Fillet, Filled: true, Color: col})
		}
		td := ebiten.DrawImageOptions{}
		td.GeoM.Translate(float64(offset.X+item.BorderPad+item.Padding+currentStyle.TextPadding*uiScale), float64(y+optionH/2))
		tdo := &text.DrawOptions{DrawImageOptions: td, LayoutOptions: loo}
		tdo.ColorScale.ScaleWithColor(style.TextColor)
		text.Draw(subImg, item.Options[i], face, tdo)
	}

	if len(item.Options) > visible {
		openH := optionH * float32(visible)
		totalH := optionH * float32(len(item.Options))
		barH := openH * openH / totalH
		maxScroll := totalH - openH
		pos := float32(0)
		if maxScroll > 0 {
			pos = (item.Scroll.Y / maxScroll) * (openH - barH)
		}
		col := NewColor(96, 96, 96, 192)
		sbW := currentStyle.BorderPad.Slider * 2
		drawFilledRect(subImg, drawRect.X1-sbW, startY+pos, sbW, barH, col.ToRGBA(), false)
	}
}

func (win *windowData) drawDebug(screen *ebiten.Image) {
	if DebugMode {
		grab := win.getMainRect()
		strokeRect(screen, grab.X0, grab.Y0, grab.X1-grab.X0, grab.Y1-grab.Y0, 1, color.RGBA{R: 255, G: 255, A: 255}, false)

		grab = win.dragbarRect()
		strokeRect(screen, grab.X0, grab.Y0, grab.X1-grab.X0, grab.Y1-grab.Y0, 1, color.RGBA{R: 255, A: 255}, false)

		grab = win.xRect()
		strokeRect(screen, grab.X0, grab.Y0, grab.X1-grab.X0, grab.Y1-grab.Y0, 1, color.RGBA{G: 255, A: 255}, false)

		grab = win.getTitleRect()
		strokeRect(screen, grab.X0, grab.Y0, grab.X1-grab.X0, grab.Y1-grab.Y0, 1, color.RGBA{B: 255, G: 255, A: 255}, false)
	}
}

// drawDropShadow draws a simple drop shadow by offsetting and expanding the
// provided rounded rectangle before drawing it. The shadow is drawn using the
// specified color with the alpha preserved.
func drawDropShadow(screen *ebiten.Image, rrect *roundRect, size float32, col Color) {
	if size <= 0 || col.A == 0 {
		return
	}

	layers := int(math.Ceil(float64(size)))
	if layers < 1 {
		layers = 1
	}

	step := size / float32(layers)
	for i := layers; i >= 1; i-- {
		expand := step * float32(i)
		alpha := float32(col.A) * float32(layers-i+1) / float32(layers)

		shadow := *rrect
		shadow.Position.X -= expand
		shadow.Position.Y -= expand
		shadow.Size.X += expand * 2
		shadow.Size.Y += expand * 2
		shadow.Fillet += expand
		shadow.Color = Color{R: col.R, G: col.G, B: col.B, A: uint8(alpha / shadowAlphaDivisor)}
		shadow.Filled = true
		drawRoundRect(screen, &shadow)
	}
}

func drawRoundRect(screen *ebiten.Image, rrect *roundRect) {
	var (
		path     vector.Path
		vertices []ebiten.Vertex
		indices  []uint16
	)

	width := float32(math.Round(float64(rrect.Border)))
	off := float32(0)
	if !rrect.Filled {
		off = pixelOffset(width)
	}

	x := float32(math.Round(float64(rrect.Position.X))) + off
	y := float32(math.Round(float64(rrect.Position.Y))) + off
	x1 := float32(math.Round(float64(rrect.Position.X+rrect.Size.X))) + off
	y1 := float32(math.Round(float64(rrect.Position.Y+rrect.Size.Y))) + off
	w := x1 - x
	h := y1 - y
	fillet := rrect.Fillet

	// When stroking, keep the outline fully inside the rectangle so
	// sub-images do not clip the bottom and right edges.
	if !rrect.Filled && width > 0 {
		inset := width / 2
		x += inset
		y += inset
		w -= width
		h -= width
		if w < 0 {
			w = 0
		}
		if h < 0 {
			h = 0
		}
		if fillet > inset {
			fillet -= inset
		} else {
			fillet = 0
		}
	}

	if fillet*2 > w {
		fillet = w / 2
	}
	if fillet*2 > h {
		fillet = h / 2
	}
	fillet = float32(math.Round(float64(fillet)))

	path.MoveTo(x+fillet, y)
	path.LineTo(x+w-fillet, y)
	path.QuadTo(
		x+w,
		y,
		x+w,
		y+fillet)
	path.LineTo(x+w, y+h-fillet)
	path.QuadTo(
		x+w,
		y+h,
		x+w-fillet,
		y+h)
	path.LineTo(x+fillet, y+h)
	path.QuadTo(
		x,
		y+h,
		x,
		y+h-fillet)
	path.LineTo(x, y+fillet)
	path.QuadTo(
		x,
		y,
		x+fillet,
		y)
	path.Close()

	if rrect.Filled {
		vertices, indices = path.AppendVerticesAndIndicesForFilling(vertices[:0], indices[:0])
	} else {
		opv := &vector.StrokeOptions{Width: width}
		vertices, indices = path.AppendVerticesAndIndicesForStroke(vertices[:0], indices[:0], opv)
	}

	col := rrect.Color
	for i := range vertices {
		vertices[i].SrcX = 1
		vertices[i].SrcY = 1
		vertices[i].ColorR = float32(col.R) / 255
		vertices[i].ColorG = float32(col.G) / 255
		vertices[i].ColorB = float32(col.B) / 255
		vertices[i].ColorA = float32(col.A) / 255
	}

	op := &ebiten.DrawTrianglesOptions{FillRule: ebiten.FillRuleNonZero, AntiAlias: true}
	screen.DrawTriangles(vertices, indices, whiteSubImage, op)
}

func drawTabShape(screen *ebiten.Image, pos point, size point, col Color, fillet float32, slope float32) {
	var (
		path     vector.Path
		vertices []ebiten.Vertex
		indices  []uint16
	)

	// Align to pixel boundaries to avoid artifacts
	pos.X = float32(math.Round(float64(pos.X)))
	pos.Y = float32(math.Round(float64(pos.Y)))
	size.X = float32(math.Round(float64(size.X)))
	size.Y = float32(math.Round(float64(size.Y)))

	origFillet := fillet

	if slope <= 0 {
		slope = size.Y / 4
	}
	if fillet <= 0 {
		fillet = size.Y / 8
	}
	fillet = float32(math.Round(float64(fillet)))

	path.MoveTo(pos.X, pos.Y+size.Y)
	path.LineTo(pos.X+slope, pos.Y+size.Y)
	path.LineTo(pos.X+slope, pos.Y+fillet)
	path.QuadTo(pos.X+slope, pos.Y, pos.X+slope+fillet, pos.Y)
	path.LineTo(pos.X+size.X-slope-fillet, pos.Y)
	path.QuadTo(pos.X+size.X-slope, pos.Y, pos.X+size.X-slope, pos.Y+fillet)
	path.LineTo(pos.X+size.X-slope, pos.Y+size.Y)
	path.LineTo(pos.X, pos.Y+size.Y)
	path.Close()

	vertices, indices = path.AppendVerticesAndIndicesForFilling(vertices[:0], indices[:0])
	c := col
	for i := range vertices {
		vertices[i].SrcX = 1
		vertices[i].SrcY = 1
		vertices[i].ColorR = float32(c.R) / 255
		vertices[i].ColorG = float32(c.G) / 255
		vertices[i].ColorB = float32(c.B) / 255
		vertices[i].ColorA = float32(c.A) / 255
	}

	op := &ebiten.DrawTrianglesOptions{}
	op.FillRule = ebiten.FillRuleNonZero
	op.AntiAlias = origFillet > 0
	screen.DrawTriangles(vertices, indices, whiteSubImage, op)
}

func strokeTabShape(screen *ebiten.Image, pos point, size point, col Color, fillet float32, slope float32, border float32) {
	var (
		path     vector.Path
		vertices []ebiten.Vertex
		indices  []uint16
	)

	// Align to pixel boundaries
	border = float32(math.Round(float64(border)))
	off := pixelOffset(border)
	pos.X = float32(math.Round(float64(pos.X))) + off
	pos.Y = float32(math.Round(float64(pos.Y))) + off
	size.X = float32(math.Round(float64(size.X)))
	size.Y = float32(math.Round(float64(size.Y)))

	if slope <= 0 {
		slope = size.Y / 4
	}
	if fillet <= 0 {
		fillet = size.Y / 8
	}
	fillet = float32(math.Round(float64(fillet)))

	path.MoveTo(pos.X, pos.Y+size.Y)
	path.LineTo(pos.X+slope, pos.Y+size.Y)
	path.LineTo(pos.X+slope, pos.Y+fillet)
	path.QuadTo(pos.X+slope, pos.Y, pos.X+slope+fillet, pos.Y)
	path.LineTo(pos.X+size.X-slope-fillet, pos.Y)
	path.QuadTo(pos.X+size.X-slope, pos.Y, pos.X+size.X-slope, pos.Y+fillet)
	path.LineTo(pos.X+size.X-slope, pos.Y+size.Y)
	path.LineTo(pos.X, pos.Y+size.Y)
	path.Close()

	opv := &vector.StrokeOptions{Width: border}
	vertices, indices = path.AppendVerticesAndIndicesForStroke(vertices[:0], indices[:0], opv)
	c := col
	for i := range vertices {
		vertices[i].SrcX = 1
		vertices[i].SrcY = 1
		vertices[i].ColorR = float32(c.R) / 255
		vertices[i].ColorG = float32(c.G) / 255
		vertices[i].ColorB = float32(c.B) / 255
		vertices[i].ColorA = float32(c.A) / 255
	}

	op := &ebiten.DrawTrianglesOptions{FillRule: ebiten.FillRuleNonZero, AntiAlias: true}
	screen.DrawTriangles(vertices, indices, whiteSubImage, op)
}

func strokeTabTop(screen *ebiten.Image, pos point, size point, col Color, fillet float32, slope float32, border float32) {
	var (
		path     vector.Path
		vertices []ebiten.Vertex
		indices  []uint16
	)

	border = float32(math.Round(float64(border)))
	off := pixelOffset(border)
	pos.X = float32(math.Round(float64(pos.X))) + off
	pos.Y = float32(math.Round(float64(pos.Y))) + off
	size.X = float32(math.Round(float64(size.X)))
	size.Y = float32(math.Round(float64(size.Y)))

	if slope <= 0 {
		slope = size.Y / 4
	}
	if fillet < 0 {
		fillet = size.Y / 8
	}
	fillet = float32(math.Round(float64(fillet)))

	if fillet > 0 {
		path.MoveTo(pos.X+slope+fillet, pos.Y)
		path.LineTo(pos.X+size.X-slope-fillet, pos.Y)
	} else {
		path.MoveTo(pos.X+slope, pos.Y)
		path.LineTo(pos.X+size.X-slope, pos.Y)
	}

	opv := &vector.StrokeOptions{Width: border}
	vertices, indices = path.AppendVerticesAndIndicesForStroke(vertices[:0], indices[:0], opv)
	c := col
	for i := range vertices {
		vertices[i].SrcX = 1
		vertices[i].SrcY = 1
		vertices[i].ColorR = float32(c.R) / 255
		vertices[i].ColorG = float32(c.G) / 255
		vertices[i].ColorB = float32(c.B) / 255
		vertices[i].ColorA = float32(c.A) / 255
	}

	op := &ebiten.DrawTrianglesOptions{FillRule: ebiten.FillRuleNonZero, AntiAlias: true}
	screen.DrawTriangles(vertices, indices, whiteSubImage, op)
}

func drawTriangle(screen *ebiten.Image, pos point, size float32, col Color) {
	var (
		path     vector.Path
		vertices []ebiten.Vertex
		indices  []uint16
	)

	// Quantize to pixel boundaries
	pos.X = float32(math.Round(float64(pos.X)))
	pos.Y = float32(math.Round(float64(pos.Y)))
	size = float32(math.Round(float64(size)))

	path.MoveTo(pos.X, pos.Y)
	path.LineTo(pos.X+size, pos.Y)
	path.LineTo(pos.X+size/2, pos.Y+size)
	path.Close()

	vertices, indices = path.AppendVerticesAndIndicesForFilling(vertices[:0], indices[:0])
	c := col
	for i := range vertices {
		vertices[i].SrcX = 1
		vertices[i].SrcY = 1
		vertices[i].ColorR = float32(c.R) / 255
		vertices[i].ColorG = float32(c.G) / 255
		vertices[i].ColorB = float32(c.B) / 255
		vertices[i].ColorA = float32(c.A) / 255
	}

	op := &ebiten.DrawTrianglesOptions{FillRule: ebiten.FillRuleNonZero, AntiAlias: true}
	screen.DrawTriangles(vertices, indices, whiteSubImage, op)
}

func drawCheckmark(screen *ebiten.Image, start, mid, end point, width float32, col Color) {
	var (
		path     vector.Path
		vertices []ebiten.Vertex
		indices  []uint16
	)

	width = float32(math.Round(float64(width)))
	off := pixelOffset(width)

	path.MoveTo(float32(math.Round(float64(start.X)))+off, float32(math.Round(float64(start.Y)))+off)
	path.LineTo(float32(math.Round(float64(mid.X)))+off, float32(math.Round(float64(mid.Y)))+off)
	path.LineTo(float32(math.Round(float64(end.X)))+off, float32(math.Round(float64(end.Y)))+off)

	opv := &vector.StrokeOptions{Width: width, LineJoin: vector.LineJoinRound, LineCap: vector.LineCapRound}
	vertices, indices = path.AppendVerticesAndIndicesForStroke(vertices[:0], indices[:0], opv)
	c := col
	for i := range vertices {
		vertices[i].SrcX = 1
		vertices[i].SrcY = 1
		vertices[i].ColorR = float32(c.R) / 255
		vertices[i].ColorG = float32(c.G) / 255
		vertices[i].ColorB = float32(c.B) / 255
		vertices[i].ColorA = float32(c.A) / 255
	}

	op := &ebiten.DrawTrianglesOptions{FillRule: ebiten.FillRuleNonZero, AntiAlias: true}
	screen.DrawTriangles(vertices, indices, whiteSubImage, op)
}

func drawArrow(screen *ebiten.Image, x0, y0, x1, y1, width float32, col Color) {
	strokeLine(screen, x0, y0, x1, y1, width, col, true)

	head := float32(6) * uiScale
	angle := math.Atan2(float64(y1-y0), float64(x1-x0))

	leftX := x1 - head*float32(math.Cos(angle-math.Pi/6))
	leftY := y1 - head*float32(math.Sin(angle-math.Pi/6))
	strokeLine(screen, x1, y1, leftX, leftY, width, col, true)

	rightX := x1 - head*float32(math.Cos(angle+math.Pi/6))
	rightY := y1 - head*float32(math.Sin(angle+math.Pi/6))
	strokeLine(screen, x1, y1, rightX, rightY, width, col, true)
}

func drawFPS(screen *ebiten.Image) {
	drawFilledRect(screen, 0, 0, 58, 16, color.RGBA{R: 0, G: 0, B: 0, A: 192}, false)
	buf := fmt.Sprintf("%4v FPS", int(math.Round(ebiten.ActualFPS())))
	ebitenutil.DebugPrintAt(screen, buf, 0, 0)
}
