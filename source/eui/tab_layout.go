package eui

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

func tabRowHeight(item, style *itemData) float32 {
	height := float32(defaultTabHeight) * uiScale
	size := tabFontSize(item, style)*uiScale + 2
	for _, tab := range item.Tabs {
		metrics := itemFace(tab, size).Metrics()
		height = max(height, float32(math.Ceil(metrics.HAscent+metrics.HDescent))+4*uiScale)
	}
	return height
}

// Tab labels determine their widths. Fixed panels wrap at their available
// width; TabColumns also imposes a maximum number of tabs per row.
// Drawing, content bounds, and hit targets all use these same rectangles.
func layoutTabs(item, style *itemData) ([]rect, point) {
	rects := make([]rect, len(item.Tabs))
	return rects, measureTabLayout(item, style, rects)
}

// Bounds queries do not need to allocate the per-tab rectangles.
func measureTabLayout(item, style *itemData, rects []rect) point {
	if item == nil || len(item.Tabs) == 0 {
		return point{}
	}
	height := tabRowHeight(item, style)
	gap := 3 * uiScale
	padding := 2 * max(8*uiScale, item.BorderPad*uiScale+4*uiScale)
	size := tabFontSize(item, style)*uiScale + 2
	limit := float32(0)
	if item.Fixed {
		limit = item.Size.X * uiScale
	}
	var x, y, width float32
	row, columns := 0, 0
	for i, tab := range item.Tabs {
		tw, _ := text.Measure(tab.Name, itemFace(tab, size), 0)
		w := max(float32(math.Ceil(tw))+padding, 40*uiScale)
		if columns > 0 && ((item.TabColumns > 0 && columns >= item.TabColumns) || (limit > 0 && x+w > limit)) {
			row++
			columns = 0
			x = float32(row%2) * item.TabRowOffset * uiScale
			y += height + 4*uiScale
		}
		if i < len(rects) {
			rects[i] = rect{X0: x, Y0: y, X1: x + w, Y1: y + height}
		}
		width = max(width, x+w)
		x += w + gap
		columns++
	}
	return point{X: width, Y: y + height}
}

func tabStripHeight(item, style *itemData) float32 {
	size := measureTabLayout(item, style, nil)
	return size.Y
}

func tabStripWidth(item *itemData) float32 {
	size := measureTabLayout(item, item.themeStyle(), nil)
	return size.X
}
