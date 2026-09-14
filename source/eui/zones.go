package eui

import "math"

// CornerSnapThreshold defines how close a moving window edge or corner must be
// to snap to a screen edge or another window.
const CornerSnapThreshold float32 = 18

// UnsnapThreshold defines how far the pointer-relative window position must
// move from a magnetic anchor before that axis releases.
const UnsnapThreshold float32 = 22

const (
	resizeSnapThreshold   float32 = 10
	resizeUnsnapThreshold float32 = 12
)

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
	pos, size := win.Position, win.Size
	s := win.scale()
	sw := float32(screenWidth) / s
	sh := float32(screenHeight) / s
	near := func(a, b float32) bool {
		return math.Abs(float64(a-b)) <= float64(CornerSnapThreshold)
	}

	// Top-left
	if near(pos.X, 0) && near(pos.Y, 0) {
		before := win.Position
		win.SetZone(HZoneLeft, VZoneTop)
		setSnapAnchor(win, before, true, true)
		return true
	}
	// Top-right
	if near(pos.X+size.X, sw) && near(pos.Y, 0) {
		before := win.Position
		win.SetZone(HZoneRight, VZoneTop)
		setSnapAnchor(win, before, true, true)
		return true
	}
	// Bottom-left
	if near(pos.X, 0) && near(pos.Y+size.Y, sh) {
		before := win.Position
		win.SetZone(HZoneLeft, VZoneBottom)
		setSnapAnchor(win, before, true, true)
		return true
	}
	// Bottom-right
	if near(pos.X+size.X, sw) && near(pos.Y+size.Y, sh) {
		before := win.Position
		win.SetZone(HZoneRight, VZoneBottom)
		setSnapAnchor(win, before, true, true)
		return true
	}
	return false
}

func setSnapAnchor(win *windowData, dragPosition point, snapX, snapY bool) {
	if !win.snapAnchorActive {
		win.snapDragPosition = dragPosition
	}
	if snapX && !win.snapAnchorX {
		win.snapDragPosition.X = dragPosition.X
	}
	if snapY && !win.snapAnchorY {
		win.snapDragPosition.Y = dragPosition.Y
	}
	win.snapAnchor = win.Position
	win.snapAnchorX = win.snapAnchorX || snapX
	win.snapAnchorY = win.snapAnchorY || snapY
	win.snapAnchorActive = win.snapAnchorX || win.snapAnchorY
}

// snapToWindow snaps a window to nearby screen or window edges. Comparisons
// use the dragged window's coordinate system so scaled and unscaled windows
// can align. The closest candidate wins independently on each axis.
func snapToWindow(win *windowData) bool {
	if !windowSnapping {
		return false
	}
	s := win.scale()
	pos, size := win.Position, win.Size
	sw, sh := float32(screenWidth)/s, float32(screenHeight)/s
	bestX, bestY := CornerSnapThreshold+1, CornerSnapThreshold+1
	delta := point{}
	snapX, snapY := false, false
	considerX := func(candidate float32) {
		distance := float32(math.Abs(float64(candidate)))
		if !win.snapAnchorX && distance <= CornerSnapThreshold && distance < bestX {
			bestX, delta.X, snapX = distance, candidate, true
		}
	}
	considerY := func(candidate float32) {
		distance := float32(math.Abs(float64(candidate)))
		if !win.snapAnchorY && distance <= CornerSnapThreshold && distance < bestY {
			bestY, delta.Y, snapY = distance, candidate, true
		}
	}

	considerX(-pos.X)
	considerX(sw - pos.X - size.X)
	considerY(-pos.Y)
	considerY(sh - pos.Y - size.Y)
	rangesNear := func(a0, a1, b0, b1 float32) bool {
		return a0 <= b1+CornerSnapThreshold && a1 >= b0-CornerSnapThreshold
	}

	for _, other := range windows {
		if other == win || !other.Open {
			continue
		}
		// Other windows may opt out of UI scaling. Convert their physical
		// bounds to this window's units before comparing edges.
		op, os := other.GetPos(), other.GetSize()
		opos := point{X: op.X / s, Y: op.Y / s}
		osize := point{X: os.X / s, Y: os.Y / s}

		// Horizontal snapping
		if rangesNear(pos.Y, pos.Y+size.Y, opos.Y, opos.Y+osize.Y) {
			considerX(opos.X + osize.X - pos.X)
			considerX(opos.X - pos.X - size.X)
			considerX(opos.X - pos.X)
			considerX(opos.X + osize.X - pos.X - size.X)
		}

		// Vertical snapping
		if rangesNear(pos.X, pos.X+size.X, opos.X, opos.X+osize.X) {
			considerY(opos.Y + osize.Y - pos.Y)
			considerY(opos.Y - pos.Y - size.Y)
			considerY(opos.Y - pos.Y)
			considerY(opos.Y + osize.Y - pos.Y - size.Y)
		}
	}
	if !snapX && !snapY {
		return false
	}
	before := win.Position
	win.Position = pointAdd(win.Position, delta)
	win.clampToScreen()
	setSnapAnchor(win, before, snapX, snapY)
	return true
}

