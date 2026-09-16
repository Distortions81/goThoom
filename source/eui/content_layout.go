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
	lineIndex := 0
	for line := range strings.SplitSeq(item.Text, "\n") {
		w, _ := text.Measure(line, face, 0)
		if lineIndex == 0 && item.Prediction != "" {
			predictionWidth, _ := text.Measure(item.Prediction, face, 0)
			w += predictionWidth
		}
		width = math.Max(width, w)
		lineIndex++
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

// FitButtonCaption reduces a diagram button's font to fit its declared area.
// Call after changing its size, caption, or UI scale. Ordinary action buttons
// should continue to grow to fit their content instead.
func (item *itemData) FitButtonCaption(maxFontSize float32) {
	if item.ItemType != ITEM_BUTTON || item.Size.X <= 0 || item.Size.Y <= 0 || maxFontSize <= 0 {
		return
	}
	item.FontSize = maxFontSize
	for item.FontSize > 1 {
		face := itemFace(item, item.FontSize*uiScale+2)
		metrics := face.Metrics()
		lineHeight := math.Ceil(metrics.HAscent) + math.Ceil(metrics.HDescent) + math.Ceil(metrics.HLineGap)
		height := float32(lineHeight*float64(strings.Count(item.Text, "\n")+1)) + 6*uiScale
		if item.buttonContentWidth() <= item.Size.X*uiScale && height <= item.Size.Y*uiScale {
			break
		}
		item.FontSize = max(float32(1), item.FontSize-.5)
	}
	item.Dirty = true
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
