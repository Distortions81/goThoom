package eui

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"strings"
	"time"
	"unicode/utf8"

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

var (
	pendingDropdowns []dropdownRender
	dumpDone         bool
	// blackPixel is a reusable 1x1 black pixel.
	blackPixel = func() *ebiten.Image {
		img := ebiten.NewImage(1, 1)
		img.Fill(color.Black)
		return img
	}()
)

// Draw renders the UI to the provided screen image.
// Call this from your Ebiten Draw function.
func Draw(screen *ebiten.Image) {

	pendingDropdowns = pendingDropdowns[:0]

	// Draw main portal windows first so game content can render beneath
	// other UI elements.
	for _, win := range windows {
		if !win.open || !win.MainPortal {
			continue
		}
		if win.Dirty {
			for _, it := range win.Contents {
				markItemTreeDirty(it)
			}
		}
		win.Draw(screen)
	}

	// Draw the remaining windows on top.
	for _, win := range windows {
		if !win.open || win.MainPortal {
			continue
		}
		if win.Dirty {
			for _, it := range win.Contents {
				markItemTreeDirty(it)
			}
		}
		win.Draw(screen)
	}

	for _, ov := range overlays {
		drawOverlay(ov, screen)
	}

	for _, dr := range pendingDropdowns {
		drawDropdownOptions(dr.item, dr.offset, dr.clip, screen)
	}

	drawHoveredTooltip(screen)

	// drawFPS(screen)

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

func drawOverlay(item *itemData, screen *ebiten.Image) {
	if item == nil {
		return
	}
	offset := item.getOverlayPosition()
	clip := rect{X0: 0, Y0: 0, X1: float32(screenWidth), Y1: float32(screenHeight)}
	if item.ItemType == ITEM_FLOW {
		item.drawFlows(nil, nil, offset, clip, screen)
	} else {
		item.drawItem(nil, offset, clip, screen)
	}
}

func drawHoveredTooltip(screen *ebiten.Image) {
	if hoveredItem == nil || hoveredItem.ItemType == ITEM_FLOW || hoveredItem.Tooltip == "" {
		return
	}

	mx, my := pointerPosition()
	mpos := point{X: float32(mx), Y: float32(my)}

	textSize := hoveredItem.FontSize
	if textSize <= 0 {
		textSize = 12
	}
	textSize *= uiScale
	face := textFace(textSize)
	tw, th := text.Measure(hoveredItem.Tooltip, face, 0)
	pad := 4 * uiScale
	x := mpos.X + 8*uiScale
	y := mpos.Y + 16*uiScale
	w := float32(tw) + pad*2
	h := float32(th) + pad*2

	if x+w > float32(screenWidth) {
		x = float32(screenWidth) - w
	}
	if y+h > float32(screenHeight) {
		y = float32(screenHeight) - h
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	bg := hoveredItem.Theme.Window.HoverColor
	fg := hoveredItem.Theme.Window.TitleTextColor

	drawRoundRect(screen, &roundRect{
		Size:     point{X: w, Y: h},
		Position: point{X: x, Y: y},
		Fillet:   4 * uiScale,
		Filled:   true,
		Color:    bg,
	})

	loo := text.LayoutOptions{
		LineSpacing:    float64(textSize) * 1.2,
		PrimaryAlign:   text.AlignStart,
		SecondaryAlign: text.AlignStart,
	}
	top := acquireTextDrawOptions()
	top.DrawImageOptions.GeoM.Translate(float64(x+pad), float64(y+pad))
	top.LayoutOptions = loo
	top.ColorScale.ScaleWithColor(fg)
	text.Draw(screen, hoveredItem.Tooltip, face, top)
	releaseTextDrawOptions(top)
}

func (win *windowData) Draw(screen *ebiten.Image) {
	pos := win.getPosition()
	localPos := point{X: win.Margin * uiScale, Y: win.Margin * uiScale}

	if win.Render == nil || win.Dirty || win.itemsDirty() {
		size := win.GetSize()
		w, h := int(math.Ceil(float64(size.X))), int(math.Ceil(float64(size.Y)))
		if win.Render == nil || win.Render.Bounds().Dx() != w || win.Render.Bounds().Dy() != h {
			if win.Render != nil {
				win.Render.Deallocate()
			}
			win.Render = ebiten.NewImage(w, h)
		} else {
			win.Render.Clear()
		}
		origPos := win.Position
		win.Position = point{}
		localPos = win.getPosition()
		if !win.MainPortal {
			win.drawBG(win.Render)
		}
		win.drawItems(win.Render)
		win.drawScrollbars(win.Render)
		win.drawWinTitle(win.Render, win.getTitleRect())
		win.drawBorder(win.Render, win.getWinRect())
		win.drawDebug(win.Render)
		win.Position = origPos
		shift := pointFloor(pointSub(pos, localPos))
		shiftDrawRects(win, shift)
		win.Dirty = false
	}

	shift := pointFloor(pointSub(pos, localPos))
	if win.MainPortal {
		win.drawPortalMask(screen)
	}
	if win.Render != nil {
		op := ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(shift.X), float64(shift.Y))
		if win.Transparent {
			op.ColorScale.Scale(1, 1, 1, win.Alpha)
		}
		screen.DrawImage(win.Render, &op)
	}

	win.collectDropdowns()
}

func (win *windowData) drawPortalMask(screen *ebiten.Image) {
	r := rectFloor(win.getWinRect())
	w := float32(screenWidth)
	h := float32(screenHeight)

	op := acquireDrawImageOptions()

	if r.Y0 > 0 {
		op.GeoM.Reset()
		op.GeoM.Scale(float64(w), float64(r.Y0))
		screen.DrawImage(blackPixel, op)
	}
	if r.Y1 < h {
		op.GeoM.Reset()
		op.GeoM.Scale(float64(w), float64(h-r.Y1))
		op.GeoM.Translate(0, float64(r.Y1))
		screen.DrawImage(blackPixel, op)
	}
	if r.X0 > 0 {
		op.GeoM.Reset()
		op.GeoM.Scale(float64(r.X0), float64(r.Y1-r.Y0))
		op.GeoM.Translate(0, float64(r.Y0))
		screen.DrawImage(blackPixel, op)
	}
	if r.X1 < w {
		op.GeoM.Reset()
		op.GeoM.Scale(float64(w-r.X1), float64(r.Y1-r.Y0))
		op.GeoM.Translate(float64(r.X1), float64(r.Y0))
		screen.DrawImage(blackPixel, op)
	}
	releaseDrawImageOptions(op)
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

func (win *windowData) drawWinTitle(screen *ebiten.Image, r rect) {
	// Window Title
	if win.TitleHeight > 0 {
		drawFilledRect(screen, r.X0, r.Y0, r.X1-r.X0, r.Y1-r.Y0, win.Theme.Window.TitleBGColor, false)

		textSize := ((win.GetTitleSize()) / 2)
		face := textFace(textSize)
		win.updateTitleCache(face, textSize)

		skipTitleText := false
		textWidth, textHeight := win.titleTextW, win.titleTextH
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
			top := acquireTextDrawOptions()
			top.DrawImageOptions.GeoM.Translate(float64(r.X0+((win.GetTitleSize())/4)),
				float64(r.Y0+((win.GetTitleSize())/2)))
			top.LayoutOptions = loo

			top.ColorScale.ScaleWithColor(win.Theme.Window.TitleTextColor)
			buf := strings.ReplaceAll(win.Title, "\n", "") //Remove newline
			buf = strings.ReplaceAll(buf, "\r", "")        //Remove return
			text.Draw(screen, buf, face, top)
			releaseTextDrawOptions(top)
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
				drawFilledRect(
					screen,
					r.X0,
					r.Y0,
					r.X1-r.X0,
					r.Y1-r.Y0,
					win.Theme.Window.CloseBGColor,
					false,
				)
			}
			xThick := 1 * uiScale
			if win.HoverClose {
				color = win.Theme.Window.HoverTitleColor
				win.HoverClose = false
			}
			strokeLine(screen,
				r.X1-(win.GetTitleSize())+xpad,
				r.Y0+xpad,

				r.X1-xpad,
				r.Y0+(win.GetTitleSize())-xpad,
				xThick, color, true)
			strokeLine(screen,
				r.X1-xpad,
				r.Y0+xpad,

				r.X1-(win.GetTitleSize())+xpad,
				r.Y0+(win.GetTitleSize())-xpad,
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
					r.X0+float32(x), r.Y0+dpad,
					r.X0+float32(x), r.Y0+(win.GetTitleSize())-dpad,
					xThick, xColor, false)
			}
		}
	}
}

