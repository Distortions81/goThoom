package eui

import (
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// buttonContentWidth treats the requested width as a minimum. Layout and
// rendering must reserve the same caption, icon, and help-indicator space.
func (item *itemData) buttonContentWidth() float32 {
	if item.ItemType != ITEM_BUTTON || item.Text == "" {
		return 0
	}
	size := item.FontSize*uiScale + 2
	face := itemFace(item, size)
	if item.ParentWindow != nil && item.ParentWindow.DefaultButton == item && item.Face == nil {
		face = boldFace(size)
	}
	var width float64
	for line := range strings.SplitSeq(item.Text, "\n") {
		w, _ := text.Measure(line, face, 0)
		width = math.Max(width, w)
	}
	width += 6 * float64(uiScale)
	if item.Image != nil {
		width += 21 * float64(uiScale)
	}
	if item.Label == "" && item.Tooltip != "" && item.ParentWindow != nil && item.ParentWindow.ShowTooltipIndicators {
		width += 18 * float64(uiScale)
	}
	return float32(math.Ceil(width))
}

// LayoutWindowBody fits a vertical root into the window's client area. Body is
// its scrolling child; other siblings stay docked above or below it. Heights
// include measured child content, labels, and position gaps rather than a
// caller-maintained subtraction constant. Call after changing content and on
// resize. The body must be a direct child of root.
func LayoutWindowBody(win *WindowData, root, body *ItemData) {
	if win == nil || root == nil || body == nil || body.Parent != root {
		return
	}
	scale := UIScale()
	if scale <= 0 {
		scale = 1
	}
	pad := (win.Padding + win.BorderPad) * win.scale()
	width := max(float32(1), win.GetSize().X-2*pad-root.getPosition(win).X)
	height := max(float32(1), win.GetSize().Y-win.GetTitleSize()-2*pad-root.getPosition(win).Y)
	root.Fixed = true
	body.Fixed = true
	body.Scrollable = true
	root.Size = Point{X: width / scale, Y: height / scale}
	reserved := float32(0)
	for _, item := range root.Contents {
		if item.Invisible {
			continue
		}
		position := item.getPosition(win)
		item.Size.X = max(float32(1), width-position.X) / scale
		reserved += position.Y
		if item == body {
			continue
		}
		item.resizeFlow(Point{X: width, Y: height})
		reserved += item.GetSize().Y
	}
	body.Size.Y = max(float32(1), height-reserved) / scale
	body.resizeFlow(Point{X: width, Y: height})
}
