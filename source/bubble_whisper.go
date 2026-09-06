package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"math"
)

// Whisper's quiet dotted edge is painted once into the shared body cache.
func drawWhisperBody(screen *ebiten.Image, g bubbleDrawGeometry) {
	left, top, right, bottom := g.left+g.offsetX, g.top+g.offsetY, g.right+g.offsetX, g.bottom+g.offsetY
	body := bubbleRoundedRectPath(left, top, right, bottom, g.radius)
	fill := &vector.DrawPathOptions{AntiAlias: true}
	fill.ColorScale.ScaleWithColor(g.fillColor)
	vector.FillPath(screen, &body, nil, fill)
	var dots vector.Path
	radius := float32(math.Max(.55, .55*float64(g.scale)))
	gapX, gapHalf, _ := bubbleOutlineGap(g)
	dot := func(x, y float32) {
		if gapHalf > 0 && y == float32(g.attachY+g.offsetY) && x+radius > float32(left+gapX-gapHalf) && x-radius < float32(left+gapX+gapHalf) {
			return
		}
		dots.MoveTo(x+radius, y)
		dots.Arc(x, y, radius, 0, 2*math.Pi, vector.Clockwise)
		dots.Close()
	}
	edge := func(x0, y0, x1, y1 float32) {
		length := float32(math.Hypot(float64(x1-x0), float64(y1-y0)))
		count := max(1, int(math.Round(float64(length/(3.5*g.scale)))))
		for i := 0; i <= count; i++ {
			f := float32(i) / float32(count)
			dot(x0+(x1-x0)*f, y0+(y1-y0)*f)
		}
	}
	inset := max(2*g.scale, g.radius)
	edge(float32(left)+inset, float32(top), float32(right)-inset, float32(top))
	edge(float32(left)+inset, float32(bottom), float32(right)-inset, float32(bottom))
	edge(float32(left), float32(top)+inset, float32(left), float32(bottom)-inset)
	edge(float32(right), float32(top)+inset, float32(right), float32(bottom)-inset)
	outline := &vector.DrawPathOptions{AntiAlias: true}
	outline.ColorScale.ScaleWithColor(g.borderColor)
	vector.FillPath(screen, &dots, nil, outline)
}
