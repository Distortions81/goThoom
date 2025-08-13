package eui

import "math"

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
