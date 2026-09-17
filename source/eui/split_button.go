package eui

// NewSplitButton joins two buttons into one horizontal control. Each segment
// keeps its normal click handler, tooltip, enabled state, and requested size.
// The returned flow can be laid out as one item, so the halves never wrap apart.
// Either segment can open a menu, making the pair an action/dropdown button.
func NewSplitButton(first, second *ItemData) *ItemData {
	if first == nil || second == nil {
		return NewRow(first, second)
	}
	position := first.Position
	first.Position, second.Position = Point{}, Point{}
	first.splitTrailing, second.splitLeading = true, true
	row := NewRow(first, second)
	row.Position = position
	return row
}

// Extend joined surfaces into their neighbor's clipped area. Outer corners
// retain the theme radius while inner corners and borders meet on a straight seam.
func (item *itemData) buttonSurfaceGeometry(offset, size point) (point, point) {
	if item.ItemType != ITEM_BUTTON {
		return offset, size
	}
	extension := max(item.Fillet, item.Border*uiScale, 1)
	if item.splitLeading {
		offset.X -= extension
		size.X += extension
	}
	if item.splitTrailing {
		size.X += extension
	}
	return offset, size
}
