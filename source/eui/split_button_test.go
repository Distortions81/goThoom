package eui

import "testing"

func TestSplitButtonSegmentsKeepIndependentActions(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	oldScale := uiScale
	oldActive, oldHovered := activeItem, hoveredItem
	t.Cleanup(func() { uiScale, activeItem, hoveredItem = oldScale, oldActive, oldHovered })
	uiScale = 1
	leftClicks, rightClicks := 0, 0
	left := NewActionButton("-", func() { leftClicks++ })
	right := NewActionButton("+", func() { rightClicks++ })
	left.Size, right.Size = Point{X: 40, Y: 24}, Point{X: 30, Y: 24}
	left.SetTooltip("Decrease")
	right.SetTooltip("Increase")
	group := NewSplitButton(left, right)
	if group.GetSize().X != 70 || left.Position != (Point{}) || right.Position != (Point{}) {
		t.Fatal("segments retained a gap or lost their sizes", group.GetSize())
	}
	left.DrawRect, right.DrawRect = rect{X1: 40, Y1: 24}, rect{X0: 40, X1: 70, Y1: 24}
	left.clickItem(point{X: 10, Y: 10}, true)
	right.clickItem(point{X: 50, Y: 10}, true)
	if leftClicks != 1 || rightClicks != 1 {
		t.Fatal("segment actions were not independent")
	}
	left.Disabled = true
	left.clickItem(point{X: 10, Y: 10}, true)
	right.clickItem(point{X: 50, Y: 10}, true)
	if leftClicks != 1 || rightClicks != 2 || left.Tooltip != "Decrease" || right.Tooltip != "Increase" {
		t.Fatal("disabled state or tooltips leaked between segments")
	}
	for _, button := range []*ItemData{left, right} {
		offset, size := button.buttonSurfaceGeometry(Point{X: 10, Y: 20}, Point{X: 40, Y: 24})
		if offset.Y != 20 || size.Y != 24 || size.X <= 40 {
			t.Fatal("joined surface changed vertical bounds or left a rounded inner corner")
		}
	}
}
