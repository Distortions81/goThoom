package eui

import "math"

// CornerSnapThreshold defines how close a window edge or corner must be to
// snap to a screen corner or another window.
const CornerSnapThreshold float32 = 10

// UnsnapThreshold defines how far a window must move from its snapped
// position before corner snapping is re-enabled.
const UnsnapThreshold float32 = 12

// HZone defines the horizontal zone positions.
type HZone int

const (
	HZoneLeft HZone = iota
	HZoneLeftCenter
	HZoneCenterLeft
	HZoneCenter
	HZoneCenterRight
	HZoneRightCenter
	HZoneRight
)

// VZone defines the vertical zone positions.
type VZone int

const (
	VZoneTop VZone = iota
	VZoneTopMiddle
	VZoneMiddleTop
	VZoneCenter
	VZoneMiddleBottom
	VZoneBottomMiddle
	VZoneBottom
)

type windowZone struct {
	h      HZone
	v      VZone
	offset Point
}

// SetZone assigns a horizontal and vertical zone to the window. The window's
// center will be kept on this zone.
func (win *windowData) SetZone(h HZone, v VZone) {
	win.zone = &windowZone{h: h, v: v}
	win.updateZonePosition()
}

// SetZoneOffset adjusts a zoned window by a screen-pixel offset. The offset
// stays applied when the screen size changes, while dragging clears the zone
// and leaves the window at its current position.
func (win *windowData) SetZoneOffset(offset Point) {
	if win.zone == nil {
		return
	}
	win.zone.offset = offset
	win.updateZonePosition()
}

// ClearZone removes any zone assignment from the window.
func (win *windowData) ClearZone() {
	win.zone = nil
}

func (win *windowData) updateZonePosition() {
	if win.zone == nil {
		return
	}
	cx := hZoneCoord(win.zone.h, screenWidth)
	cy := vZoneCoord(win.zone.v, screenHeight)
	size := win.GetSize()
	s := win.scale()
	win.Position.X = (cx - size.X/2 + win.zone.offset.X) / s
	win.Position.Y = (cy - size.Y/2 + win.zone.offset.Y) / s

	maxX := (float32(screenWidth) - size.X) / s
	maxY := (float32(screenHeight) - size.Y) / s
	if maxX < 0 {
		maxX = 0
	}
	if maxY < 0 {
		maxY = 0
	}
	if win.Position.X < 0 {
		win.Position.X = 0
	} else if win.Position.X > maxX {
		win.Position.X = maxX
	}
	if win.Position.Y < 0 {
		win.Position.Y = 0
	} else if win.Position.Y > maxY {
		win.Position.Y = maxY
	}
	win.clampToScreen()
}

func hZoneCoord(z HZone, width int) float32 {
	switch z {
	case HZoneLeft:
		return 0
	case HZoneLeftCenter:
		return float32(width) * (1.0 / 6.0)
	case HZoneCenterLeft:
		return float32(width) * (2.0 / 6.0)
	case HZoneCenter:
		return float32(width) * 0.5
	case HZoneCenterRight:
		return float32(width) * (4.0 / 6.0)
	case HZoneRightCenter:
		return float32(width) * (5.0 / 6.0)
	case HZoneRight:
		return float32(width)
	default:
		return float32(width) * 0.5
	}
}

func vZoneCoord(z VZone, height int) float32 {
	switch z {
	case VZoneTop:
		return 0
	case VZoneTopMiddle:
		return float32(height) * (1.0 / 6.0)
	case VZoneMiddleTop:
		return float32(height) * (2.0 / 6.0)
	case VZoneCenter:
		return float32(height) * 0.5
	case VZoneMiddleBottom:
		return float32(height) * (4.0 / 6.0)
	case VZoneBottomMiddle:
		return float32(height) * (5.0 / 6.0)
	case VZoneBottom:
		return float32(height)
	default:
		return float32(height) * 0.5
	}
}

// snapToCorner assigns a zone when a window is dragged close to a screen
// corner. It returns true if a zone was applied.
func snapToCorner(win *windowData) bool {
	if !windowSnapping {
		return false
	}
	pos := win.getPosition()
	size := win.Size

	sw := float32(screenWidth) / uiScale
	sh := float32(screenHeight) / uiScale

	// Top-left
	if pos.X <= CornerSnapThreshold && pos.Y <= CornerSnapThreshold {
		win.SetZone(HZoneLeft, VZoneTop)
		win.snapAnchor = win.Position
		win.snapAnchorActive = true
		return true
	}
	// Top-right
	if pos.X+size.X >= sw-CornerSnapThreshold && pos.Y <= CornerSnapThreshold {
		win.SetZone(HZoneRight, VZoneTop)
		win.snapAnchor = win.Position
		win.snapAnchorActive = true
		return true
	}
	// Bottom-left
	if pos.X <= CornerSnapThreshold && pos.Y+size.Y >= sh-CornerSnapThreshold {
		win.SetZone(HZoneLeft, VZoneBottom)
		win.snapAnchor = win.Position
		win.snapAnchorActive = true
		return true
	}
	// Bottom-right
	if pos.X+size.X >= sw-CornerSnapThreshold && pos.Y+size.Y >= sh-CornerSnapThreshold {
		win.SetZone(HZoneRight, VZoneBottom)
		win.snapAnchor = win.Position
		win.snapAnchorActive = true
		return true
	}
	return false
}

