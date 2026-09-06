package main

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2/vector"
)

// The gap is relative to the body, so world movement does not churn caches.
func bubbleOutlineGap(g bubbleDrawGeometry) (x, half int, top bool) {
	if g.request.metrics.tailHalf <= 0 || g.noArrow || g.request.noArrow || g.request.far || !bubbleHasSpeakerTail(g.bubbleType) || g.bubbleType == kBubblePonder {
		return 0, 0, false
	}
	return g.baseX - g.left, g.request.metrics.tailHalf, g.attachY == g.top
}

// Start at one lip of the tail and finish at the other, leaving the joining
// edge open. Fill still uses the complete rounded rectangle.
func bubbleOutlinePath(left, top, right, bottom int, radius float32, x, half int, upper bool) vector.Path {
	if half <= 0 {
		return bubbleRoundedRectPath(left, top, right, bottom, radius)
	}
	l, t, r, b := float32(left), float32(top), float32(right), float32(bottom)
	var p vector.Path
	if upper {
		p.MoveTo(float32(x+half), t)
	} else {
		p.MoveTo(float32(x-half), b)
		p.LineTo(l+radius, b)
		p.Arc(l+radius, b-radius, radius, math.Pi/2, math.Pi, vector.Clockwise)
		p.LineTo(l, t+radius)
		p.Arc(l+radius, t+radius, radius, math.Pi, 3*math.Pi/2, vector.Clockwise)
	}
	p.LineTo(r-radius, t)
	p.Arc(r-radius, t+radius, radius, -math.Pi/2, 0, vector.Clockwise)
	p.LineTo(r, b-radius)
	p.Arc(r-radius, b-radius, radius, 0, math.Pi/2, vector.Clockwise)
	if !upper {
		p.LineTo(float32(x+half), b)
		return p
	}
	p.LineTo(l+radius, b)
	p.Arc(l+radius, b-radius, radius, math.Pi/2, math.Pi, vector.Clockwise)
	p.LineTo(l, t+radius)
	p.Arc(l+radius, t+radius, radius, math.Pi, 3*math.Pi/2, vector.Clockwise)
	p.LineTo(float32(x-half), t)
	return p
}
