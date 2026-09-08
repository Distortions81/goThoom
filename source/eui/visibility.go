package eui

// Visibility is inherited by descendants, including controls in a hidden flow.
func (item *itemData) isInvisible() bool {
	for it := item; it != nil; it = it.Parent {
		if it.Invisible {
			return true
		}
	}
	return false
}

// A hidden subtree must not retain hit rectangles, focus, or popup overlays
// from the previous frame. Requested sizes remain intact for showing it again.
func (item *itemData) clearHiddenState() {
	item.DrawRect = rect{}
	item.Hovered, item.Focused = false, false
	if item.ItemType == ITEM_DROPDOWN {
		item.Open = false
	}
	if focusedItem == item {
		focusedItem = nil
	}
	if activeItem == item {
		activeItem = nil
	}
	if hoveredItem == item {
		hoveredItem = nil
	}
	if selectedTextItem == item {
		selectedTextItem = nil
	}
	if dragFlow == item {
		dragFlow, dragWin, dragPart = nil, nil, PART_NONE
	}
	for _, child := range item.Contents {
		child.clearHiddenState()
	}
	for _, tab := range item.Tabs {
		tab.clearHiddenState()
	}
}