// snapToWindow snaps a window's edges to nearby windows within the threshold.
// It returns true if the window position was adjusted.
func snapToWindow(win *windowData) bool {
	if !windowSnapping {
		return false
	}
	pos := win.getPosition()
	size := win.Size
	snapped := false

	for _, other := range windows {
		if other == win || !other.Open {
			continue
		}
		opos := other.getPosition()
		osize := other.Size

		// Horizontal snapping
		if pos.Y < opos.Y+osize.Y && pos.Y+size.Y > opos.Y {
			// Snap left edge to other's right edge
			if math.Abs(float64(pos.X-(opos.X+osize.X))) <= float64(CornerSnapThreshold) {
				win.Position.X = opos.X + osize.X
				snapped = true
				pos.X = win.Position.X
			}
			// Snap right edge to other's left edge
			if math.Abs(float64((pos.X+size.X)-opos.X)) <= float64(CornerSnapThreshold) {
				win.Position.X = opos.X - size.X
				snapped = true
				pos.X = win.Position.X
			}
		}

		// Vertical snapping
		if pos.X < opos.X+osize.X && pos.X+size.X > opos.X {
			// Snap top edge to other's bottom edge
			if math.Abs(float64(pos.Y-(opos.Y+osize.Y))) <= float64(CornerSnapThreshold) {
				win.Position.Y = opos.Y + osize.Y
				snapped = true
				pos.Y = win.Position.Y
			}
			// Snap bottom edge to other's top edge
			if math.Abs(float64((pos.Y+size.Y)-opos.Y)) <= float64(CornerSnapThreshold) {
				win.Position.Y = opos.Y - size.Y
				snapped = true
				pos.Y = win.Position.Y
			}
		}
	}
	return snapped
}

// snapResize adjusts a window's size and position when resizing so edges
// snap to nearby screen edges or other windows within the threshold.
// It returns true if the window size or position was adjusted.
func snapResize(win *windowData, part dragType) bool {
	if !windowSnapping {
		return false
	}
	s := win.scale()
	pos, size := win.Position, win.Size
	sw, sh := float32(screenWidth)/s, float32(screenHeight)/s
	includesLeft := part == PART_LEFT || part == PART_TOP_LEFT || part == PART_BOTTOM_LEFT
	includesRight := part == PART_RIGHT || part == PART_TOP_RIGHT || part == PART_BOTTOM_RIGHT
	includesTop := part == PART_TOP || part == PART_TOP_LEFT || part == PART_TOP_RIGHT
	includesBottom := part == PART_BOTTOM || part == PART_BOTTOM_LEFT || part == PART_BOTTOM_RIGHT

	var delta point
	near := func(a, b float32) bool { return math.Abs(float64(a-b)) <= float64(CornerSnapThreshold) }
	if includesLeft && near(pos.X, 0) {
		delta.X = -pos.X
	}
	if includesRight && near(pos.X+size.X, sw) {
		delta.X = sw - pos.X - size.X
	}
	if includesTop && near(pos.Y, 0) {
		delta.Y = -pos.Y
	}
	if includesBottom && near(pos.Y+size.Y, sh) {
		delta.Y = sh - pos.Y - size.Y
	}
	for _, other := range windows {
		if other == win || !other.Open {
			continue
		}
		// Other windows may opt out of UI scaling. Compare in this window's units.
		op, os := other.GetPos(), other.GetSize()
		opos, osize := point{X: op.X / s, Y: op.Y / s}, point{X: os.X / s, Y: os.Y / s}
		if pos.Y < opos.Y+osize.Y && pos.Y+size.Y > opos.Y {
			if includesLeft && near(pos.X, opos.X+osize.X) {
				delta.X = opos.X + osize.X - pos.X
			}
			if includesRight && near(pos.X+size.X, opos.X) {
				delta.X = opos.X - pos.X - size.X
			}
		}
		if pos.X < opos.X+osize.X && pos.X+size.X > opos.X {
			if includesTop && near(pos.Y, opos.Y+osize.Y) {
				delta.Y = opos.Y + osize.Y - pos.Y
			}
			if includesBottom && near(pos.Y+size.Y, opos.Y) {
				delta.Y = opos.Y - pos.Y - size.Y
			}
		}
	}
	if delta == (point{}) {
		return false
	}
	return dragWindowResize(win, part, delta)
}
