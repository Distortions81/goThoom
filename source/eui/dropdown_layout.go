package eui

import "github.com/hajimehoshi/ebiten/v2/text/v2"

// A hover preview may relayout the entire window. Keep the menu's text and hit
// targets anchored to the geometry it opened with, while colors update live.
type dropdownLayout struct {
	bounds         rect
	visible        int
	rowHeight      float32
	fontSize       float32
	face           text.Face
	textInset      float32
	iconSize       float32
	iconGap        float32
	scrollbarWidth float32
}

// Option rows exclude the dropdown's field label.
func dropdownOptionHeight(item *itemData) float32 {
	if item.Open && item.dropdownLayout != nil {
		return item.dropdownLayout.rowHeight
	}
	height := item.GetSize().Y
	if item.Label != "" {
		height -= item.FontSize*uiScale + 2 + currentStyle.TextPadding*uiScale
	}
	return max(1, height)
}

// Drawing and pointer handling must use the same bounds during a preview.
func dropdownOpenRect(item *itemData, offset point) (rect, int) {
	layout := dropdownMenuLayout(item, offset)
	return layout.bounds, layout.visible
}

func dropdownMenuLayout(item *itemData, offset point) dropdownLayout {
	if item.Open && item.dropdownLayout != nil {
		return *item.dropdownLayout
	}
	maxSize := item.GetSize()
	optionH := dropdownOptionHeight(item)
	visible := item.MaxVisible
	if visible <= 0 {
		visible = len(item.Options)
	}
	visible = min(visible, len(item.Options))
	maxVisible := max(1, int((float32(screenHeight)-optionH*dropdownOverlayReserve*2)/optionH))
	visible = min(visible, maxVisible)

	startY := offset.Y + maxSize.Y
	r := rect{X0: offset.X, Y0: startY, X1: offset.X + maxSize.X, Y1: startY + optionH*float32(visible)}
	bottomLimit := float32(screenHeight) - optionH*dropdownOverlayReserve
	if r.Y1 > bottomLimit {
		diff := r.Y1 - bottomLimit
		r.Y0 -= diff
		r.Y1 -= diff
	}
	topLimit := optionH * dropdownOverlayReserve
	if r.Y0 < topLimit {
		diff := topLimit - r.Y0
		r.Y0 += diff
		r.Y1 += diff
	}
	layout := dropdownLayout{
		bounds: r, visible: visible, rowHeight: optionH,
		fontSize: item.FontSize*uiScale + 2, face: item.Face,
		textInset: item.BorderPad + item.Padding + currentStyle.TextPadding*uiScale,
		iconSize:  max(0, optionH-8*uiScale), iconGap: 4 * uiScale,
		scrollbarWidth: currentStyle.BorderPad.Slider * 2,
	}
	if item.Open && item.OnHover != nil {
		item.dropdownLayout = &layout
	}
	return layout
}
