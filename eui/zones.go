package eui

import "math"

// HZone defines the horizontal zone positions.
type HZone int

const (
	HZoneLeft HZone = iota
	HZoneLeftCenter
	HZoneCenter
	HZoneRightCenter
	HZoneRight
)

// VZone defines the vertical zone positions.
type VZone int

const (
	VZoneTop VZone = iota
	VZoneTopMiddle
	VZoneMiddle
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
}

func hZoneCoord(z HZone, width int) float32 {
	switch z {
	case HZoneLeft:
		return 0
	case HZoneLeftCenter:
		return float32(width) * 0.25
	case HZoneCenter:
		return float32(width) * 0.5
	case HZoneRightCenter:
		return float32(width) * 0.75
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
		return float32(height) * 0.25
	case VZoneMiddle:
		return float32(height) * 0.5
	case VZoneBottomMiddle:
		return float32(height) * 0.75
	case VZoneBottom:
		return float32(height)
	default:
		return float32(height) * 0.5
	}
}

func nearestHZone(x float32, width int) HZone {
	zones := []HZone{HZoneLeft, HZoneLeftCenter, HZoneCenter, HZoneRightCenter, HZoneRight}
	closest := zones[0]
	min := float32(math.MaxFloat32)
	for _, z := range zones {
		diff := float32(math.Abs(float64(x - hZoneCoord(z, width))))
		if diff < min {
			min = diff
			closest = z
		}
	}
	return closest
}

func nearestVZone(y float32, height int) VZone {
	zones := []VZone{VZoneTop, VZoneTopMiddle, VZoneMiddle, VZoneBottomMiddle, VZoneBottom}
	closest := zones[0]
	min := float32(math.MaxFloat32)
	for _, z := range zones {
		diff := float32(math.Abs(float64(y - vZoneCoord(z, height))))
		if diff < min {
			min = diff
			closest = z
		}
	}
	return closest
}

func (win *windowData) PinToClosestZone() {
	cx := win.getPosition().X + win.GetSize().X/2
	cy := win.getPosition().Y + win.GetSize().Y/2
	h := nearestHZone(cx, screenWidth)
	v := nearestVZone(cy, screenHeight)
	win.SetZone(h, v)
}
