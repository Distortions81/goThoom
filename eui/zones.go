package eui

import "math"

// CornerSnapThreshold defines how close a window edge or corner must be to
// snap to a screen corner or another window.
const CornerSnapThreshold float32 = 10

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
	h HZone
	v VZone
}

// SetZone assigns a horizontal and vertical zone to the window. The window's
// center will be kept on this zone.
func (win *windowData) SetZone(h HZone, v VZone) {
	win.zone = &windowZone{h: h, v: v}
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
	win.Position.X = (cx - size.X/2) / uiScale
	win.Position.Y = (cy - size.Y/2) / uiScale

	maxX := (float32(screenWidth) - size.X) / uiScale
	maxY := (float32(screenHeight) - size.Y) / uiScale
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

func nearestHZone(x, w float32, width int) HZone {
	zones := []HZone{HZoneLeft, HZoneLeftCenter, HZoneCenterLeft, HZoneCenter, HZoneCenterRight, HZoneRightCenter, HZoneRight}
	closest := zones[0]
	min := float32(math.MaxFloat32)
	positions := []float32{x, x + w/2, x + w}
	for _, z := range zones {
		zx := hZoneCoord(z, width)
		for _, px := range positions {
			diff := float32(math.Abs(float64(px - zx)))
			if diff < min {
				min = diff
				closest = z
			}
		}
	}
	return closest
}

func nearestVZone(y, h float32, height int) VZone {
	zones := []VZone{VZoneTop, VZoneTopMiddle, VZoneMiddleTop, VZoneCenter, VZoneMiddleBottom, VZoneBottomMiddle, VZoneBottom}
	closest := zones[0]
	min := float32(math.MaxFloat32)
	positions := []float32{y, y + h/2, y + h}
	for _, z := range zones {
		zy := vZoneCoord(z, height)
		for _, py := range positions {
			diff := float32(math.Abs(float64(py - zy)))
			if diff < min {
				min = diff
				closest = z
			}
		}
	}
	return closest
}

func (win *windowData) PinToClosestZone() {
	pos := win.getPosition()
	size := win.GetSize()
	h := nearestHZone(pos.X, size.X, screenWidth)
	v := nearestVZone(pos.Y, size.Y, screenHeight)
	win.SetZone(h, v)
}

// snapToCorner assigns a zone when a window is dragged close to a screen
// corner. It returns true if a zone was applied.
func snapToCorner(win *windowData) bool {
	pos := win.getPosition()
	size := win.GetSize()

	sw := float32(screenWidth)
	sh := float32(screenHeight)

	// Top-left
	if pos.X <= CornerSnapThreshold && pos.Y <= CornerSnapThreshold {
		win.SetZone(HZoneLeft, VZoneTop)
		return true
	}
	// Top-right
	if pos.X+size.X >= sw-CornerSnapThreshold && pos.Y <= CornerSnapThreshold {
		win.SetZone(HZoneRight, VZoneTop)
		return true
	}
	// Bottom-left
	if pos.X <= CornerSnapThreshold && pos.Y+size.Y >= sh-CornerSnapThreshold {
		win.SetZone(HZoneLeft, VZoneBottom)
		return true
	}
	// Bottom-right
	if pos.X+size.X >= sw-CornerSnapThreshold && pos.Y+size.Y >= sh-CornerSnapThreshold {
		win.SetZone(HZoneRight, VZoneBottom)
		return true
	}
	return false
}

// snapToWindow snaps a window's edges to nearby windows within the threshold.
// It returns true if the window position was adjusted.
func snapToWindow(win *windowData) bool {
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
	pos := win.getPosition()
	size := win.Size
	snapped := false

	includesLeft := part == PART_LEFT || part == PART_TOP_LEFT || part == PART_BOTTOM_LEFT
	includesRight := part == PART_RIGHT || part == PART_TOP_RIGHT || part == PART_BOTTOM_RIGHT
	includesTop := part == PART_TOP || part == PART_TOP_LEFT || part == PART_TOP_RIGHT
	includesBottom := part == PART_BOTTOM || part == PART_BOTTOM_LEFT || part == PART_BOTTOM_RIGHT

	// Snap to screen edges
	if includesLeft {
		if math.Abs(float64(pos.X)) <= float64(CornerSnapThreshold) {
			delta := pos.X
			win.Position.X = 0
			win.setSize(point{X: size.X - delta, Y: size.Y})
			pos = win.getPosition()
			size = win.Size
			snapped = true
		}
	}
	if includesRight {
		right := pos.X + size.X
		sw := float32(screenWidth)
		if math.Abs(float64(sw-right)) <= float64(CornerSnapThreshold) {
			win.setSize(point{X: sw - pos.X, Y: size.Y})
			size = win.Size
			snapped = true
		}
	}
	if includesTop {
		if math.Abs(float64(pos.Y)) <= float64(CornerSnapThreshold) {
			delta := pos.Y
			win.Position.Y = 0
			win.setSize(point{X: size.X, Y: size.Y - delta})
			pos = win.getPosition()
			size = win.Size
			snapped = true
		}
	}
	if includesBottom {
		bottom := pos.Y + size.Y
		sh := float32(screenHeight)
		if math.Abs(float64(sh-bottom)) <= float64(CornerSnapThreshold) {
			win.setSize(point{X: size.X, Y: sh - pos.Y})
			size = win.Size
			snapped = true
		}
	}

	// Snap to other windows
	for _, other := range windows {
		if other == win || !other.Open {
			continue
		}
		opos := other.getPosition()
		osize := other.Size

		if includesLeft {
			if pos.Y < opos.Y+osize.Y && pos.Y+size.Y > opos.Y {
				target := opos.X + osize.X
				if math.Abs(float64(pos.X-target)) <= float64(CornerSnapThreshold) {
					delta := pos.X - target
					win.Position.X = target
					win.setSize(point{X: size.X - delta, Y: size.Y})
					pos = win.getPosition()
					size = win.Size
					snapped = true
				}
			}
		}
		if includesRight {
			if pos.Y < opos.Y+osize.Y && pos.Y+size.Y > opos.Y {
				target := opos.X
				right := pos.X + size.X
				if math.Abs(float64(right-target)) <= float64(CornerSnapThreshold) {
					win.setSize(point{X: target - pos.X, Y: size.Y})
					size = win.Size
					snapped = true
				}
			}
		}
		if includesTop {
			if pos.X < opos.X+osize.X && pos.X+size.X > opos.X {
				target := opos.Y + osize.Y
				if math.Abs(float64(pos.Y-target)) <= float64(CornerSnapThreshold) {
					delta := pos.Y - target
					win.Position.Y = target
					win.setSize(point{X: size.X, Y: size.Y - delta})
					pos = win.getPosition()
					size = win.Size
					snapped = true
				}
			}
		}
		if includesBottom {
			if pos.X < opos.X+osize.X && pos.X+size.X > opos.X {
				target := opos.Y
				bottom := pos.Y + size.Y
				if math.Abs(float64(bottom-target)) <= float64(CornerSnapThreshold) {
					win.setSize(point{X: size.X, Y: target - pos.Y})
					size = win.Size
					snapped = true
				}
			}
		}
	}

	return snapped
}
