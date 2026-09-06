package eui

import (
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// tooltipIndicatorRect stays inside the existing hover target and respects UI
// scaling. Configuration windows opt in; ordinary content and toolbars do not.
func (item *itemData) tooltipIndicatorRect(offset, size point) rect {
	if item.ColorSwatch || item.Tooltip == "" || item.ParentWindow == nil || !item.ParentWindow.ShowTooltipIndicators {
		return rect{}
	}
	textSize := item.FontSize*uiScale + 2
	face := itemFace(item, textSize)
	caption := item.Text
	startX := offset.X
	height := size.Y
	if item.Label != "" {
		caption = item.Label
		height = textSize
	} else if item.ItemType == ITEM_CHECKBOX || item.ItemType == ITEM_RADIO {
		height = item.AuxSize.Y * uiScale
		startX += item.AuxSize.X*uiScale + item.AuxSpace
	} else if item.ItemType == ITEM_TEXT {
		height = textSize
	} else if item.ItemType == ITEM_BUTTON && item.ParentWindow.DefaultButton == item && item.Face == nil {
		face = boldFace(textSize)
	}
	diameter := float32(12) * uiScale
	if size.X < 24*uiScale || height < diameter {
		return rect{}
	}
	var textWidth float32
	for line := range strings.SplitSeq(caption, "\n") {
		w, _ := text.Measure(line, face, 0)
		textWidth = max(textWidth, float32(w))
	}
	if item.Label == "" && item.ItemType == ITEM_BUTTON {
		// Match the button's centered caption, including its marker allowance.
		startX = offset.X + size.X/2 - 9*uiScale - textWidth/2
	}
	x := min(startX+textWidth+6*uiScale, offset.X+size.X-diameter-2*uiScale)
	y := offset.Y + (height-diameter)/2
	return rect{X0: x, Y0: y, X1: x + diameter, Y1: y + diameter}
}

func drawTooltipIndicator(dst *ebiten.Image, r rect, tint Color) {
	x, y := (r.X0+r.X1)/2, (r.Y0+r.Y1)/2
	radius := (r.X1 - r.X0) / 2
	vector.StrokeCircle(dst, x, y, radius-uiScale/2, uiScale, tint, true)
	vector.FillCircle(dst, x, y-2.5*uiScale, .8*uiScale, tint, true)
	vector.FillRect(dst, x-.6*uiScale, y-.5*uiScale, 1.2*uiScale, 4*uiScale, tint, true)
}
