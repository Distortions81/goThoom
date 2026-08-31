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
var renderNow time.Time

var dropdownReuse []openDropdown

func acquireDrawImageOptions() *ebiten.DrawImageOptions {
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	op.DisableMipmaps = true
	return op
}

func acquireTextDrawOptions() *text.DrawOptions {
	op := &text.DrawOptions{}
	op.DrawImageOptions.Filter = ebiten.FilterNearest
	op.DrawImageOptions.DisableMipmaps = true
	return op
}

func releaseTextDrawOptions(op *text.DrawOptions) {}

type openDropdown struct {
	item   *itemData
	offset point
}

func itemFace(item *itemData, size float32) text.Face {
	// If an item already has a face set, prefer it only when it matches the
	// requested size. Otherwise, pull the appropriately sized face from the
	// cache so the supplied font size is honored.
	if item != nil && item.Face != nil {
		if gf, ok := item.Face.(*text.GoTextFace); ok {
			if gf.Size != float64(size) {
				return textFace(size)
			}
		}
		return item.Face
	}
	return textFace(size)
}

// Draw renders the UI to the provided screen image.
// Call this from your Ebiten Draw function.
func Draw(screen *ebiten.Image) {
	renderNow = time.Now()
	defer func() { renderNow = time.Time{} }()

	dropdowns := dropdownReuse[:0]
	if cap(dropdowns) < len(windows) {
		dropdowns = make([]openDropdown, 0, len(windows))
	}
	drawWindow := func(win *windowData) {
		if !win.Open {
			return
		}
		if win.refreshPending && (win.lastRefresh.IsZero() || renderNow.Sub(win.lastRefresh) >= win.refreshInterval) {
			win.refreshPending = false
			win.Dirty = true
		}
		// If a window contains an indeterminate progress bar, force a repaint
		// so the barber-pole animation advances even without data events.
		if !win.Dirty && win.HasIndeterminate {
			win.Dirty = true
		}
		win.Draw(screen, &dropdowns)
	}
	// Docked panes form the workspace's base layer. Draw its gutters over the
	// panes, then draw standalone utility windows so they remain ordinary
	// floating windows above the workspace.
	for _, win := range windows {
		if win.Docked {
			drawWindow(win)
		}
	}
	drawTileDividers(screen)
	for _, win := range windows {
		if !win.Docked {
			drawWindow(win)
		}
	}

	screenClip := rect{X0: 0, Y0: 0, X1: float32(screenWidth), Y1: float32(screenHeight)}
	for _, dd := range dropdowns {
		drawDropdownOptions(dd.item, dd.offset, screenClip, screen)
	}

	// Draw any active context menus as overlays using the same dropdown menu
	// rendering. Context menus originate from a screen-space position stored in
	// the item's DrawRect X0/Y0.
	if len(contextMenus) > 0 {
		for _, cm := range contextMenus {
			if cm == nil || !cm.Open {
				continue
			}
			off := point{X: cm.DrawRect.X0, Y: cm.DrawRect.Y0}
			drawDropdownOptions(cm, off, screenClip, screen)
		}
	}

	if shouldDrawTooltip(hoveredItem, dropdowns, contextMenus) {
		drawTooltip(screen, hoveredItem)
	}
	dropdownReuse = dropdowns

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

func shouldDrawTooltip(item *itemData, dropdowns []openDropdown, menus []*itemData) bool {
	if item == nil || item.Tooltip == "" || len(dropdowns) > 0 {
		return false
	}
	for _, menu := range menus {
		if menu != nil && menu.Open {
			return false
		}
	}
	return true
}

func drawTooltip(screen *ebiten.Image, item *itemData) {
	if item.tooltipW == 0 && item.Tooltip != "" {
		item.updateTooltipBounds()
	}
	faceSize := float32(12) * uiScale
	face := textFace(faceSize)
	pad := float32(tooltipPadding) * uiScale
	width := item.tooltipW + pad*2
	height := item.tooltipH + pad*2

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
	if x < 0 {
		x = 0
	}

	bg, fg, border := tooltipColors()

	drawFilledRect(screen, x, y, width, height, bg, true)
	strokeRect(screen, x, y, width, height, 1, border, true)

	dop := ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
	dop.GeoM.Translate(float64(x+pad), float64(y+pad))
	top := &text.DrawOptions{
		DrawImageOptions: dop,
		LayoutOptions: text.LayoutOptions{
			LineSpacing: tooltipLineSpacing(face),
		},
	}
	top.ColorScale.ScaleWithColor(fg)
	text.Draw(screen, item.Tooltip, face, top)
}

func tooltipColors() (background, foreground, border color.RGBA) {
	return color.RGBA{16, 18, 22, 255}, color.RGBA{255, 255, 255, 255}, color.RGBA{208, 212, 220, 255}
}

func (win *windowData) reusableRenderTarget(w, h int) *ebiten.Image {
	if win.Render != nil && win.Render.Bounds().Dx() == w && win.Render.Bounds().Dy() == h {
		win.Render.Clear()
		return win.Render
	}
	if win.Render != nil {
		win.Render.Deallocate()
	}
	win.Render = newUnmanagedImage(w, h)
	return win.Render
}

func (win *windowData) Draw(screen *ebiten.Image, dropdowns *[]openDropdown) {
	if win.NoCache {
		// In NoCache mode, if opacity is < 1, render to a temporary offscreen
		// image and composite with alpha. Otherwise, render directly to screen.
		if CacheCheck {
			win.RenderCount++
		}
		size := win.GetSize()
		if size.X < 1 || size.Y < 1 {
			return
		}
		if win.Opacity >= 0.9999 {
			// Direct render
			win.drawBG(screen)
			win.drawItems(screen, point{}, dropdowns)
			win.drawScrollbars(screen)
			win.drawWinTitle(screen)
			win.drawBorder(screen)
			win.Dirty = false
			// Collect dropdowns for separate overlay rendering and draw debug.
			win.collectDropdowns(dropdowns)
			win.drawDebug(screen)
			if CacheCheck {
				ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%d", win.RenderCount), int(win.getPosition().X), int(win.getPosition().Y))
			}
			return
		}

		// Offscreen render for opacity blending
		imgW, imgH := renderImageSize(size)
		tmp := win.reusableRenderTarget(imgW, imgH)
		// Draw into tmp in local coords: temporarily zero Position like cached path
		origPos := win.Position
		basePos := win.getPosition()
		win.Position = point{}
		win.drawBG(tmp)
		win.drawItems(tmp, basePos, dropdowns)
		win.drawScrollbars(tmp)
		win.drawWinTitle(tmp)
		win.drawBorder(tmp)
		win.Position = origPos
		win.Dirty = false

		op := acquireDrawImageOptions()
		op.GeoM.Translate(float64(basePos.X), float64(basePos.Y))
		op.ColorScale.Scale(1, 1, 1, win.Opacity)
		screen.DrawImage(tmp, op)
		// Collect dropdowns for separate overlay rendering and draw debug.
		win.collectDropdowns(dropdowns)
		win.drawDebug(screen)
		if CacheCheck {
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%d", win.RenderCount), int(win.getPosition().X), int(win.getPosition().Y))
		}
		return
	}

	// Cached/offscreen render path
	if win.Dirty || win.Render == nil {
		if CacheCheck {
			win.RenderCount++
		}
		size := win.GetSize()
		imgW, imgH := renderImageSize(size)
		if size.X < 1 || size.Y < 1 {
			return
		}
		win.reusableRenderTarget(imgW, imgH)
		origPos := win.Position
		basePos := win.getPosition()
		win.Position = point{}
		win.drawBG(win.Render)
		win.drawItems(win.Render, basePos, dropdowns)
		win.drawScrollbars(win.Render)
		win.drawWinTitle(win.Render)
		win.drawBorder(win.Render)
		win.Position = origPos
		win.Dirty = false
		win.lastRefresh = renderNow
	} else {
		win.collectDropdowns(dropdowns)
	}
	op := acquireDrawImageOptions()
	op.GeoM.Translate(float64(win.getPosition().X), float64(win.getPosition().Y))
	// Apply per-window opacity when compositing cached image
	if win.Opacity <= 0 {
		// Nothing to draw
		win.drawDebug(screen)
		if CacheCheck {
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%d", win.RenderCount), int(win.getPosition().X), int(win.getPosition().Y))
		}
		return
	}
	if win.Opacity < 0.9999 {
		op.ColorScale.Scale(1, 1, 1, win.Opacity)
	}
	screen.DrawImage(win.Render, op)
	win.drawDebug(screen)
	if CacheCheck {
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%d", win.RenderCount), int(win.getPosition().X), int(win.getPosition().Y))
	}
}

func renderImageSize(size point) (int, int) {
	w := int(math.Round(float64(size.X)))
	h := int(math.Round(float64(size.Y)))
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return w, h
}

func (win *windowData) drawBG(screen *ebiten.Image) {
	// In NoBGColor mode, skip all background work entirely (no shadow, no fill).
	if win.NoBGColor {
		return
	}
	if !win.Docked && windowShadows && win.ShadowSize > 0 && win.ShadowColor.A > 0 {
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
		X0: win.getPosition().X + win.BorderPad*win.scale(),
		Y0: win.getPosition().Y + win.BorderPad*win.scale(),
		X1: win.getPosition().X + win.GetSize().X - win.BorderPad*win.scale(),
		Y1: win.getPosition().Y + win.GetSize().Y - win.BorderPad*win.scale(),
	}
	fillet := win.Fillet
	if win.Docked {
		fillet = 0
	}
	drawRoundRect(screen, &roundRect{
		Size:     point{X: r.X1 - r.X0, Y: r.Y1 - r.Y0},
		Position: point{X: r.X0, Y: r.Y0},
		Fillet:   fillet,
		Filled:   true,
		Color:    win.backgroundColor(),
	})
}

func (win *windowData) drawWinTitle(screen *ebiten.Image) {
	// Window Title
	if win.TitleHeight > 0 {
		tr := win.getTitleRect()
		drawFilledRect(screen, tr.X0, tr.Y0, tr.X1-tr.X0, tr.Y1-tr.Y0, win.titleBackgroundColor(), true)

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
			tdop := acquireDrawImageOptions()
			tdop.GeoM.Translate(float64(win.getPosition().X+((win.GetTitleSize())/4)),
				float64(win.getPosition().Y+((win.GetTitleSize())/2)))

			top := &text.DrawOptions{DrawImageOptions: *tdop, LayoutOptions: loo}

			top.ColorScale.ScaleWithColor(win.Theme.Window.TitleTextColor)
			buf := strings.ReplaceAll(win.Title, "\n", "") //Remove newline
			buf = strings.ReplaceAll(buf, "\r", "")        //Remove return
			text.Draw(screen, buf, face, top)
		} else {
			textWidth = 0
		}

		//Close, Maximize, Search, and Pin icons
		var buttonsWidth float32 = 0
		if win.Closable {
			var xpad float32 = (win.GetTitleSize()) / 3.0
			color := win.Theme.Window.TitleColor
			if win.Theme.Window.CloseBGColor.A > 0 {
				r := win.xRect()
				drawFilledRect(screen, r.X0, r.Y0, r.X1-r.X0, r.Y1-r.Y0, win.Theme.Window.CloseBGColor, true)
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

		// Maximize icon
		if win.Maximizable {
			mr := win.maxRect()
			color := win.Theme.Window.TitleColor
			if win.HoverMax {
				color = win.Theme.Window.HoverTitleColor
				win.HoverMax = false
			}
			// Draw a window-like maximize icon: outer frame + top bar
			inset := uiScale * 5
			x := mr.X0 + inset
			y := mr.Y0 + inset
			w := (mr.X1 - mr.X0) - inset*2
			h := (mr.Y1 - mr.Y0) - inset*2
			if w < uiScale*4 {
				w = uiScale * 4
			}
			if h < uiScale*4 {
				h = uiScale * 4
			}
			strokeRect(screen, x, y, w, h, uiScale, color, true)
			// top bar
			barH := uiScale * 2
			if barH < 1 {
				barH = 1
			}
			drawFilledRect(screen, x+uiScale, y+uiScale, w-uiScale*2, barH, color.ToRGBA(), true)
			buttonsWidth += (win.GetTitleSize())
		}

		// Search icon
		if win.Searchable {
			sr := win.searchRect()
			color := win.Theme.Window.TitleColor
			if win.HoverSearch {
				color = win.Theme.Window.HoverTitleColor
				win.HoverSearch = false
			}
			cx := sr.X0 + (sr.X1-sr.X0)/2
			cy := sr.Y0 + (sr.Y1-sr.Y0)/2
			r := (sr.X1 - sr.X0) / 4
			vector.StrokeCircle(screen, cx, cy, r, uiScale, color.ToRGBA(), true)
			strokeLine(screen, cx+r/2, cy+r/2, sr.X1-uiScale*2, sr.Y1-uiScale*2, uiScale, color, true)
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

		if win.searchOpen {
			sb := win.searchBoxRect()
			drawRoundRect(screen, &roundRect{Size: point{X: sb.X1 - sb.X0, Y: sb.Y1 - sb.Y0}, Position: point{X: sb.X0, Y: sb.Y0}, Fillet: 0, Filled: true, Color: win.backgroundColor()})
			drawRoundRect(screen, &roundRect{Size: point{X: sb.X1 - sb.X0, Y: sb.Y1 - sb.Y0}, Position: point{X: sb.X0, Y: sb.Y0}, Fillet: 0, Filled: false, Border: uiScale, Color: win.Theme.Window.TitleColor})
			textSize := win.GetTitleSize() / 2
			face := textFace(textSize)
			loo := text.LayoutOptions{LineSpacing: 0, PrimaryAlign: text.AlignStart, SecondaryAlign: text.AlignCenter}
			tdop := acquireDrawImageOptions()
			tdop.GeoM.Translate(float64(sb.X0+textSize/2), float64(sb.Y0+(sb.Y1-sb.Y0)/2))
			top := &text.DrawOptions{DrawImageOptions: *tdop, LayoutOptions: loo}
			top.ColorScale.ScaleWithColor(win.Theme.Window.TitleColor)
			text.Draw(screen, win.SearchText, face, top)
			cr := win.searchCloseRect()
			xpad := (cr.X1 - cr.X0) / 3
			strokeLine(screen,
				cr.X0+xpad,
				cr.Y0+xpad,
				cr.X1-xpad,
				cr.Y1-xpad,
				uiScale, win.Theme.Window.TitleColor, true)
			strokeLine(screen,
				cr.X0+xpad,
				cr.Y1-xpad,
				cr.X1-xpad,
				cr.Y0+xpad,
				uiScale, win.Theme.Window.TitleColor, true)
		}
	}
}

func (win *windowData) drawBorder(screen *ebiten.Image) {
	// Tiled workspaces draw their boundaries through TileDividers. Outlining
	// each docked pane would also stroke the outside edge of the screen and
	// double-draw shared pane edges.
	if win.drawsStandaloneOutline() {
		FrameColor := win.Theme.Window.BorderColor
		if activeWindow == win {
			FrameColor = win.Theme.Window.ActiveColor
		} else if win.Hovered {
			FrameColor = win.Theme.Window.HoverColor
		}
		drawWindowOutline(screen, win.getPosition(), win.GetSize(), win.Fillet, win.Border, FrameColor)
	}
	if win.Resizable && !win.Docked {
		win.drawResizeThumb(screen)
	}
}

func (win *windowData) drawsStandaloneOutline() bool {
	return !win.Docked && win.Outlined && win.Border > 0
}

func drawWindowOutline(screen *ebiten.Image, pos, size point, fillet, border float32, col Color) {
	width := float32(math.Round(float64(border)))
	if width <= 0 || size.X <= 0 || size.Y <= 0 {
		return
	}

	x := float32(math.Round(float64(pos.X)))
	y := float32(math.Round(float64(pos.Y)))
	w := float32(math.Round(float64(size.X)))
	h := float32(math.Round(float64(size.Y)))
	if w <= 0 || h <= 0 {
		return
	}
	if width > w {
		width = w
	}
	if width > h {
		width = h
	}

	drawColor := color.RGBA(col)
	if fillet <= 0 {
		drawInsideRectBorder(screen, x, y, w, h, width, drawColor)
		return
	}

	inset := width / 2
	x += inset
	y += inset
	w -= width
	h -= width
	if w <= 0 || h <= 0 {
		return
	}
	if fillet > inset {
		fillet -= inset
	} else {
		fillet = 0
	}
	if fillet*2 > w {
		fillet = w / 2
	}
	if fillet*2 > h {
		fillet = h / 2
	}
	fillet = float32(math.Round(float64(fillet)))

	var path vector.Path
	path.MoveTo(x+fillet, y)
	path.LineTo(x+w-fillet, y)
	path.QuadTo(x+w, y, x+w, y+fillet)
	path.LineTo(x+w, y+h-fillet)
	path.QuadTo(x+w, y+h, x+w-fillet, y+h)
	path.LineTo(x+fillet, y+h)
	path.QuadTo(x, y+h, x, y+h-fillet)
	path.LineTo(x, y+fillet)
	path.QuadTo(x, y, x+fillet, y)
	path.Close()

	strokeOp := &vector.StrokeOptions{Width: width}
	drawOp := &vector.DrawPathOptions{AntiAlias: true}
	drawOp.ColorScale.ScaleWithColor(drawColor)
	vector.StrokePath(screen, &path, strokeOp, drawOp)
}

func drawInsideRectBorder(screen *ebiten.Image, x, y, w, h, width float32, col color.Color) {
	for _, r := range insideRectBorderRects(x, y, w, h, width) {
		drawFilledRect(screen, r.X0, r.Y0, r.X1-r.X0, r.Y1-r.Y0, col, false)
	}
}

func insideRectBorderRects(x, y, w, h, width float32) [4]rect {
	return [4]rect{
		{X0: x, Y0: y, X1: x + w, Y1: y + width},
		{X0: x, Y0: y + h - width, X1: x + w, Y1: y + h},
		{X0: x, Y0: y, X1: x + width, Y1: y + h},
		{X0: x + w - width, Y0: y, X1: x + w, Y1: y + h},
	}
}

func (win *windowData) drawResizeThumb(screen *ebiten.Image) {
	size := float32(12) * win.scale()
	step := float32(4) * win.scale()
	pad := win.BorderPad * win.scale()
	pos := win.getPosition()
	x1 := pos.X + win.GetSize().X - pad
	y1 := pos.Y + win.GetSize().Y - pad

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
		barH := scrollbarThumbLength(avail.Y, req.Y, win.scale())
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
		barW := scrollbarThumbLength(avail.X, req.X, win.scale())
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
	// In NoCache mode we draw to the main screen using absolute coordinates.
	// Offset window-local positions by the window's screen position so items
	// render at the correct place.
	if win.NoCache {
		winPos = pointAdd(winPos, win.getPosition())
	}
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
	if item.Disabled {
		style = disabledStyle(style)
	}

	if item.Filled || item.Outlined {
		col := item.Color
		if col == (Color{}) && style != nil {
			col = style.SelectedColor
		}
		if item.Filled {
			drawFilledRect(subImg, offset.X, offset.Y, item.GetSize().X, item.GetSize().Y, col.ToRGBA(), true)
		}
		if item.Outlined {
			b := item.Border * uiScale
			if b <= 0 {
				b = 1 * uiScale
			}
			oc := item.OutlineColor
			if oc == (Color{}) && style != nil {
				oc = style.OutlineColor
			}
			strokeRect(subImg, offset.X, offset.Y, item.GetSize().X, item.GetSize().Y, b, oc, true)
		}
	}

	var activeContents []*itemData
	drawOffset := pointSub(offset, item.Scroll)
	// Children should not draw or accept input under this flow's scrollbar.
	childClip := drawRect
	if item.Scrollable {
		req := item.contentBounds()
		size := item.GetSize()
		sbW := currentStyle.BorderPad.Slider * 2
		if item.FlowType == FLOW_VERTICAL && req.Y > size.Y {
			childClip.X1 -= sbW
		}
		if item.FlowType == FLOW_HORIZONTAL && req.X > size.X {
			childClip.Y1 -= sbW
		}
	}

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
			face := itemFace(tab, textSize)
			tw, _ := text.Measure(tab.Name, face, 0)
			w := float32(tw) + 8
			if w < float32(defaultTabWidth)*uiScale {
				w = float32(defaultTabWidth) * uiScale
			}
			col := style.Color
			if renderNow.Sub(tab.Clicked) < clickFlash {
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
			dop := ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
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
			inter := intersectRect(subRect, childClip)
			if inter.X1 <= inter.X0 || inter.Y1 <= inter.Y0 {
				subItem.DrawRect = rectAdd(inter, base)
			} else {
				subItem.drawFlows(win, item, itemPos, base, childClip, screen, dropdowns)
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
				inter := intersectRect(subRect, childClip)
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
				inter := intersectRect(subRect, childClip)
				if inter.X1 <= inter.X0 || inter.Y1 <= inter.Y0 {
					subItem.DrawRect = rectAdd(inter, base)
				} else {
					subItem.drawItem(item, objOff, base, childClip, screen, dropdowns)
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
			barH := scrollbarThumbLength(size.Y, req.Y, UIScale())
			maxScroll := req.Y - size.Y
			pos := float32(0)
			if maxScroll > 0 {
				pos = (item.Scroll.Y / maxScroll) * (size.Y - barH)
			}
			sbW := currentStyle.BorderPad.Slider * 2
			if len(item.ScrollMarks) > 0 {
				markCol := AccentColor().ToRGBA()
				for _, m := range item.ScrollMarks {
					if m < 0 || m > 1 {
						continue
					}
					y := drawRect.Y0 + m*size.Y
					drawFilledRect(subImg, drawRect.X1-sbW, y, sbW, uiScale, markCol, false)
				}
			}
			col := NewColor(96, 96, 96, 192)
			drawFilledRect(subImg, drawRect.X1-sbW, drawRect.Y0+pos, sbW, barH, col.ToRGBA(), false)
		} else if item.FlowType == FLOW_HORIZONTAL && req.X > size.X {
			barW := scrollbarThumbLength(size.X, req.X, UIScale())
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
	if item.Disabled {
		style = disabledStyle(style)
	}

	if item.Label != "" {
		textSize := (item.FontSize * uiScale) + 2
		face := itemFace(item, textSize)
		loo := text.LayoutOptions{PrimaryAlign: text.AlignStart, SecondaryAlign: text.AlignCenter}
		tdop := ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
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
		face := itemFace(item, textSize)
		loo := text.LayoutOptions{
			LineSpacing:    1.2,
			PrimaryAlign:   text.AlignStart,
			SecondaryAlign: text.AlignCenter,
		}
		tdop := ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
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
		face := itemFace(item, textSize)
		loo := text.LayoutOptions{
			LineSpacing:    1.2,
			PrimaryAlign:   text.AlignStart,
			SecondaryAlign: text.AlignCenter,
		}
		tdop := ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
		tdop.GeoM.Translate(
			float64(offset.X+auxSize.X+item.AuxSpace),
			float64(offset.Y+(auxSize.Y/2)),
		)
		top := &text.DrawOptions{DrawImageOptions: tdop, LayoutOptions: loo}
		top.ColorScale.ScaleWithColor(style.TextColor)
		text.Draw(subImg, item.Text, face, top)

	} else if item.ItemType == ITEM_BUTTON {

		itemColor := style.Color
		if renderNow.Sub(item.Clicked) < clickFlash {
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
		face := itemFace(item, textSize)
		if item.ParentWindow != nil && item.ParentWindow.DefaultButton == item && item.Face == nil {
			face = boldFace(textSize)
		}
		textCenterX := float64(offset.X + maxSize.X/2)
		if item.Image != nil {
			filter := ebiten.FilterNearest
			disableMipmaps := true
			if item.SmoothImage {
				filter = ebiten.FilterLinear
				disableMipmaps = false
			}
			sop := &ebiten.DrawImageOptions{Filter: filter, DisableMipmaps: disableMipmaps}
			imageWidth := float64(item.Image.Bounds().Dx())
			imageHeight := float64(item.Image.Bounds().Dy())
			availableWidth := float64(maxSize.X - 6*uiScale)
			availableHeight := float64(maxSize.Y - 6*uiScale)
			scale := float64(0)
			if imageWidth > 0 && imageHeight > 0 {
				scale = math.Min(availableWidth/imageWidth, availableHeight/imageHeight)
			}
			imageX := float64(offset.X) + (float64(maxSize.X)-imageWidth*scale)/2
			if scale > 0 && item.Text != "" {
				textWidth := float64(0)
				for _, line := range strings.Split(item.Text, "\n") {
					width, _ := text.Measure(line, face, 0)
					textWidth = math.Max(textWidth, width)
				}
				gap := float64(3 * uiScale)
				iconWidth := math.Max(0, availableWidth-textWidth-gap)
				scale = math.Min(scale, iconWidth/imageWidth)
				if scale > 0 {
					groupWidth := imageWidth*scale + gap + textWidth
					imageX = float64(offset.X) + (float64(maxSize.X)-groupWidth)/2
					textCenterX = imageX + imageWidth*scale + gap + textWidth/2
				}
			}
			if scale > 0 {
				sop.GeoM.Scale(scale, scale)
				sop.GeoM.Translate(
					imageX,
					float64(offset.Y)+(float64(maxSize.Y)-imageHeight*scale)/2,
				)
			}
			subImg.DrawImage(item.Image, sop)
		}

		loo := text.LayoutOptions{
			LineSpacing:    0,
			PrimaryAlign:   text.AlignCenter,
			SecondaryAlign: text.AlignCenter,
		}
		tdop := ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
		metrics := face.Metrics()
		lineHeight := math.Ceil(metrics.HAscent) + math.Ceil(metrics.HDescent) + math.Ceil(metrics.HLineGap)
		lines := strings.Split(item.Text, "\n")
		startY := float64(offset.Y) + float64(maxSize.Y)/2 - float64(lineHeight)*(float64(len(lines)-1))/2
		for i, line := range lines {
			tdop.GeoM.Reset()
			tdop.GeoM.Translate(
				textCenterX,
				startY+float64(i)*lineHeight,
			)
			top := &text.DrawOptions{DrawImageOptions: tdop, LayoutOptions: loo}
			top.ColorScale.ScaleWithColor(style.TextColor)
			text.Draw(subImg, line, face, top)
		}

		//Text
	} else if item.ItemType == ITEM_INPUT {

		itemColor := item.Color
		if itemColor == (Color{}) {
			itemColor = style.Color
		}
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
		face := itemFace(item, textSize)
		metrics := face.Metrics()
		lineHeight := math.Ceil(metrics.HAscent) + math.Ceil(metrics.HDescent) + math.Ceil(metrics.HLineGap)
		lines := strings.Split(item.Text, "\n")
		loo := text.LayoutOptions{
			LineSpacing:    0,
			PrimaryAlign:   text.AlignStart,
			SecondaryAlign: text.AlignCenter,
		}
		startY := float64(offset.Y) + float64(maxSize.Y)/2 - lineHeight*(float64(len(lines)-1))/2
		for i, line := range lines {
			tdop := ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
			tdop.GeoM.Translate(
				float64(offset.X+item.BorderPad+item.Padding+currentStyle.TextPadding*uiScale),
				startY+float64(i)*lineHeight,
			)
			top := &text.DrawOptions{DrawImageOptions: tdop, LayoutOptions: loo}
			top.ColorScale.ScaleWithColor(style.TextColor)
			text.Draw(subImg, line, face, top)
		}

		if item.Focused {
			runes := []rune(item.Text)
			if focused := item.CursorPos; focused < 0 {
				item.CursorPos = 0
			} else if focused > len(runes) {
				item.CursorPos = len(runes)
			}
			prefix := string(runes[:item.CursorPos])
			preLines := strings.Split(prefix, "\n")
			lineIdx := len(preLines) - 1
			lastLine := preLines[lineIdx]
			width, _ := text.Measure(lastLine, face, 0)
			cx := offset.X + item.BorderPad + item.Padding + currentStyle.TextPadding*uiScale + float32(width)
			cy := float32(startY + float64(lineIdx)*lineHeight)
			topY := cy - float32(math.Ceil(metrics.HAscent))
			bottomY := cy + float32(math.Ceil(metrics.HDescent))
			strokeLine(subImg,
				cx, topY,
				cx, bottomY,
				1, style.TextColor, false)
		}

	} else if item.ItemType == ITEM_SLIDER {

		itemColor := style.Color
		if item.Hovered {
			itemColor = style.HoverColor
		}

		if item.Vertical {
			knobW := item.AuxSize.X * uiScale
			knobH := item.AuxSize.Y * uiScale
			trackHeight := maxSize.Y - knobH
			trackX := offset.X + maxSize.X/2
			trackTop := offset.Y + knobH/2
			trackBottom := trackTop + trackHeight

			ratio := 0.0
			if item.MaxValue > item.MinValue {
				ratio = float64((item.Value - item.MinValue) / (item.MaxValue - item.MinValue))
			}
			if ratio < 0 {
				ratio = 0
			} else if ratio > 1 {
				ratio = 1
			}

			knobCenter := trackBottom - float32(ratio)*trackHeight
			filledCol := style.SelectedColor
			strokeLine(subImg, trackX, trackBottom, trackX, knobCenter, 2*uiScale, filledCol, true)
			strokeLine(subImg, trackX, knobCenter, trackX, trackTop, 2*uiScale, itemColor, true)
			knobRect := point{X: offset.X + (maxSize.X-knobW)/2, Y: knobCenter - knobH/2}
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
		} else {
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
			face := itemFace(item, textSize)
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
				tdop := ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
				tdop.GeoM.Translate(
					float64(trackStart+trackWidth+gap),
					float64(offset.Y+(maxSize.Y/2)),
				)
				top := &text.DrawOptions{DrawImageOptions: tdop, LayoutOptions: loo}
				top.ColorScale.ScaleWithColor(style.TextColor)
				text.Draw(subImg, valueText, face, top)
			}
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
		face := itemFace(item, textSize)
		loo := text.LayoutOptions{PrimaryAlign: text.AlignStart, SecondaryAlign: text.AlignCenter}
		tdop := ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
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
			if item.Image != nil {
				item.Image.Deallocate()
			}
			item.Image = colorWheelImage(int(wheelSize))
		}
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
		op.GeoM.Translate(float64(offset.X), float64(offset.Y))
		subImg.DrawImage(item.Image, op)

		h, _, v, _ := rgbaToHSVA(color.RGBA(item.WheelColor))
		radius := wheelSize / 2
		cx := offset.X + radius
		cy := offset.Y + radius
		px := cx + float32(math.Cos(h*math.Pi/180))*radius*float32(v)
		py := cy + float32(math.Sin(h*math.Pi/180))*radius*float32(v)
		vector.FillCircle(subImg, px, py, 4*uiScale, color.Black, true)
		vector.FillCircle(subImg, px, py, 2*uiScale, color.White, true)

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
			op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear, DisableMipmaps: true}
			scale := math.Min(float64(maxSize.X)/float64(iw), float64(maxSize.Y)/float64(ih))
			if scale != 1.0 {
				op.GeoM.Scale(scale, scale)
			}
			nw := float64(iw) * scale
			nh := float64(ih) * scale
			op.GeoM.Translate(float64(offset.X)+(float64(maxSize.X)-nw)/2, float64(offset.Y)+(float64(maxSize.Y)-nh)/2)
			if item.Disabled {
				// Lightly dim disabled images to indicate inactive/offline state.
				op.ColorScale.Scale(0.35, 0.35, 0.35, 1.0)
			}
			subImg.DrawImage(item.Image, op)
		}
	} else if item.ItemType == ITEM_IMAGE_FAST {
		if item.Image != nil {
			iw, ih := item.Image.Bounds().Dx(), item.Image.Bounds().Dy()
			op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
			scale := math.Min(float64(maxSize.X)/float64(iw), float64(maxSize.Y)/float64(ih))
			if scale != 1.0 {
				op.GeoM.Scale(scale, scale)
			}
			nw := float64(iw) * scale
			nh := float64(ih) * scale
			op.GeoM.Translate(float64(offset.X)+(float64(maxSize.X)-nw)/2, float64(offset.Y)+(float64(maxSize.Y)-nh)/2)
			if item.Disabled {
				// Lightly dim disabled images to indicate inactive/offline state.
				op.ColorScale.Scale(0.35, 0.35, 0.35, 1.0)
			}
			subImg.DrawImage(item.Image, op)
		}
	} else if item.ItemType == ITEM_TEXT {

		itemColor := item.Color
		if itemColor == (Color{}) {
			itemColor = style.Color
		}
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
		face := itemFace(item, textSize)
		metrics := face.Metrics()
		lineSpacing := float32(textSize) * 1.2
		loo := text.LayoutOptions{
			LineSpacing:    float64(lineSpacing),
			PrimaryAlign:   text.AlignStart,
			SecondaryAlign: text.AlignStart,
		}
		tdop := ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
		tdop.GeoM.Translate(
			float64(offset.X),
			float64(offset.Y))

		top := &text.DrawOptions{DrawImageOptions: tdop, LayoutOptions: loo}
		tcolor := style.TextColor
		if item.ForceTextColor && !item.Disabled {
			tcolor = item.TextColor
		}
		top.ColorScale.ScaleWithColor(tcolor)
		text.Draw(subImg, item.Text, face, top)
		if item.Prediction != "" {
			lines := strings.Split(item.Text, "\n")
			line := len(lines) - 1
			prefixWidth, _ := text.Measure(lines[line], face, 0)
			ghostOpts := &text.DrawOptions{LayoutOptions: loo}
			ghostOpts.GeoM.Translate(
				float64(offset.X)+prefixWidth,
				float64(offset.Y)+float64(line)*float64(lineSpacing),
			)
			ghostOpts.ColorScale.ScaleWithColor(style.DisabledColor)
			text.Draw(subImg, item.Prediction, face, ghostOpts)
		}

		if item.SelectableText && item.SelectStart != item.SelectEnd && len(item.Text) > 0 {
			selectionColor := style.ClickColor
			selectionColor.A = 0x60
			start, end := item.SelectStart, item.SelectEnd
			if start > end {
				start, end = end, start
			}
			runes := []rune(item.Text)
			if start < 0 {
				start = 0
			}
			if end > len(runes) {
				end = len(runes)
			}
			if start < end {
				lineStarts := []int{0}
				for i, r := range runes {
					if r == '\n' {
						lineStarts = append(lineStarts, i+1)
					}
				}
				for line, lineStart := range lineStarts {
					lineEnd := len(runes)
					if line+1 < len(lineStarts) {
						lineEnd = lineStarts[line+1] - 1
					}
					selectedStart := max(start, lineStart)
					selectedEnd := min(end, lineEnd)
					if selectedStart >= selectedEnd {
						continue
					}
					prefixWidth, _ := text.Measure(string(runes[lineStart:selectedStart]), face, 0)
					selectedWidth, _ := text.Measure(string(runes[selectedStart:selectedEnd]), face, 0)
					baseY := offset.Y + float32(line)*lineSpacing + float32(metrics.HAscent)
					topY := baseY - float32(math.Ceil(metrics.HAscent))
					bottomY := baseY + float32(math.Ceil(metrics.HDescent))
					drawRoundRect(subImg, &roundRect{
						Size:     point{X: float32(selectedWidth), Y: bottomY - topY},
						Position: point{X: offset.X + float32(prefixWidth), Y: topY},
						Filled:   true,
						Color:    selectionColor,
					})
				}
			}
		}

		if item.Focused {
			runes := []rune(item.Text)
			if focused := item.CursorPos; focused < 0 {
				item.CursorPos = 0
			} else if focused > len(runes) {
				item.CursorPos = len(runes)
			}
			prefix := string(runes[:item.CursorPos])
			preLines := strings.Split(prefix, "\n")
			lineIdx := len(preLines) - 1
			lastLine := preLines[lineIdx]
			width, _ := text.Measure(lastLine, face, 0)
			cx := offset.X + float32(width)
			baseY := offset.Y + float32(lineIdx)*lineSpacing + float32(metrics.HAscent)
			topY := baseY - float32(math.Ceil(metrics.HAscent))
			bottomY := baseY + float32(math.Ceil(metrics.HDescent))
			strokeLine(subImg,
				cx, topY,
				cx, bottomY,
				1, style.TextColor, false)
		}

		if len(item.Underlines) > 0 {
			rs := []rune(item.Text)
			for _, ul := range item.Underlines {
				if ul.Start < 0 || ul.End > len(rs) || ul.Start >= ul.End {
					continue
				}
				line := 0
				lineStart := 0
				for i := 0; i < ul.Start; i++ {
					if rs[i] == '\n' {
						line++
						lineStart = i + 1
					}
				}
				prefix := string(rs[lineStart:ul.Start])
				x0, _ := text.Measure(prefix, face, 0)
				word := string(rs[ul.Start:ul.End])
				w, _ := text.Measure(word, face, 0)
				baseY := offset.Y + float32(line)*lineSpacing + float32(metrics.HAscent)
				y := baseY + 1
				underlineColor := color.Color(color.NRGBA{R: 0xFF, G: 0x00, B: 0x00, A: 0xFF})
				if ul.MatchTextColor {
					underlineColor = tcolor
				}
				vector.StrokeLine(subImg,
					offset.X+float32(x0), y,
					offset.X+float32(x0+w), y,
					1,
					underlineColor, true)
			}
		}

		if item.OnURLClick != nil {
			runes := []rune(item.Text)
			for _, span := range item.urlRanges() {
				line, lineStart := 0, 0
				for i := 0; i < span.Start; i++ {
					if runes[i] == '\n' {
						line++
						lineStart = i + 1
					}
				}
				prefixWidth, _ := text.Measure(string(runes[lineStart:span.Start]), face, 0)
				urlWidth, _ := text.Measure(string(runes[span.Start:span.End]), face, 0)
				baseY := offset.Y + float32(line)*lineSpacing + float32(metrics.HAscent)
				vector.StrokeLine(subImg,
					offset.X+float32(prefixWidth), baseY+1,
					offset.X+float32(prefixWidth+urlWidth), baseY+1,
					1, color.NRGBA{R: 0x66, G: 0xAA, B: 0xFF, A: 0xFF}, true)
			}
		}

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
			offsetAnim := float32((renderNow.UnixNano()/int64(time.Millisecond))%1000) / 1000.0 * stripeW * 2
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
		outlineColor := item.OutlineColor
		if outlineColor == (Color{}) {
			outlineColor = style.OutlineColor
		}
		drawRoundRect(subImg, &roundRect{
			Size:     maxSize,
			Position: offset,
			Fillet:   item.Fillet,
			Filled:   false,
			Color:    outlineColor,
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
	drawOp := &vector.DrawPathOptions{}
	drawOp.ColorScale.ScaleWithColor(col)
	vector.FillPath(dst, &path, nil, drawOp)
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
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
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
	face := itemFace(item, textSize)
	loo := text.LayoutOptions{PrimaryAlign: text.AlignStart, SecondaryAlign: text.AlignCenter}

	if windowShadows && item.ShadowSize > 0 && item.ShadowColor.A > 0 {
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
	if item.Disabled {
		style = disabledStyle(style)
	}
	drawFilledRect(subImg,
		visibleRect.X0,
		visibleRect.Y0,
		visibleRect.X1-visibleRect.X0,
		visibleRect.Y1-visibleRect.Y0,
		style.Color, false)
	for i := first; i < first+visible && i < len(item.Options); i++ {
		y := offY + float32(i-first)*optionH
		// Do not highlight header rows.
		if (i == item.Selected || i == item.HoverIndex) && i >= item.HeaderCount {
			col := style.SelectedColor
			if i == item.HoverIndex && i != item.Selected {
				col = style.HoverColor
			}
			drawRoundRect(subImg, &roundRect{Size: maxSize, Position: point{X: offset.X, Y: y}, Fillet: item.Fillet, Filled: true, Color: col})
		}
		textX := offset.X + item.BorderPad + item.Padding + currentStyle.TextPadding*uiScale
		if itemHasOptionImages(item) {
			iconSize := dropdownOptionImageSize(item)
			if i < len(item.OptionImages) && item.OptionImages[i] != nil && iconSize > 0 {
				icon := item.OptionImages[i]
				iw := float64(icon.Bounds().Dx())
				ih := float64(icon.Bounds().Dy())
				if iw > 0 && ih > 0 {
					scale := math.Min(float64(iconSize)/iw, float64(iconSize)/ih)
					op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
					if item.SmoothImage {
						op.Filter = ebiten.FilterLinear
						op.DisableMipmaps = false
					}
					op.GeoM.Scale(scale, scale)
					op.GeoM.Translate(
						float64(textX)+(float64(iconSize)-iw*scale)/2,
						float64(y)+(float64(optionH)-ih*scale)/2,
					)
					subImg.DrawImage(icon, op)
				}
			}
			textX += dropdownOptionImageSlot(item)
		}
		td := acquireDrawImageOptions()
		td.GeoM.Translate(float64(textX), float64(y+optionH/2))
		tdo := acquireTextDrawOptions()
		tdo.DrawImageOptions = *td
		tdo.LayoutOptions = loo
		if i < item.HeaderCount {
			// Render headers with disabled text color.
			tdo.ColorScale.ScaleWithColor(style.DisabledColor)
		} else {
			tdo.ColorScale.ScaleWithColor(style.TextColor)
		}
		text.Draw(subImg, item.Options[i], face, tdo)
		releaseTextDrawOptions(tdo)
	}

	if len(item.Options) > visible {
		openH := optionH * float32(visible)
		totalH := optionH * float32(len(item.Options))
		barH := scrollbarThumbLength(openH, totalH, UIScale())
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
	var path vector.Path

	width := float32(math.Round(float64(rrect.Border)))
	if rrect.Fillet <= 0 {
		drawColor := color.RGBA(rrect.Color)
		if rrect.Filled {
			drawFilledRect(screen, rrect.Position.X, rrect.Position.Y, rrect.Size.X, rrect.Size.Y, drawColor, true)
		}
		if width > 0 {
			strokeRect(screen, rrect.Position.X, rrect.Position.Y, rrect.Size.X, rrect.Size.Y, width, drawColor, true)
		}
		return
	}
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

	drawColor := color.RGBA(rrect.Color)
	if rrect.Filled {
		drawOp := &vector.DrawPathOptions{AntiAlias: true}
		drawOp.ColorScale.ScaleWithColor(drawColor)
		vector.FillPath(screen, &path, nil, drawOp)
		return
	}

	if width <= 0 {
		return
	}
	strokeOp := &vector.StrokeOptions{Width: width}
	drawOp := &vector.DrawPathOptions{AntiAlias: true}
	drawOp.ColorScale.ScaleWithColor(drawColor)
	vector.StrokePath(screen, &path, strokeOp, drawOp)
}

func drawTabShape(screen *ebiten.Image, pos point, size point, col Color, fillet float32, slope float32) {
	var path vector.Path

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

	drawOp := &vector.DrawPathOptions{AntiAlias: origFillet > 0}
	drawOp.ColorScale.ScaleWithColor(color.RGBA(col))
	vector.FillPath(screen, &path, nil, drawOp)
}

func strokeTabShape(screen *ebiten.Image, pos point, size point, col Color, fillet float32, slope float32, border float32) {
	var path vector.Path

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

	strokeOp := &vector.StrokeOptions{Width: border}
	drawOp := &vector.DrawPathOptions{AntiAlias: true}
	drawOp.ColorScale.ScaleWithColor(color.RGBA(col))
	vector.StrokePath(screen, &path, strokeOp, drawOp)
}

func strokeTabTop(screen *ebiten.Image, pos point, size point, col Color, fillet float32, slope float32, border float32) {
	var path vector.Path

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

	path.MoveTo(pos.X, pos.Y)
	path.LineTo(pos.X+slope, pos.Y)
	path.LineTo(pos.X+slope, pos.Y-fillet)
	path.QuadTo(pos.X+slope, pos.Y-slope, pos.X+slope+fillet, pos.Y-slope)
	path.LineTo(pos.X+size.X-slope-fillet, pos.Y-slope)
	path.QuadTo(pos.X+size.X-slope, pos.Y-slope, pos.X+size.X-slope, pos.Y-fillet)
	path.LineTo(pos.X+size.X-slope, pos.Y)
	path.LineTo(pos.X, pos.Y)
	path.Close()

	strokeOp := &vector.StrokeOptions{Width: border}
	drawOp := &vector.DrawPathOptions{AntiAlias: true}
	drawOp.ColorScale.ScaleWithColor(color.RGBA(col))
	vector.StrokePath(screen, &path, strokeOp, drawOp)
}

func drawTriangle(screen *ebiten.Image, pos point, size float32, col Color) {
	var path vector.Path

	pos.X = float32(math.Round(float64(pos.X)))
	pos.Y = float32(math.Round(float64(pos.Y)))
	size = float32(math.Round(float64(size)))

	path.MoveTo(pos.X, pos.Y)
	path.LineTo(pos.X+size, pos.Y)
	path.LineTo(pos.X+size/2, pos.Y+size)
	path.Close()

	drawOp := &vector.DrawPathOptions{AntiAlias: true}
	drawOp.ColorScale.ScaleWithColor(color.RGBA(col))
	vector.FillPath(screen, &path, nil, drawOp)
}

func drawCheckmark(screen *ebiten.Image, start, mid, end point, width float32, col Color) {
	var path vector.Path

	width = float32(math.Round(float64(width)))
	off := pixelOffset(width)

	path.MoveTo(float32(math.Round(float64(start.X)))+off, float32(math.Round(float64(start.Y)))+off)
	path.LineTo(float32(math.Round(float64(mid.X)))+off, float32(math.Round(float64(mid.Y)))+off)
	path.LineTo(float32(math.Round(float64(end.X)))+off, float32(math.Round(float64(end.Y)))+off)

	strokeOpts := &vector.StrokeOptions{Width: width, LineJoin: vector.LineJoinRound, LineCap: vector.LineCapRound}
	drawOp := &vector.DrawPathOptions{AntiAlias: true}
	drawOp.ColorScale.ScaleWithColor(color.RGBA(col))
	vector.StrokePath(screen, &path, strokeOpts, drawOp)
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
