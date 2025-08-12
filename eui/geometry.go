package eui

import (
	"image"
	"math"
)

// containsPoint checks whether the given point lies within the rectangle.
func (r rect) containsPoint(p point) bool {
	return p.X >= r.X0 && p.Y >= r.Y0 && p.X <= r.X1 && p.Y <= r.Y1
}

// getRectangle converts a rect to the standard image.Rectangle type.
func (r rect) getRectangle() image.Rectangle {
	return image.Rectangle{
		Min: image.Point{X: int(math.Floor(float64(r.X0))), Y: int(math.Floor(float64(r.Y0)))},
		Max: image.Point{X: int(math.Ceil(float64(r.X1))), Y: int(math.Ceil(float64(r.Y1)))},
	}
}

// withinRange returns true if a and b are within the provided tolerance.
func withinRange(a, b float32, tol float32) bool {
	return math.Abs(float64(a-b)) <= float64(tol)
}

func pointAdd(a, b point) point { return point{X: a.X + b.X, Y: a.Y + b.Y} }
func pointSub(a, b point) point { return point{X: a.X - b.X, Y: a.Y - b.Y} }
func pointMul(a, b point) point { return point{X: a.X * b.X, Y: a.Y * b.Y} }
func pointDiv(a, b point) point { return point{X: a.X / b.X, Y: a.Y / b.Y} }

func pointScaleMul(a point) point { return point{X: a.X * uiScale, Y: a.Y * uiScale} }
func pointScaleDiv(a point) point { return point{X: a.X / uiScale, Y: a.Y / uiScale} }

// unionRect expands a to encompass b and returns the result.
func unionRect(a, b rect) rect {
	if b.X0 < a.X0 {
		a.X0 = b.X0
	}
	if b.Y0 < a.Y0 {
		a.Y0 = b.Y0
	}
	if b.X1 > a.X1 {
		a.X1 = b.X1
	}
	if b.Y1 > a.Y1 {
		a.Y1 = b.Y1
	}
	return a
}

// intersectRect returns the overlapping area of a and b.
// If there is no overlap, an empty rectangle is returned.
func intersectRect(a, b rect) rect {
	if a.X0 < b.X0 {
		a.X0 = b.X0
	}
	if a.Y0 < b.Y0 {
		a.Y0 = b.Y0
	}
	if a.X1 > b.X1 {
		a.X1 = b.X1
	}
	if a.Y1 > b.Y1 {
		a.Y1 = b.Y1
	}
	if a.X1 < a.X0 {
		a.X1 = a.X0
	}
	if a.Y1 < a.Y0 {
		a.Y1 = a.Y0
	}
	return a
}
