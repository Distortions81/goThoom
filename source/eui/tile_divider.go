package eui

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

// TileDividerOrientation describes the direction of a tiled-workspace gutter.
type TileDividerOrientation uint8

const (
	TileDividerHorizontal TileDividerOrientation = iota
	TileDividerVertical
)

// TileDivider is a screen-space divider owned by a tiled workspace. Position
// is the Y coordinate for a horizontal divider and X for a vertical divider;
// Start and End delimit its span on the other axis.
type TileDivider struct {
	Orientation TileDividerOrientation
	Position    float32
	Start       float32
	End         float32
	Thickness   float32
	HitSize     float32
	OnDrag      func(position float32)
}

var (
	tileDividers       []TileDivider
	hoveredTileDivider = -1
	activeTileDivider  = -1
)

// SetTileDividers replaces the active workspace gutters. Passing nil removes
// them. Dividers are copied so callers can reuse their input slice.
func SetTileDividers(dividers []TileDivider) {
	tileDividers = append(tileDividers[:0], dividers...)
	if activeTileDivider >= len(tileDividers) {
		activeTileDivider = -1
	}
	if hoveredTileDivider >= len(tileDividers) {
		hoveredTileDivider = -1
	}
}

func tileDividerHit(d TileDivider, p point) bool {
	hit := d.HitSize
	if hit <= 0 {
		hit = 12 * UIScale()
	}
	half := hit / 2
	if d.Orientation == TileDividerVertical {
		return p.X >= d.Position-half && p.X <= d.Position+half && p.Y >= d.Start && p.Y <= d.End
	}
	return p.Y >= d.Position-half && p.Y <= d.Position+half && p.X >= d.Start && p.X <= d.End
}

func tileDividerAt(p point) int {
	for i := len(tileDividers) - 1; i >= 0; i-- {
		if tileDividerHit(tileDividers[i], p) {
			return i
		}
	}
	return -1
}

func tileDividerCoveredByStandaloneWindow(p point) bool {
	for _, win := range windows {
		if win.Open && !win.Docked && win.getWinRect().containsPoint(p) {
			return true
		}
	}
	return false
}

func updateTileDividerInput(p point, click bool) (handled bool, cursor ebiten.CursorShapeType) {
	if activeTileDivider >= 0 && !pointerPressed() {
		activeTileDivider = -1
	}
	if activeTileDivider < 0 && (tileDividerCoveredByStandaloneWindow(p) || dropdownOpenContainsAnywhere(p) || contextMenuContainsAnywhere(p)) {
		hoveredTileDivider = -1
		return false, ebiten.CursorShapeDefault
	}
	hoveredTileDivider = tileDividerAt(p)
	if click && hoveredTileDivider >= 0 {
		activeTileDivider = hoveredTileDivider
	}
	if activeTileDivider >= 0 && activeTileDivider < len(tileDividers) {
		d := tileDividers[activeTileDivider]
		position := p.Y
		cursor = ebiten.CursorShapeNSResize
		if d.Orientation == TileDividerVertical {
			position = p.X
			cursor = ebiten.CursorShapeEWResize
		}
		if d.OnDrag != nil {
			d.OnDrag(position)
		}
		return true, cursor
	}
	if hoveredTileDivider >= 0 {
		if tileDividers[hoveredTileDivider].Orientation == TileDividerVertical {
			return false, ebiten.CursorShapeEWResize
		}
		return false, ebiten.CursorShapeNSResize
	}
	return false, ebiten.CursorShapeDefault
}

func drawTileDividers(screen *ebiten.Image) {
	accentWidth := max(float32(1), UIScale())
	for i, d := range tileDividers {
		thickness := d.Thickness
		if thickness <= 0 {
			thickness = 4 * UIScale()
		}
		base := color.RGBA{R: 24, G: 27, B: 32, A: 255}
		accent := AccentColor().ToRGBA()
		if i != hoveredTileDivider && i != activeTileDivider {
			accent.A = 150
		}
		if d.Orientation == TileDividerVertical {
			drawFilledRect(screen, d.Position-thickness/2, d.Start, thickness, d.End-d.Start, base, false)
			drawFilledRect(screen, d.Position-accentWidth/2, d.Start, accentWidth, d.End-d.Start, accent, false)
			continue
		}
		drawFilledRect(screen, d.Start, d.Position-thickness/2, d.End-d.Start, thickness, base, false)
		drawFilledRect(screen, d.Start, d.Position-accentWidth/2, d.End-d.Start, accentWidth, accent, false)
	}
}