// snapMovedWindow applies move snapping after a drag update. A snapped window
// must first leave its anchor threshold before it can acquire another target.
func snapMovedWindow(win *windowData) bool {
	if !windowSnapping || win == nil || win.zone != nil {
		return false
	}
	if !win.snapAnchorActive && snapToCorner(win) {
		return true
	}
	return snapToWindow(win)
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

	bestX, bestY := resizeSnapThreshold+1, resizeSnapThreshold+1
	delta := point{}
	snapX, snapY := false, false
	considerX := func(candidate float32) {
		distance := float32(math.Abs(float64(candidate)))
		if !win.resizeSnapAnchorX && distance <= resizeSnapThreshold && distance < bestX {
			bestX, delta.X, snapX = distance, candidate, true
		}
	}
	considerY := func(candidate float32) {
		distance := float32(math.Abs(float64(candidate)))
		if !win.resizeSnapAnchorY && distance <= resizeSnapThreshold && distance < bestY {
			bestY, delta.Y, snapY = distance, candidate, true
		}
	}
	if includesLeft {
		considerX(-pos.X)
	}
	if includesRight {
		considerX(sw - pos.X - size.X)
	}
	if includesTop {
		considerY(-pos.Y)
	}
	if includesBottom {
		considerY(sh - pos.Y - size.Y)
	}
	for _, other := range windows {
		if other == win || !other.Open {
			continue
		}
		// Other windows may opt out of UI scaling. Compare in this window's units.
		op, os := other.GetPos(), other.GetSize()
		opos, osize := point{X: op.X / s, Y: op.Y / s}, point{X: os.X / s, Y: os.Y / s}
		if pos.Y < opos.Y+osize.Y && pos.Y+size.Y > opos.Y {
			if includesLeft {
				considerX(opos.X + osize.X - pos.X)
			}
			if includesRight {
				considerX(opos.X - pos.X - size.X)
			}
		}
		if pos.X < opos.X+osize.X && pos.X+size.X > opos.X {
			if includesTop {
				considerY(opos.Y + osize.Y - pos.Y)
			}
			if includesBottom {
				considerY(opos.Y - pos.Y - size.Y)
			}
		}
	}
	if !snapX && !snapY {
		return false
	}
	if delta != (point{}) {
		dragWindowResize(win, part, delta)
	}
	win.resizeSnapAnchorPosition = win.Position
	win.resizeSnapAnchorSize = win.Size
	win.resizeSnapAnchorPart = part
	win.resizeSnapAnchorX = win.resizeSnapAnchorX || snapX
	win.resizeSnapAnchorY = win.resizeSnapAnchorY || snapY
	return true
}
