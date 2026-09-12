package main

import "image"

// gameOverlayPosition anchors an overlay group to one or two edges of the
// game view. Overlays that share a position are stacked vertically in the
// order they are added.
type gameOverlayPosition uint8

const (
	gameOverlayTop gameOverlayPosition = 1 << iota
	gameOverlayBottom
	gameOverlayLeft
	gameOverlayRight
)

const (
	gameOverlayTopLeft     = gameOverlayTop | gameOverlayLeft
	gameOverlayTopRight    = gameOverlayTop | gameOverlayRight
	gameOverlayBottomLeft  = gameOverlayBottom | gameOverlayLeft
	gameOverlayBottomRight = gameOverlayBottom | gameOverlayRight
)

const gameOverlayInset = 8
const gameOverlayGap = 4

type gameOverlayReservation struct {
	position gameOverlayPosition
	size     image.Point
}

// gameOverlayLayout collects all visible overlays before assigning rectangles.
// Collecting first lets edge-only positions (top, bottom, left, and right)
// center the complete group instead of centering each overlay independently.
type gameOverlayLayout struct {
	bounds       image.Rectangle
	inset, gap   int
	reservations []gameOverlayReservation
}

func newGameOverlayLayout(bounds image.Rectangle) *gameOverlayLayout {
	return &gameOverlayLayout{bounds: bounds, inset: gameOverlayInset, gap: gameOverlayGap}
}

func (layout *gameOverlayLayout) Add(position gameOverlayPosition, size image.Point) int {
	if size.X < 0 {
		size.X = 0
	}
	if size.Y < 0 {
		size.Y = 0
	}
	handle := len(layout.reservations)
	layout.reservations = append(layout.reservations, gameOverlayReservation{
		position: normalizeGameOverlayPosition(position),
		size:     size,
	})
	return handle
}

func (layout *gameOverlayLayout) Rects() []image.Rectangle {
	result := make([]image.Rectangle, len(layout.reservations))
	var counts [16]int
	var heights [16]int
	for _, reservation := range layout.reservations {
		position := int(reservation.position)
		counts[position]++
		heights[position] += reservation.size.Y
	}
	var nextY [16]int
	for position := range counts {
		if counts[position] == 0 {
			continue
		}
		totalHeight := heights[position] + layout.gap*(counts[position]-1)
		nextY[position] = layout.bounds.Min.Y + (layout.bounds.Dy()-totalHeight)/2
		switch gameOverlayPosition(position) & (gameOverlayTop | gameOverlayBottom) {
		case gameOverlayTop:
			nextY[position] = layout.bounds.Min.Y + layout.inset
		case gameOverlayBottom:
			nextY[position] = layout.bounds.Max.Y - layout.inset - totalHeight
		}
	}

	for index, reservation := range layout.reservations {
		position := int(reservation.position)
		size := reservation.size
		x := layout.bounds.Min.X + (layout.bounds.Dx()-size.X)/2
		switch reservation.position & (gameOverlayLeft | gameOverlayRight) {
		case gameOverlayLeft:
			x = layout.bounds.Min.X + layout.inset
		case gameOverlayRight:
			x = layout.bounds.Max.X - layout.inset - size.X
		}
		y := nextY[position]
		result[index] = image.Rect(x, y, x+size.X, y+size.Y)
		nextY[position] += size.Y + layout.gap
	}
	return result
}

func normalizeGameOverlayPosition(position gameOverlayPosition) gameOverlayPosition {
	position &= gameOverlayTop | gameOverlayBottom | gameOverlayLeft | gameOverlayRight
	if position&(gameOverlayTop|gameOverlayBottom) == gameOverlayTop|gameOverlayBottom {
		position &^= gameOverlayTop | gameOverlayBottom
	}
	if position&(gameOverlayLeft|gameOverlayRight) == gameOverlayLeft|gameOverlayRight {
		position &^= gameOverlayLeft | gameOverlayRight
	}
	return position
}