func (win *windowData) drawBorder(screen *ebiten.Image, r rect) {
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
			Size:     point{X: r.X1 - r.X0, Y: r.Y1 - r.Y0},
			Position: point{X: r.X0, Y: r.Y0},
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

func shiftItemRects(item *itemData, delta point) {
	item.DrawRect.X0 += delta.X
	item.DrawRect.X1 += delta.X
	item.DrawRect.Y0 += delta.Y
	item.DrawRect.Y1 += delta.Y
	for _, child := range item.Contents {
		shiftItemRects(child, delta)
	}
	for _, tab := range item.Tabs {
		shiftItemRects(tab, delta)
	}
}

func shiftDrawRects(win *windowData, delta point) {
	for _, item := range win.Contents {
		shiftItemRects(item, delta)
	}
}

func collectDropdownsFromItems(items []*itemData) {
	for _, it := range items {
		if it.ItemType == ITEM_DROPDOWN && it.Open {
			dropOff := point{X: it.DrawRect.X0, Y: it.DrawRect.Y0}
			lh := it.labelHeight()
			if lh > 0 {
				dropOff.Y += lh + currentStyle.TextPadding*uiScale
			}
			screenClip := rect{X0: 0, Y0: 0, X1: float32(screenWidth), Y1: float32(screenHeight)}
			pendingDropdowns = append(pendingDropdowns, dropdownRender{item: it, offset: dropOff, clip: screenClip})
		}

		if it.ItemType == ITEM_FLOW {
			if len(it.Tabs) > 0 {
				if it.ActiveTab >= len(it.Tabs) {
					it.ActiveTab = 0
				}
				collectDropdownsFromItems(it.Tabs[it.ActiveTab].Contents)
			} else {
				collectDropdownsFromItems(it.Contents)
			}
		} else if len(it.Contents) > 0 {
			collectDropdownsFromItems(it.Contents)
		}
	}
}

func (win *windowData) collectDropdowns() {
	collectDropdownsFromItems(win.Contents)
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
		face := textFace(textSize)
		x := offset.X
		spacing := float32(4) * uiScale
		for i, tab := range item.Tabs {
			if tab.nameImage == nil || tab.prevName != tab.Name || tab.nameFontSize != textSize {
				tw, th := text.Measure(tab.Name, face, 0)
				tab.nameWidth = float32(tw)
				tab.nameHeight = float32(th)
				iw := int(math.Ceil(float64(tab.nameWidth)))
				ih := int(math.Ceil(float64(tab.nameHeight)))
				if iw <= 0 {
					iw = 1
				}
				if ih <= 0 {
					ih = 1
				}
				if tab.nameImage != nil {
					tab.nameImage.Deallocate()
				}
				tab.nameImage = ebiten.NewImage(iw, ih)
				dto := &text.DrawOptions{}
				dto.ColorScale.ScaleWithColor(color.White)
				text.Draw(tab.nameImage, tab.Name, face, dto)
				tab.prevName = tab.Name
				tab.nameFontSize = textSize
			}
			w := tab.nameWidth + 8
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
				drawTabShape(screen,
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
				strokeTabShape(screen,
					point{X: x, Y: offset.Y},
					point{X: w, Y: tabHeight},
					style.OutlineColor,
					item.Fillet*uiScale,
					item.BorderPad*uiScale,
					border,
				)
			}
			if item.ActiveOutline && i == item.ActiveTab {
				strokeTabTop(screen,
					point{X: x, Y: offset.Y},
					point{X: w, Y: tabHeight},
					style.ClickColor,
					item.Fillet*uiScale,
					item.BorderPad*uiScale,
					3*uiScale,
				)
			}
			loo := text.LayoutOptions{PrimaryAlign: text.AlignCenter, SecondaryAlign: text.AlignCenter}
			dto := acquireTextDrawOptions()
			dto.DrawImageOptions.GeoM.Translate(float64(x+w/2), float64(offset.Y+tabHeight/2))
			dto.LayoutOptions = loo
			dto.ColorScale.ScaleWithColor(style.TextColor)
			text.Draw(screen, tab.Name, face, dto)
			releaseTextDrawOptions(dto)
			tab.DrawRect = rect{X0: x, Y0: offset.Y, X1: x + w, Y1: offset.Y + tabHeight}
			x += w + spacing
		}
		drawOffset = pointAdd(drawOffset, point{Y: tabHeight})
		drawFilledRect(screen,
			offset.X,
			offset.Y+tabHeight-3*uiScale,
			item.GetSize().X,
			3*uiScale,
			style.SelectedColor,
			false)
		strokeRect(screen,
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
			drawFilledRect(screen, item.DrawRect.X1-sbW, item.DrawRect.Y0+pos, sbW, barH, col.ToRGBA(), false)
		} else if item.FlowType == FLOW_HORIZONTAL && req.X > size.X {
			barW := size.X * size.X / req.X
			maxScroll := req.X - size.X
			pos := float32(0)
			if maxScroll > 0 {
				pos = (item.Scroll.X / maxScroll) * (size.X - barW)
			}
			col := NewColor(96, 96, 96, 192)
			sbW := currentStyle.BorderPad.Slider * 2
			drawFilledRect(screen, item.DrawRect.X0+pos, item.DrawRect.Y1-sbW, barW, sbW, col.ToRGBA(), false)
		}
	}

	if DebugMode {
		strokeRect(screen,
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
			drawArrow(screen, item.DrawRect.X0+margin, midY, item.DrawRect.X1-margin, midY, 1, col)
		case FLOW_VERTICAL:
			drawArrow(screen, midX, item.DrawRect.Y0+margin, midX, item.DrawRect.Y1-margin, 1, col)
		case FLOW_HORIZONTAL_REV:
			drawArrow(screen, item.DrawRect.X1-margin, midY, item.DrawRect.X0+margin, midY, 1, col)
		case FLOW_VERTICAL_REV:
			drawArrow(screen, midX, item.DrawRect.Y1-margin, midX, item.DrawRect.Y0+margin, 1, col)
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
	style := item.themeStyle()

	labelH := item.labelHeight()
	var labelW float32
	if item.LabelImage != nil {
		bw := float32(item.LabelImage.Bounds().Dx())
		bh := float32(item.LabelImage.Bounds().Dy())
		dw := bw
		dh := bh
		if item.LabelImageSize.X > 0 {
			dw = item.LabelImageSize.X
		}
		if item.LabelImageSize.Y > 0 {
			dh = item.LabelImageSize.Y
		}
		sop := acquireDrawImageOptions()
		sop.GeoM.Scale(float64(dw/bw*uiScale), float64(dh/bh*uiScale))
		sop.GeoM.Translate(float64(offset.X), float64(offset.Y+(labelH-dh*uiScale)/2))
		screen.DrawImage(item.LabelImage, sop)
		releaseDrawImageOptions(sop)
		labelW = dw * uiScale
	}
	if item.Label != "" {
		textSize := (item.FontSize * uiScale) + 2
		face := textFace(textSize)
		loo := text.LayoutOptions{PrimaryAlign: text.AlignStart, SecondaryAlign: text.AlignCenter}
		top := acquireTextDrawOptions()
		top.DrawImageOptions.GeoM.Translate(float64(offset.X+labelW+currentStyle.TextPadding*uiScale), float64(offset.Y+labelH/2))
		top.LayoutOptions = loo
		if style != nil {
			top.ColorScale.ScaleWithColor(style.TextColor)
		}
		text.Draw(screen, item.Label, face, top)
		releaseTextDrawOptions(top)
	}
	if labelH > 0 {
		offset.Y += labelH + currentStyle.TextPadding*uiScale
		maxSize.Y -= labelH + currentStyle.TextPadding*uiScale
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
			drawRoundRect(screen, &roundRect{
				Size:     auxSize,
				Position: offset,
				Fillet:   item.Fillet,
				Filled:   true,
				Color:    itemColor,
			})
		}
		drawRoundRect(screen, &roundRect{
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

			drawCheckmark(screen, start, mid, end, cThick, style.TextColor)
		}

		textSize := (item.FontSize * uiScale) + 2
		face := textFace(textSize)
		loo := text.LayoutOptions{
			LineSpacing:    1.2,
			PrimaryAlign:   text.AlignStart,
			SecondaryAlign: text.AlignCenter,
		}
		top := acquireTextDrawOptions()
		top.DrawImageOptions.GeoM.Translate(
			float64(offset.X+auxSize.X+item.AuxSpace),
			float64(offset.Y+(auxSize.Y/2)),
		)
		top.LayoutOptions = loo
		top.ColorScale.ScaleWithColor(style.TextColor)
		text.Draw(screen, item.Text, face, top)
		releaseTextDrawOptions(top)

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
			drawRoundRect(screen, &roundRect{
				Size:     auxSize,
				Position: offset,
				Fillet:   auxSize.X / 2,
				Filled:   true,
				Color:    itemColor,
			})
		}
		drawRoundRect(screen, &roundRect{
			Size:     auxSize,
			Position: offset,
			Fillet:   auxSize.X / 2,
			Filled:   false,
			Color:    bColor,
			Border:   bThick,
		})
		if item.Checked {
			inner := auxSize.X / 2.5
			drawRoundRect(screen, &roundRect{
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
		top := acquireTextDrawOptions()
		top.DrawImageOptions.GeoM.Translate(
			float64(offset.X+auxSize.X+item.AuxSpace),
			float64(offset.Y+(auxSize.Y/2)),
		)
		top.LayoutOptions = loo
		top.ColorScale.ScaleWithColor(style.TextColor)
		text.Draw(screen, item.Text, face, top)
		releaseTextDrawOptions(top)

	} else if item.ItemType == ITEM_BUTTON {

		if item.Image != nil {
			sop := acquireDrawImageOptions()
			sop.GeoM.Scale(float64(maxSize.X)/float64(item.Image.Bounds().Dx()),
				float64(maxSize.Y)/float64(item.Image.Bounds().Dy()))
			sop.GeoM.Translate(float64(offset.X), float64(offset.Y))
			screen.DrawImage(item.Image, sop)
			releaseDrawImageOptions(sop)
		} else {
			itemColor := style.Color
			if time.Since(item.Clicked) < clickFlash {
				itemColor = style.ClickColor
			} else if item.Hovered {
				item.Hovered = false
				itemColor = style.HoverColor
			}
			if item.Filled {
				drawRoundRect(screen, &roundRect{
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
		top := acquireTextDrawOptions()
		top.DrawImageOptions.GeoM.Translate(
			float64(offset.X+((maxSize.X)/2)),
			float64(offset.Y+((maxSize.Y)/2)))
		top.LayoutOptions = loo
		top.ColorScale.ScaleWithColor(style.TextColor)
		text.Draw(screen, item.Text, face, top)
		releaseTextDrawOptions(top)

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
			drawRoundRect(screen, &roundRect{
				Size:     maxSize,
				Position: offset,
				Fillet:   item.Fillet,
				Filled:   true,
				Color:    itemColor,
			})
		}

		var eyeRect rect
		if item.Hide {
			eyeSize := maxSize.Y - (item.BorderPad+item.Padding)*2
			if eyeSize < 0 {
				eyeSize = 0
			}
			eyeRect = rect{
				X0: offset.X + maxSize.X - eyeSize - item.BorderPad - item.Padding,
				Y0: offset.Y + (maxSize.Y-eyeSize)/2,
				X1: offset.X + maxSize.X - item.BorderPad - item.Padding,
				Y1: offset.Y + (maxSize.Y-eyeSize)/2 + eyeSize,
			}
		}

		disp := item.Text
		if item.Hide && !item.Reveal {
			n := utf8.RuneCountInString(item.Text)
			disp = strings.Repeat("*", n)
		}

		textSize := (item.FontSize * uiScale) + 2
		face := textFace(textSize)
		loo := text.LayoutOptions{
			LineSpacing:    0,
			PrimaryAlign:   text.AlignStart,
			SecondaryAlign: text.AlignCenter,
		}
		top := acquireTextDrawOptions()
		top.DrawImageOptions.GeoM.Translate(
			float64(offset.X+item.BorderPad+item.Padding+currentStyle.TextPadding*uiScale),
			float64(offset.Y+((maxSize.Y)/2)),
		)
		top.LayoutOptions = loo
		top.ColorScale.ScaleWithColor(style.TextColor)
		text.Draw(screen, disp, face, top)
		releaseTextDrawOptions(top)

		if item.Hide {
			drawEye(screen, eyeRect, style.TextColor)
		}

		if item.Focused {
			width, _ := text.Measure(disp, face, 0)
			cx := offset.X + item.BorderPad + item.Padding + currentStyle.TextPadding*uiScale + float32(width)
			strokeLine(screen,
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
		// slider track remains consistent length. Use a constant max
		// label so sliders have consistent track lengths regardless of
		// their numeric range.
		maxLabel := sliderMaxLabel
		textSize := (item.FontSize * uiScale) + 2
		face := textFace(textSize)
		maxW, maxH := text.Measure(maxLabel, face, 0)

		gap := currentStyle.SliderValueGap
		knobW := item.AuxSize.X * uiScale
		knobH := item.AuxSize.Y * uiScale

		ratio := 0.0
		var valueLog float64
		if item.MaxValue > item.MinValue {
			if item.Log && item.MinValue > 0 && item.MaxValue > 0 {
				minLog := math.Log(float64(item.MinValue)) / math.Log(float64(item.LogValue))
				maxLog := math.Log(float64(item.MaxValue)) / math.Log(float64(item.LogValue))
				valueLog = math.Log(float64(item.Value)) / math.Log(float64(item.LogValue))
				ratio = (valueLog - minLog) / (maxLog - minLog)
			} else {
				ratio = float64((item.Value - item.MinValue) / (item.MaxValue - item.MinValue))
			}
		}
		if ratio < 0 {
			ratio = 0
		} else if ratio > 1 {
			ratio = 1
		}

		valueText := fmt.Sprintf("%.2f", item.Value)
		if item.IntOnly {
			width := len(maxLabel)
			valueText = fmt.Sprintf("%*d", width, int(item.Value))
		}

		if item.Vertical {
			trackHeight := maxSize.Y - knobH - gap - float32(maxH)
			if trackHeight < 0 {
				trackHeight = 0
			}

			trackX := offset.X + maxSize.X/2
			trackTop := offset.Y + knobH/2
			trackBottom := trackTop + trackHeight
			knobCenter := trackBottom - float32(ratio)*trackHeight
			filledCol := style.SelectedColor
			strokeLine(screen, trackX, trackBottom, trackX, knobCenter, 2*uiScale, filledCol, true)
			strokeLine(screen, trackX, knobCenter, trackX, trackTop, 2*uiScale, itemColor, true)
			knobRect := point{X: offset.X + (maxSize.X-knobW)/2, Y: knobCenter - knobH/2}
			drawRoundRect(screen, &roundRect{
				Size:     pointScaleMul(item.AuxSize),
				Position: knobRect,
				Fillet:   item.Fillet,
				Filled:   true,
				Color:    style.Color,
			})
			drawRoundRect(screen, &roundRect{
				Size:     pointScaleMul(item.AuxSize),
				Position: knobRect,
				Fillet:   item.Fillet,
				Filled:   false,
				Border:   1 * uiScale,
				Color:    style.OutlineColor,
			})

			// value text drawn below the slider track
			loo := text.LayoutOptions{LineSpacing: 1.2, PrimaryAlign: text.AlignCenter, SecondaryAlign: text.AlignStart}
			top := acquireTextDrawOptions()
			top.DrawImageOptions.GeoM.Translate(
				float64(offset.X+maxSize.X/2),
				float64(trackBottom+gap),
			)
			top.LayoutOptions = loo
			top.ColorScale.ScaleWithColor(style.TextColor)
			text.Draw(screen, valueText, face, top)
			releaseTextDrawOptions(top)

		} else {
			trackWidth := maxSize.X - knobW - gap - float32(maxW)
			if trackWidth < 0 {
				trackWidth = 0
			}

			trackStart := offset.X + knobW/2
			trackY := offset.Y + maxSize.Y/2
			knobCenter := trackStart + float32(ratio)*trackWidth
			filledCol := style.SelectedColor
			strokeLine(screen, trackStart, trackY, knobCenter, trackY, 2*uiScale, filledCol, true)
			strokeLine(screen, knobCenter, trackY, trackStart+trackWidth, trackY, 2*uiScale, itemColor, true)
			knobRect := point{X: knobCenter - knobW/2, Y: offset.Y + (maxSize.Y-knobH)/2}
			drawRoundRect(screen, &roundRect{
				Size:     pointScaleMul(item.AuxSize),
				Position: knobRect,
				Fillet:   item.Fillet,
				Filled:   true,
				Color:    style.Color,
			})
			drawRoundRect(screen, &roundRect{
				Size:     pointScaleMul(item.AuxSize),
				Position: knobRect,
				Fillet:   item.Fillet,
				Filled:   false,
				Border:   1 * uiScale,
				Color:    style.OutlineColor,
			})

			// value text drawn to the right of the slider track
			loo := text.LayoutOptions{LineSpacing: 1.2, PrimaryAlign: text.AlignStart, SecondaryAlign: text.AlignCenter}
			top := acquireTextDrawOptions()
			top.DrawImageOptions.GeoM.Translate(
				float64(trackStart+trackWidth+gap),
				float64(offset.Y+(maxSize.Y/2)),
			)
			top.LayoutOptions = loo
			top.ColorScale.ScaleWithColor(style.TextColor)
			text.Draw(screen, valueText, face, top)
			releaseTextDrawOptions(top)
		}

	} else if item.ItemType == ITEM_DROPDOWN {

		itemColor := style.Color
		if item.Open {
			itemColor = style.SelectedColor
		} else if item.Hovered {
			item.Hovered = false
			itemColor = style.HoverColor
		}

		if item.Filled {
			drawRoundRect(screen, &roundRect{
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
		top := acquireTextDrawOptions()
		top.DrawImageOptions.GeoM.Translate(float64(offset.X+item.BorderPad+item.Padding+currentStyle.TextPadding*uiScale), float64(offset.Y+maxSize.Y/2))
		top.LayoutOptions = loo
		top.ColorScale.ScaleWithColor(style.TextColor)
		label := item.Text
		if item.Selected >= 0 && item.Selected < len(item.Options) {
			label = item.Options[item.Selected]
		}
		text.Draw(screen, label, face, top)
		releaseTextDrawOptions(top)

		arrow := maxSize.Y * 0.4
		drawTriangle(screen,
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
		op := acquireDrawImageOptions()
		op.GeoM.Translate(float64(offset.X), float64(offset.Y))
		screen.DrawImage(item.Image, op)
		releaseDrawImageOptions(op)

		h, _, v, _ := rgbaToHSVA(color.RGBA(item.WheelColor))
		radius := wheelSize / 2
		cx := offset.X + radius
		cy := offset.Y + radius
		px := cx + float32(math.Cos(h*math.Pi/180))*radius*float32(v)
		py := cy + float32(math.Sin(h*math.Pi/180))*radius*float32(v)
		vector.DrawFilledCircle(screen, px, py, 4*uiScale, color.Black, true)
		vector.DrawFilledCircle(screen, px, py, 2*uiScale, color.White, true)

		sw := wheelSize / 5
		if sw < 10*uiScale {
			sw = 10 * uiScale
		}
		sx := offset.X + wheelSize + 4*uiScale
		sy := offset.Y + maxSize.Y - sw - 4*uiScale
		drawFilledRect(screen, sx, sy, sw, sw, color.RGBA(item.WheelColor), true)
		strokeRect(screen, sx, sy, sw, sw, 1, color.Black, true)

	} else if item.ItemType == ITEM_TEXT {

		textSize := (item.FontSize * uiScale) + 2
		face := textFace(textSize)
		loo := text.LayoutOptions{
			LineSpacing:    float64(textSize) * 1.2,
			PrimaryAlign:   text.AlignStart,
			SecondaryAlign: text.AlignStart,
		}
		top := acquireTextDrawOptions()
		top.DrawImageOptions.GeoM.Translate(
			float64(offset.X),
			float64(offset.Y))

		top.LayoutOptions = loo
		top.ColorScale.ScaleWithColor(style.TextColor)
		text.Draw(screen, item.Text, face, top)
		releaseTextDrawOptions(top)
	}

	if item.Outlined && item.Border > 0 && item.ItemType != ITEM_CHECKBOX && item.ItemType != ITEM_RADIO {
		drawRoundRect(screen, &roundRect{
			Size:     maxSize,
			Position: offset,
			Fillet:   item.Fillet,
			Filled:   false,
			Color:    style.OutlineColor,
			Border:   item.Border * uiScale,
		})
	}

	if DebugMode {
		strokeRect(screen,
			item.DrawRect.X0,
			item.DrawRect.Y0,
			item.DrawRect.X1-item.DrawRect.X0,
			item.DrawRect.Y1-item.DrawRect.Y0,
			1, color.RGBA{R: 128}, false)
	}

}

func (item *itemData) ensureRender() {
	if item.ItemType == ITEM_FLOW {
		return
	}
	size := item.GetSize()
	w, h := int(math.Ceil(float64(size.X))), int(math.Ceil(float64(size.Y)))
	if item.Render == nil || item.Render.Bounds().Dx() != w || item.Render.Bounds().Dy() != h {
		item.Render = ebiten.NewImage(w, h)
		item.Dirty = true
	}
	if item.ItemType == ITEM_INPUT {
		if item.Hide != item.prevHide || item.Reveal != item.prevReveal || item.Text != item.prevText {
			item.Dirty = true
		}
		item.prevHide = item.Hide
		item.prevReveal = item.Reveal
		item.prevText = item.Text
	}
	if !item.Dirty {
		return
	}
	prevRect := item.DrawRect
	prevHover := item.Hovered
	item.Render.Clear()
	item.drawItemInternal(nil, point{}, rect{X0: 0, Y0: 0, X1: size.X, Y1: size.Y}, item.Render)
	if DebugMode {
		item.RenderCount++
		ebitenutil.DebugPrintAt(item.Render, fmt.Sprintf("%d", item.RenderCount), 0, 0)
	}
	item.DrawRect = prevRect
	item.Hovered = prevHover
	item.Dirty = false
}

func (item *itemData) drawItem(parent *itemData, offset point, clip rect, screen *ebiten.Image) {
	if item.ItemType != ITEM_FLOW {
		item.ensureRender()

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
		sub := item.Render.SubImage(src).(*ebiten.Image)
		op := acquireDrawImageOptions()
		op.GeoM.Translate(float64(item.DrawRect.X0), float64(item.DrawRect.Y0))
		screen.DrawImage(sub, op)
		releaseDrawImageOptions(op)

		if item.ItemType == ITEM_DROPDOWN && item.Open {
			dropOff := offset
			lh := item.labelHeight()
			if lh > 0 {
				dropOff.Y += lh + currentStyle.TextPadding*uiScale
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
	style := item.themeStyle()
	drawFilledRect(screen,
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
			drawRoundRect(screen, &roundRect{Size: maxSize, Position: point{X: offset.X, Y: y}, Fillet: item.Fillet, Filled: true, Color: col})
		}
		tdo := acquireTextDrawOptions()
		tdo.DrawImageOptions.GeoM.Translate(float64(offset.X+item.BorderPad+item.Padding+currentStyle.TextPadding*uiScale), float64(y+optionH/2))
		tdo.LayoutOptions = loo
		tdo.ColorScale.ScaleWithColor(style.TextColor)
		text.Draw(screen, item.Options[i], face, tdo)
		releaseTextDrawOptions(tdo)
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
		drawFilledRect(screen, drawRect.X1-sbW, startY+pos, sbW, barH, col.ToRGBA(), false)
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
	var path vector.Path
	vertices := getVertices()
	indices := getIndices()
	defer func() {
		putVertices(vertices)
		putIndices(indices)
	}()

	width := float32(math.Round(float64(rrect.Border)))
	off := float32(0)
	if !rrect.Filled {
		off = pixelOffset(width)
	}

	x := float32(math.Round(float64(rrect.Position.X))) + off
	y := float32(math.Round(float64(rrect.Position.Y))) + off
	x1 := float32(math.Round(float64(rrect.Position.X+rrect.Size.X))) - off
	y1 := float32(math.Round(float64(rrect.Position.Y+rrect.Size.Y))) - off
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
		vertices, indices = path.AppendVerticesAndIndicesForFilling(vertices, indices)
	} else {
		opv := &vector.StrokeOptions{Width: width}
		vertices, indices = path.AppendVerticesAndIndicesForStroke(vertices, indices, opv)
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
	var path vector.Path
	vertices := getVertices()
	indices := getIndices()
	defer func() {
		putVertices(vertices)
		putIndices(indices)
	}()

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

	vertices, indices = path.AppendVerticesAndIndicesForFilling(vertices, indices)
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
	var path vector.Path
	vertices := getVertices()
	indices := getIndices()
	defer func() {
		putVertices(vertices)
		putIndices(indices)
	}()

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
	vertices, indices = path.AppendVerticesAndIndicesForStroke(vertices, indices, opv)
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
	var path vector.Path
	vertices := getVertices()
	indices := getIndices()
	defer func() {
		putVertices(vertices)
		putIndices(indices)
	}()

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
	vertices, indices = path.AppendVerticesAndIndicesForStroke(vertices, indices, opv)
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
	var path vector.Path
	vertices := getVertices()
	indices := getIndices()
	defer func() {
		putVertices(vertices)
		putIndices(indices)
	}()

	// Quantize to pixel boundaries
	pos.X = float32(math.Round(float64(pos.X)))
	pos.Y = float32(math.Round(float64(pos.Y)))
	size = float32(math.Round(float64(size)))

	path.MoveTo(pos.X, pos.Y)
	path.LineTo(pos.X+size, pos.Y)
	path.LineTo(pos.X+size/2, pos.Y+size)
	path.Close()

	vertices, indices = path.AppendVerticesAndIndicesForFilling(vertices, indices)
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

func drawEye(screen *ebiten.Image, r rect, col Color) {
	w := r.X1 - r.X0
	h := r.Y1 - r.Y0
	drawRoundRect(screen, &roundRect{
		Size:     point{X: w, Y: h},
		Position: point{X: r.X0, Y: r.Y0},
		Fillet:   h / 2,
		Filled:   false,
		Color:    col,
		Border:   1 * uiScale,
	})
	pupil := h / 2
	drawRoundRect(screen, &roundRect{
		Size:     point{X: pupil, Y: pupil},
		Position: point{X: r.X0 + w/2 - pupil/2, Y: r.Y0 + h/2 - pupil/2},
		Fillet:   pupil / 2,
		Filled:   true,
		Color:    col,
	})
}

func drawCheckmark(screen *ebiten.Image, start, mid, end point, width float32, col Color) {
	var path vector.Path
	vertices := getVertices()
	indices := getIndices()
	defer func() {
		putVertices(vertices)
		putIndices(indices)
	}()

	width = float32(math.Round(float64(width)))
	off := pixelOffset(width)

	path.MoveTo(float32(math.Round(float64(start.X)))+off, float32(math.Round(float64(start.Y)))+off)
	path.LineTo(float32(math.Round(float64(mid.X)))+off, float32(math.Round(float64(mid.Y)))+off)
	path.LineTo(float32(math.Round(float64(end.X)))+off, float32(math.Round(float64(end.Y)))+off)

	opv := &vector.StrokeOptions{Width: width, LineJoin: vector.LineJoinRound, LineCap: vector.LineCapRound}
	vertices, indices = path.AppendVerticesAndIndicesForStroke(vertices, indices, opv)
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
