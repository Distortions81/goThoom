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

var dumpDone bool
var zoneIndicatorWin *windowData

type openDropdown struct {
	item   *itemData
	offset point
}

// Draw renders the UI to the provided screen image.
// Call this from your Ebiten Draw function.
func Draw(screen *ebiten.Image) {
	zoneIndicatorWin = nil
	dropdowns := []openDropdown{}
	for _, win := range windows {
		if !win.Open {
			continue
		}
		// If a window contains an indeterminate progress bar, force a repaint
		// so the barber-pole animation advances even without data events.
		if !win.Dirty && windowHasIndeterminateProgress(win) {
			win.Dirty = true
		}
		win.Draw(screen, &dropdowns)
	}

	if dragPart == PART_BAR && dragWin != nil {
		zoneIndicatorWin = dragWin
	}

	if zoneIndicatorWin != nil {
		drawZoneOverlay(screen, zoneIndicatorWin)
	}

	screenClip := rect{X0: 0, Y0: 0, X1: float32(screenWidth), Y1: float32(screenHeight)}
	for _, dd := range dropdowns {
		drawDropdownOptions(dd.item, dd.offset, screenClip, screen)
	}

	if hoveredItem != nil && hoveredItem.Tooltip != "" {
		drawTooltip(screen, hoveredItem)
	}

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

// windowHasIndeterminateProgress reports whether win contains any indeterminate
// progress bar that requires continuous repaint to animate.
func windowHasIndeterminateProgress(win *windowData) bool {
	return itemsHaveIndeterminateProgress(win.Contents)
}

func itemsHaveIndeterminateProgress(items []*itemData) bool {
	for _, it := range items {
		if it.ItemType == ITEM_PROGRESS && it.Indeterminate {
			return true
		}
		if len(it.Tabs) > 0 {
			if it.ActiveTab >= len(it.Tabs) {
				// Clamp defensively
				if len(it.Tabs) > 0 {
					it.ActiveTab = 0
				}
			}
			if it.ActiveTab >= 0 && it.ActiveTab < len(it.Tabs) {
				if itemsHaveIndeterminateProgress(it.Tabs[it.ActiveTab].Contents) {
					return true
				}
			}
		}
		if itemsHaveIndeterminateProgress(it.Contents) {
			return true
		}
	}
	return false
}

func drawZoneOverlay(screen *ebiten.Image, win *windowData) {
	size := float32(20) * uiScale
	fillet := size / 4
	dark := color.NRGBA{R: 0x40, G: 0x40, B: 0x40, A: 64}
	red := color.NRGBA{R: 0xFF, G: 0x00, B: 0x00, A: 64}

	pos := win.getPosition()
	winSize := win.GetSize()
	hSel := nearestHZone(pos.X, winSize.X, screenWidth)
	vSel := nearestVZone(pos.Y, winSize.Y, screenHeight)

	for h := HZoneLeft; h <= HZoneRight; h++ {
		for v := VZoneTop; v <= VZoneBottom; v++ {
			x := hZoneCoord(h, screenWidth)
			y := vZoneCoord(v, screenHeight)
			col := dark
			if h == hSel && v == vSel {
				col = red
			}
			rr := roundRect{
				Size:     point{X: size, Y: size},
				Position: point{X: x - size/2, Y: y - size/2},
				Fillet:   fillet,
				Filled:   true,
				Color:    Color{R: col.R, G: col.G, B: col.B, A: col.A},
			}
			drawRoundRect(screen, &rr)
		}
	}
}

func drawTooltip(screen *ebiten.Image, item *itemData) {
	faceSize := float32(12) * uiScale
	face := textFace(faceSize)
	w, h := text.Measure(item.Tooltip, face, 0)
	pad := float32(4) * uiScale
	width := float32(w) + pad*2
	height := float32(h) + pad*2

	x := item.DrawRect.X0
	y := item.DrawRect.Y1 + pad
	if y+height > float32(screenHeight) {
		y = item.DrawRect.Y0 - height - pad
		if y < 0 {
			y = 0
		}
	}
	if x+width > float32(screenWidth) {
		x = float32(screenWidth) - width
	}

	style := item.themeStyle()
	bg := color.RGBA{0, 0, 0, 200}
	fg := color.RGBA{255, 255, 255, 255}
	border := color.RGBA{255, 255, 255, 255}
	if style != nil {
		bg = style.HoverColor.ToRGBA()
		fg = style.TextColor.ToRGBA()
		border = style.OutlineColor.ToRGBA()
	}

	drawFilledRect(screen, x, y, width, height, bg, true)
	strokeRect(screen, x, y, width, height, 1, border, true)

	dop := ebiten.DrawImageOptions{}
	dop.GeoM.Translate(float64(x+pad), float64(y+pad))
	top := &text.DrawOptions{DrawImageOptions: dop}
	top.ColorScale.ScaleWithColor(fg)
	text.Draw(screen, item.Tooltip, face, top)
}

func (win *windowData) Draw(screen *ebiten.Image, dropdowns *[]openDropdown) {
	if win.Dirty || win.Render == nil {
		if CacheCheck {
			win.RenderCount++
		}
		size := win.GetSize()
		if win.Render == nil || win.Render.Bounds().Dx() != int(size.X) || win.Render.Bounds().Dy() != int(size.Y) {
			if size.X < 1 || size.Y < 1 {
				return
			}
			win.Render = newImage(int(size.X), int(size.Y))
		} else {
			win.Render.Clear()
		}
		origPos := win.Position
		win.Position = point{}
		win.drawBG(win.Render)
		win.drawItems(win.Render, origPos, dropdowns)
		win.drawScrollbars(win.Render)
		titleArea := win.Render.SubImage(win.getTitleRect().getRectangle()).(*ebiten.Image)
		win.drawWinTitle(titleArea)
		windowArea := win.Render.SubImage(win.getWinRect().getRectangle()).(*ebiten.Image)
		win.drawBorder(windowArea)
		win.Position = origPos
		win.Dirty = false
	} else {
		win.collectDropdowns(dropdowns)
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(win.getPosition().X), float64(win.getPosition().Y))
	screen.DrawImage(win.Render, op)
	win.drawDebug(screen)
	if CacheCheck {
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%d", win.RenderCount), int(win.getPosition().X), int(win.getPosition().Y))
	}
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
	if win.NoBGColor {
		return
	}
	r := rect{
		X0: win.getPosition().X + win.BorderPad*win.scale(),
		Y0: win.getPosition().Y + win.BorderPad*win.scale(),
		X1: win.getPosition().X + win.GetSize().X - win.BorderPad*win.scale(),
		Y1: win.getPosition().Y + win.GetSize().Y - win.BorderPad*win.scale(),
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

		//Close X and Pin icon
		var buttonsWidth float32 = 0
		if win.Closable {
			var xpad float32 = (win.GetTitleSize()) / 3.0
			color := win.Theme.Window.TitleColor
			if win.Theme.Window.CloseBGColor.A > 0 {
				r := win.xRect()
				closeArea := screen.SubImage(r.getRectangle()).(*ebiten.Image)
				closeArea.Fill(win.Theme.Window.CloseBGColor)
			}
			xThick := 1 * win.scale()
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

		// Pin icon
		{
			pr := win.pinRect()
			color := win.Theme.Window.TitleColor
			if win.zone == nil {
				if c, ok := namedColors["disabled"]; ok {
					color = c
				} else {
					color = ColorGray
				}
			}
			if win.HoverPin {
				color = win.Theme.Window.HoverTitleColor
				win.HoverPin = false
			}
			radius := win.GetTitleSize() / 6
			cx := pr.X0 + (pr.X1-pr.X0)/2
			cy := pr.Y0 + (pr.Y1-pr.Y0)/2
			vector.DrawFilledCircle(screen, cx, cy-radius/2, radius, color, true)
			if win.zone != nil {
				strokeLine(screen, cx, cy-radius/2, cx, pr.Y1-radius/3, uiScale, color, true)
			} else {
				strokeLine(screen, cx, cy-radius/2, cx+radius, pr.Y1-radius/3, uiScale, color, true)
			}
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
			for x := textWidth + float64((win.GetTitleSize())/1.5); x < float64(win.GetSize().X-buttonsWidth); x = x + float64(win.scale()*spacing) {
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
	if win.Resizable {
		win.drawResizeThumb(screen)
	}
}

func (win *windowData) drawResizeThumb(screen *ebiten.Image) {
	size := float32(12) * uiScale
	step := float32(4) * uiScale
	pad := win.BorderPad * win.scale()
	x1 := win.GetSize().X - pad
	y1 := win.GetSize().Y - pad

	col := win.Theme.Window.BorderColor
	if activeWindow == win {
		col = win.Theme.Window.ActiveColor
	} else if win.Hovered {
		col = win.Theme.Window.HoverColor
	}

	x0 := x1 - size
	y0 := y1 - size

	strokeLine(screen, x0, y1, x1, y1, uiScale, col, true)
	strokeLine(screen, x1, y0, x1, y1, uiScale, col, true)
	strokeLine(screen, x0, y1, x1, y0, uiScale, col, true)

	for off := step; off < size; off += step {
		strokeLine(screen, x1-off, y1, x1, y1-off, uiScale, col, true)
	}
}

func (win *windowData) drawScrollbars(screen *ebiten.Image) {
	if win.NoScroll {
		return
	}
	pad := (win.Padding + win.BorderPad) * win.scale()
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
			Position: point{X: win.getPosition().X + win.GetSize().X - win.BorderPad*win.scale() - sbW, Y: win.getPosition().Y + win.GetTitleSize() + win.BorderPad*win.scale() + pos},
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
			Position: point{X: win.getPosition().X + win.BorderPad*win.scale() + pos, Y: win.getPosition().Y + win.GetSize().Y - win.BorderPad*win.scale() - sbW},
			Fillet:   currentStyle.Fillet.Slider,
			Filled:   true,
			Color:    win.Theme.Window.ActiveColor,
		})
	}
}

func (win *windowData) drawItems(screen *ebiten.Image, base point, dropdowns *[]openDropdown) {
	pad := (win.Padding + win.BorderPad) * win.scale()
	winPos := point{X: pad, Y: win.GetTitleSize() + pad}
	winPos = pointSub(winPos, win.Scroll)
	clip := win.getMainRect()

	for _, item := range win.Contents {
		itemPos := pointAdd(winPos, item.getPosition(win))

		if item.ItemType == ITEM_FLOW {
			item.drawFlows(win, nil, itemPos, base, clip, screen, dropdowns)
		} else {
			item.drawItem(nil, itemPos, base, clip, screen, dropdowns)
		}
	}
}

func (item *itemData) drawFlows(win *windowData, parent *itemData, offset point, base point, clip rect, screen *ebiten.Image, dropdowns *[]openDropdown) {
	if CacheCheck {
		item.RenderCount++
	}
	itemRect := rect{
		X0: offset.X,
		Y0: offset.Y,
		X1: offset.X + item.GetSize().X,
		Y1: offset.Y + item.GetSize().Y,
	}
	drawRect := intersectRect(itemRect, clip)

	if drawRect.X1 <= drawRect.X0 || drawRect.Y1 <= drawRect.Y0 {
		item.DrawRect = rectAdd(drawRect, base)
		return
	}
	subImg := screen.SubImage(drawRect.getRectangle()).(*ebiten.Image)
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
			// Use window-aware scaled position to handle NoScale windows correctly.
			flowPos := pointAdd(drawOffset, item.getPosition(win))
			flowOff := pointAdd(flowPos, flowOffset)
			itemPos := pointAdd(flowOff, subItem.getPosition(win))
			subRect := rect{
				X0: itemPos.X,
				Y0: itemPos.Y,
				X1: itemPos.X + subItem.GetSize().X,
				Y1: itemPos.Y + subItem.GetSize().Y,
			}
			inter := intersectRect(subRect, drawRect)
			if inter.X1 <= inter.X0 || inter.Y1 <= inter.Y0 {
				subItem.DrawRect = rectAdd(inter, base)
			} else {
				subItem.drawFlows(win, item, itemPos, base, drawRect, screen, dropdowns)
			}
		} else {
			flowOff := pointAdd(drawOffset, flowOffset)

			if subItem.PinTo != PIN_TOP_LEFT {
				pad := (win.Padding + win.BorderPad) * win.scale()
				objOff := pointAdd(win.getPosition(), point{X: pad, Y: win.GetTitleSize() + pad})
				objOff = pointSub(objOff, win.Scroll)
				objOff = pointAdd(objOff, subItem.getPosition(win))
				subRect := rect{
					X0: objOff.X,
					Y0: objOff.Y,
					X1: objOff.X + subItem.GetSize().X,
					Y1: objOff.Y + subItem.GetSize().Y,
				}
				inter := intersectRect(subRect, drawRect)
				if inter.X1 <= inter.X0 || inter.Y1 <= inter.Y0 {
					subItem.DrawRect = rectAdd(inter, base)
				} else {
					clipWin := win.getMainRect()
					subItem.drawItem(item, objOff, base, clipWin, screen, dropdowns)
				}
			} else {
				objOff := flowOff
				if parent != nil && parent.ItemType == ITEM_FLOW {
					objOff = pointAdd(objOff, subItem.getPosition(win))
				}
				subRect := rect{
					X0: objOff.X,
					Y0: objOff.Y,
					X1: objOff.X + subItem.GetSize().X,
					Y1: objOff.Y + subItem.GetSize().Y,
				}
				inter := intersectRect(subRect, drawRect)
				if inter.X1 <= inter.X0 || inter.Y1 <= inter.Y0 {
					subItem.DrawRect = rectAdd(inter, base)
				} else {
					subItem.drawItem(item, objOff, base, drawRect, screen, dropdowns)
				}
			}
		}

		if item.ItemType == ITEM_FLOW {
			if item.FlowType == FLOW_HORIZONTAL {
				flowOffset = pointAdd(flowOffset, point{X: subItem.GetSize().X, Y: 0})
				flowOffset = pointAdd(flowOffset, point{X: subItem.getPosition(win).X})
			} else if item.FlowType == FLOW_VERTICAL {
				flowOffset = pointAdd(flowOffset, point{X: 0, Y: subItem.GetSize().Y})
				flowOffset = pointAdd(flowOffset, point{Y: subItem.getPosition(win).Y})
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
			drawFilledRect(subImg, drawRect.X1-sbW, drawRect.Y0+pos, sbW, barH, col.ToRGBA(), false)
		} else if item.FlowType == FLOW_HORIZONTAL && req.X > size.X {
			barW := size.X * size.X / req.X
			maxScroll := req.X - size.X
			pos := float32(0)
			if maxScroll > 0 {
				pos = (item.Scroll.X / maxScroll) * (size.X - barW)
			}
			col := NewColor(96, 96, 96, 192)
			sbW := currentStyle.BorderPad.Slider * 2
			drawFilledRect(subImg, drawRect.X0+pos, drawRect.Y1-sbW, barW, sbW, col.ToRGBA(), false)
		}
	}

	if DebugMode {
		strokeRect(subImg,
			drawRect.X0,
			drawRect.Y0,
			drawRect.X1-drawRect.X0,
			drawRect.Y1-drawRect.Y0,
			1,
			Color{G: 255},
			false)

		midX := (drawRect.X0 + drawRect.X1) / 2
		midY := (drawRect.Y0 + drawRect.Y1) / 2
		margin := float32(4) * uiScale
		col := Color{B: 255, A: 255}

		switch item.FlowType {
		case FLOW_HORIZONTAL:
			drawArrow(subImg, drawRect.X0+margin, midY, drawRect.X1-margin, midY, 1, col)
		case FLOW_VERTICAL:
			drawArrow(subImg, midX, drawRect.Y0+margin, midX, drawRect.Y1-margin, 1, col)
		case FLOW_HORIZONTAL_REV:
			drawArrow(subImg, drawRect.X1-margin, midY, drawRect.X0+margin, midY, 1, col)
		case FLOW_VERTICAL_REV:
			drawArrow(subImg, midX, drawRect.Y1-margin, midX, drawRect.Y0+margin, 1, col)
		}
	}
	if CacheCheck {
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%d", item.RenderCount), int(drawRect.X0), int(drawRect.Y0))
	}
	item.DrawRect = rectAdd(drawRect, base)
}

func (item *itemData) drawItemInternal(parent *itemData, offset point, base point, clip rect, screen *ebiten.Image) {

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
		item.DrawRect = rectAdd(item.DrawRect, base)
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
		showValue := true
		if trackWidth < knobW {
			trackWidth = maxSize.X - knobW
			showValue = false
			if trackWidth < 0 {
				trackWidth = 0
			}
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

		if showValue {
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
		}

	} else if item.ItemType == ITEM_DROPDOWN {

		itemColor := style.Color
		if item.Open {
			itemColor = style.SelectedColor
		} else if item.Hovered {
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

	} else if item.ItemType == ITEM_IMAGE {
		if item.Image != nil {
			iw, ih := item.Image.Bounds().Dx(), item.Image.Bounds().Dy()
			op := &ebiten.DrawImageOptions{}
			if int(maxSize.X) != iw || int(maxSize.Y) != ih {
				op.GeoM.Scale(float64(maxSize.X)/float64(iw), float64(maxSize.Y)/float64(ih))
			}
			op.GeoM.Translate(float64(offset.X), float64(offset.Y))
			subImg.DrawImage(item.Image, op)
		}
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

	} else if item.ItemType == ITEM_PROGRESS {

		// Draw progress track
		track := maxSize
		if item.Filled {
			drawRoundRect(subImg, &roundRect{
				Size:     track,
				Position: offset,
				Fillet:   item.Fillet,
				Filled:   true,
				Color:    style.Color,
			})
		}

		// Determine ratio
		ratio := 0.0
		if !item.Indeterminate && item.MaxValue > item.MinValue {
			ratio = float64((item.Value - item.MinValue) / (item.MaxValue - item.MinValue))
			if ratio < 0 {
				ratio = 0
			} else if ratio > 1 {
				ratio = 1
			}
		}

		if item.Indeterminate {
			// Barber pole: animate diagonal stripes moving to the right
			stripeW := float32(8) * uiScale
			offsetAnim := float32((time.Now().UnixNano()/int64(time.Millisecond))%1000) / 1000.0 * stripeW * 2
			bg := style.HoverColor.ToRGBA()
			// Fill base with hover color
			drawRoundRect(subImg, &roundRect{Size: track, Position: offset, Fillet: item.Fillet, Filled: true, Color: Color(bg)})
			// Draw stripes
			for x := offset.X - track.Y; x < offset.X+track.X+track.Y; x += stripeW * 2 {
				sx := x + offsetAnim
				drawParallelogram(subImg, sx, offset.Y, stripeW, track.Y, stripeW, style.SelectedColor.ToRGBA())
			}
		} else {
			filledW := float32(ratio) * track.X
			if filledW > 0 {
				drawRoundRect(subImg, &roundRect{
					Size:     point{X: filledW, Y: track.Y},
					Position: offset,
					Fillet:   item.Fillet,
					Filled:   true,
					Color:    style.SelectedColor,
				})
			}
		}
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

	item.DrawRect = rectAdd(item.DrawRect, base)
}

// drawParallelogram draws a filled, axis-aligned parallelogram with a rightward slant.
func drawParallelogram(dst *ebiten.Image, x, y, w, h, slant float32, col color.Color) {
	// Parallelogram points: (x,y) -> (x+w,y) -> (x+w+slant,y+h) -> (x+slant,y+h)
	path := vector.Path{}
	path.MoveTo(x, y)
	path.LineTo(x+w, y)
	path.LineTo(x+w+slant, y+h)
	path.LineTo(x+slant, y+h)
	path.Close()
	vs, is := path.AppendVerticesAndIndicesForFilling(nil, nil)
	for i := range vs {
		vs[i].ColorR, vs[i].ColorG, vs[i].ColorB, vs[i].ColorA = colorToVec4(col)
	}
	dst.DrawTriangles(vs, is, ebiten.NewImage(1, 1), &ebiten.DrawTrianglesOptions{})
}

func colorToVec4(c color.Color) (r, g, b, a float32) {
	rr, gg, bb, aa := c.RGBA()
	return float32(rr) / 65535, float32(gg) / 65535, float32(bb) / 65535, float32(aa) / 65535
}

func (item *itemData) drawItem(parent *itemData, offset point, base point, clip rect, screen *ebiten.Image, dropdowns *[]openDropdown) {
	if CacheCheck {
		item.RenderCount++
	}

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
	drawRect := intersectRect(itemRect, clip)
	if drawRect.X1 <= drawRect.X0 || drawRect.Y1 <= drawRect.Y0 {
		item.DrawRect = rectAdd(drawRect, base)
		return
	}

	if item.Render != nil {
		src := image.Rect(
			int(drawRect.X0-offset.X),
			int(drawRect.Y0-offset.Y),
			int(drawRect.X1-offset.X),
			int(drawRect.Y1-offset.Y),
		)
		sub := item.Render.SubImage(src).(*ebiten.Image)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(drawRect.X0), float64(drawRect.Y0))
		screen.DrawImage(sub, op)
	} else {
		item.drawItemInternal(parent, offset, base, drawRect, screen)
	}

	if item.ItemType == ITEM_DROPDOWN && item.Open {
		dropOff := pointAdd(offset, base)
		if item.Label != "" {
			textSize := (item.FontSize * uiScale) + 2
			dropOff.Y += textSize + currentStyle.TextPadding*uiScale
		}
		*dropdowns = append(*dropdowns, openDropdown{item: item, offset: dropOff})
	}

	if DebugMode {
		strokeRect(screen, drawRect.X0, drawRect.Y0, drawRect.X1-drawRect.X0, drawRect.Y1-drawRect.Y0, 1, color.RGBA{R: 128}, false)
	}
	if CacheCheck {
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%d", item.RenderCount), int(drawRect.X0), int(drawRect.Y0))
	}

	item.DrawRect = rectAdd(drawRect, base)
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

func (win *windowData) collectDropdowns(dropdowns *[]openDropdown) {
	collectItemDropdowns(win.Contents, dropdowns)
}

func collectItemDropdowns(items []*itemData, dropdowns *[]openDropdown) {
	for _, it := range items {
		if it.ItemType == ITEM_DROPDOWN && it.Open {
			off := point{X: it.DrawRect.X0, Y: it.DrawRect.Y0}
			if it.Label != "" {
				textSize := (it.FontSize * uiScale) + 2
				off.Y += textSize + currentStyle.TextPadding*uiScale
			}
			*dropdowns = append(*dropdowns, openDropdown{item: it, offset: off})
		}
		if len(it.Tabs) > 0 {
			if it.ActiveTab >= len(it.Tabs) {
				it.ActiveTab = 0
			}
			collectItemDropdowns(it.Tabs[it.ActiveTab].Contents, dropdowns)
		}
		collectItemDropdowns(it.Contents, dropdowns)
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
