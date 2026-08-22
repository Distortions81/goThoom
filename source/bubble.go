package main

import (
	"gothoom/eui"
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// whiteImage is a reusable 1x1 white pixel used across the UI for drawing
// solid rectangles and lines without creating multiple images.
var whiteImage *ebiten.Image
var blackImage *ebiten.Image
var grayImage *ebiten.Image

func init() {
	whiteImage = newImage(1, 1)
	whiteImage.Fill(color.White)

	blackImage = newImage(1, 1)
	blackImage.Fill(color.Black)

	grayImage = newImage(1, 1)
	grayImage.Fill(eui.Color{R: 128, G: 128, B: 128})
}

// adjustBubbleRect calculates the on-screen rectangle for a bubble and clamps
// it to the visible area. The tail tip coordinates remain unchanged and must
// be handled by the caller if needed. Set noTail when the bubble has no arrow
// pointing to a character so the rectangle is based directly on (x, y).
func adjustBubbleRect(x, y, width, height, tailHeight, sw, sh int, noTail bool) (left, top, right, bottom int) {
	bottom = y
	if !noTail {
		bottom = y - tailHeight
	}
	left = x - width/2
	top = bottom - height

	if left < 0 {
		left = 0
	}
	if left+width > sw {
		left = sw - width
	}
	if top < 0 {
		top = 0
	}
	if top+height > sh {
		top = sh - height
	}

	right = left + width
	bottom = top + height
	return
}

// bubbleColors selects the border, background, and text colors for a bubble
// based on its type. Alpha values are premultiplied to match Ebiten's color
// expectations.

func bubbleColors(typ int) (border, bg, text color.Color) {
	alpha := uint8(gs.BubbleOpacity * 255)
	if gs.DarkBubblesAndNames {
		bg = color.NRGBA{0x24, 0x24, 0x24, alpha}
		text = color.White
		switch typ & kBubbleTypeMask {
		case kBubbleWhisper:
			border = color.NRGBA{0x80, 0x80, 0x80, 0xff}
		case kBubbleYell:
			border = color.NRGBA{0xff, 0xff, 0x00, 0xff}
		case kBubbleThought:
			border = color.NRGBA{0x00, 0x00, 0x00, 0x00}
		case kBubblePonder:
			border = color.NRGBA{0x24, 0x24, 0x24, alpha}
		case kBubbleRealAction:
			border = color.NRGBA{0x00, 0x00, 0x80, 0xff}
		case kBubblePlayerAction:
			border = color.NRGBA{0x80, 0x00, 0x00, 0xff}
		case kBubbleNarrate:
			border = color.NRGBA{0x00, 0x80, 0x00, 0xff}
		case kBubbleMonster:
			border = color.NRGBA{0xd6, 0xd6, 0xd6, 0xff}
		default:
			border = color.White
		}
		return
	}
	switch typ & kBubbleTypeMask {
	case kBubbleWhisper:
		border = color.NRGBA{0x80, 0x80, 0x80, 0xff}
		bg = color.NRGBA{0x33, 0x33, 0x33, alpha}
		text = color.White
	case kBubbleYell:
		border = color.NRGBA{0xff, 0xff, 0x00, 0xff}
		bg = color.NRGBA{0xff, 0xff, 0xff, alpha}
		text = color.Black
	case kBubbleThought:
		border = color.NRGBA{0x00, 0x00, 0x00, 0x00}
		bg = color.NRGBA{0x80, 0x80, 0x80, alpha}
		text = color.Black
	case kBubblePonder:
		border = color.NRGBA{0xcc, 0xcc, 0xcc, alpha}
		bg = color.NRGBA{0xcc, 0xcc, 0xcc, alpha}
		text = color.Black
	case kBubbleRealAction:
		border = color.NRGBA{0x00, 0x00, 0x80, 0xff}
		bg = color.NRGBA{0xff, 0xff, 0xff, alpha}
		text = color.Black
	case kBubblePlayerAction:
		border = color.NRGBA{0x80, 0x00, 0x00, 0xff}
		bg = color.NRGBA{0xff, 0xff, 0xff, alpha}
		text = color.Black
	case kBubbleNarrate:
		border = color.NRGBA{0x00, 0x80, 0x00, 0xff}
		bg = color.NRGBA{0xff, 0xff, 0xff, alpha}
		text = color.Black
	case kBubbleMonster:
		border = color.NRGBA{0xd6, 0xd6, 0xd6, 0xff}
		bg = color.NRGBA{0x47, 0x47, 0x47, alpha}
		text = color.White
	default:
		border = color.White
		bg = color.NRGBA{0xff, 0xff, 0xff, alpha}
		text = color.Black
	}
	return
}

// scaledBubbleFace returns a text face scaled by the provided factor. When the
// scale is 1 the original face is returned. The returned face shares the same
// font source but uses an adjusted size so text is rasterized at the desired
// resolution instead of being drawn and then scaled, which avoids blurring.
func scaledBubbleFace(face text.Face, scale float64) text.Face {
	if face == nil {
		return nil
	}
	if scale <= 0 {
		scale = 1
	}
	if math.Abs(scale-1) < 1e-9 {
		return face
	}
	if gf, ok := face.(*text.GoTextFace); ok {
		scaled := *gf
		scaled.Size = gf.Size * scale
		return &scaled
	}
	return face
}

// drawBubble renders a text bubble anchored so that (x, y) corresponds to the
// bottom-center point of the balloon tail. If the bubble would extend past the
// screen edges it is clamped while leaving the tail anchored at (x, y). If far
// is true the tail is omitted and (x, y) represents the bottom-center of the
// bubble itself. The tail can also be skipped explicitly via noArrow. The typ
// parameter is currently unused but retained for future compatibility with the
// original bubble images. The colors of the border, background, and text can be
// customized via borderCol, bgCol, and textCol respectively. fontScale controls
// the font size so text is rasterized at native resolution for the current
// window scale.
func drawBubble(screen *ebiten.Image, txt string, x, y int, typ int, far bool, noArrow bool, borderCol, bgCol, textCol color.Color, bubbleScale, fontScale float64) {
	if txt == "" {
		return
	}
	bounds := screen.Bounds()
	offsetX := bounds.Min.X
	offsetY := bounds.Min.Y
	sw := bounds.Dx()
	sh := bounds.Dy()
	if sw <= 0 || sh <= 0 {
		return
	}
	if bubbleScale <= 0 {
		bubbleScale = 0.1
	}
	if fontScale <= 0 {
		fontScale = 0.1
	}

	tailX, tailY := x, y
	if tailX < 0 || tailX >= sw || tailY < 0 || tailY >= sh {
		noArrow = true
	}
	// Visual scale for bubbles independent of font size
	s := bubbleScale
	pad := int(math.Round(6 * s))
	if pad < 1 {
		pad = 1
	}
	tailHeight := int(math.Round(10 * s))
	if tailHeight < 1 {
		tailHeight = 1
	}
	tailHalf := int(math.Round(6 * s))
	if tailHalf < 1 {
		tailHalf = 1
	}
	bubbleType := typ & kBubbleTypeMask

	// Allow wider bubbles with larger scale; preserve font size
	maxLineWidth := int(math.Round(float64(gameAreaSizeX)/4*s)) - 2*pad
	if maxLineWidth < 1 {
		maxLineWidth = 1
	}
	font := bubbleFont
	if bubbleType == kBubbleWhisper {
		font = bubbleFontRegular
	}
	font = scaledBubbleFace(font, fontScale)
	if font == nil {
		if bubbleType == kBubbleWhisper {
			font = bubbleFontRegular
		} else {
			font = bubbleFont
		}
	}
	available := float64(maxLineWidth)
	if available < 1 {
		available = 1
	}
	baseWidth, lines := wrapText(txt, font, available)
	width := int(math.Ceil(float64(baseWidth)))
	width += 2 * pad
	metrics := font.Metrics()
	baseLineHeight := math.Ceil(metrics.HAscent) + math.Ceil(metrics.HDescent) + math.Ceil(metrics.HLineGap)
	lineHeight := int(math.Ceil(baseLineHeight))
	if lineHeight < 1 {
		lineHeight = 1
	}
	height := lineHeight*len(lines) + 2*pad

	left, top, right, bottom := adjustBubbleRect(x, y, width, height, tailHeight, sw, sh, far || noArrow)
	baseX := left + width/2

	bgR, bgG, bgB, bgA := bgCol.RGBA()
	bdR, bdG, bdB, bdA := borderCol.RGBA()

	radius := float32(4 * s)
	if bubbleType == kBubblePonder {
		radius = float32(8 * s)
	}

	fx := float32(offsetX)
	fy := float32(offsetY)

	var body vector.Path
	body.MoveTo(float32(left)+radius+fx, float32(top)+fy)
	body.LineTo(float32(right)-radius+fx, float32(top)+fy)
	body.Arc(float32(right)-radius+fx, float32(top)+radius+fy, radius, -math.Pi/2, 0, vector.Clockwise)
	body.LineTo(float32(right)+fx, float32(bottom)-radius+fy)
	body.Arc(float32(right)-radius+fx, float32(bottom)-radius+fy, radius, 0, math.Pi/2, vector.Clockwise)
	body.LineTo(float32(left)+radius+fx, float32(bottom)+fy)
	body.Arc(float32(left)+radius+fx, float32(bottom)-radius+fy, radius, math.Pi/2, math.Pi, vector.Clockwise)
	body.LineTo(float32(left)+fx, float32(top)+radius+fy)
	body.Arc(float32(left)+radius+fx, float32(top)+radius+fy, radius, math.Pi, 3*math.Pi/2, vector.Clockwise)
	body.Close()

	var tail vector.Path
	if !far && !noArrow {
		if bubbleType == kBubblePonder {
			r1 := float32(tailHalf)
			phase := float64(time.Now().UnixNano()) / float64(time.Second)
			offset1 := r1 * 0.3 * float32(math.Sin(phase))
			cx1 := float32(baseX) + fx
			// Bias ponder tail circles closer to the mobile so the origin is
			// easier to see. Space the first (largest) circle at ~20% of the
			// way from the bubble bottom to the tail tip instead of directly
			// hugging the bubble.
			dist := float32(tailY - bottom)
			if dist < 0 {
				dist = 0
			}
			cy1 := float32(bottom) + r1 + dist*0.2 - offset1 + fy
			tail.MoveTo(cx1+r1, cy1)
			tail.Arc(cx1, cy1, r1, 0, 2*math.Pi, vector.Clockwise)
			tail.Close()
			rMid := r1 * 0.6
			offsetMid := rMid * 0.5 * float32(math.Sin(phase+math.Pi/4))
			cxMid := float32(baseX+tailX)/2 + fx
			// Place the middle circle at ~65% down the path toward the tail.
			cyMid := float32(bottom) + dist*0.65 - offsetMid + fy
			tail.MoveTo(cxMid+rMid, cyMid)
			tail.Arc(cxMid, cyMid, rMid, 0, 2*math.Pi, vector.Clockwise)
			tail.Close()
			r2 := float32(tailHalf) / 2
			offset2 := r2 * 0.6 * float32(math.Sin(phase+math.Pi/2))
			cx2 := float32(tailX) + fx
			cy2 := float32(tailY) - offset2 + fy
			tail.MoveTo(cx2+r2, cy2)
			tail.Arc(cx2, cy2, r2, 0, 2*math.Pi, vector.Clockwise)
			tail.Close()
		} else {
			tail.MoveTo(float32(baseX-tailHalf)+fx, float32(bottom)+fy)
			tail.LineTo(float32(tailX)+fx, float32(tailY)+fy)
			tail.LineTo(float32(baseX+tailHalf)+fx, float32(bottom)+fy)
			tail.Close()
		}
	}

	fillColor := color.RGBA64{R: uint16(bgR), G: uint16(bgG), B: uint16(bgB), A: uint16(bgA)}
	borderColor := color.RGBA64{R: uint16(bdR), G: uint16(bdG), B: uint16(bdB), A: uint16(bdA)}

	if bubbleType != kBubblePonder {
		fillOp := &vector.DrawPathOptions{AntiAlias: true}
		fillOp.ColorScale.ScaleWithColor(fillColor)
		vector.FillPath(screen, &body, nil, fillOp)
	}
	if !far && !noArrow {
		tailOp := &vector.DrawPathOptions{AntiAlias: true}
		tailOp.ColorScale.ScaleWithColor(fillColor)
		vector.FillPath(screen, &tail, nil, tailOp)
	}
	if bubbleType != kBubblePonder {
		var outline vector.Path
		outline.MoveTo(float32(left)+radius+fx, float32(top)+fy)
		outline.LineTo(float32(right)-radius+fx, float32(top)+fy)
		outline.Arc(float32(right)-radius+fx, float32(top)+radius+fy, radius, -math.Pi/2, 0, vector.Clockwise)
		outline.LineTo(float32(right)+fx, float32(bottom)-radius+fy)
		outline.Arc(float32(right)-radius+fx, float32(bottom)-radius+fy, radius, 0, math.Pi/2, vector.Clockwise)
		if !far && !noArrow {
			outline.LineTo(float32(baseX+tailHalf)+fx, float32(bottom)+fy)
			outline.LineTo(float32(tailX)+fx, float32(tailY)+fy)
			outline.LineTo(float32(baseX-tailHalf)+fx, float32(bottom)+fy)
		}
		outline.LineTo(float32(left)+radius+fx, float32(bottom)+fy)
		outline.Arc(float32(left)+radius+fx, float32(bottom)-radius+fy, radius, math.Pi/2, math.Pi, vector.Clockwise)
		outline.LineTo(float32(left)+fx, float32(top)+radius+fy)
		outline.Arc(float32(left)+radius+fx, float32(top)+radius+fy, radius, math.Pi, 3*math.Pi/2, vector.Clockwise)
		outline.Close()

		// Thicken outline a bit with scale
		strokeW := float32(math.Max(1, s))
		strokeOp := &vector.StrokeOptions{Width: strokeW}
		drawOutline := &vector.DrawPathOptions{AntiAlias: true}
		drawOutline.ColorScale.ScaleWithColor(borderColor)
		vector.StrokePath(screen, &outline, strokeOp, drawOutline)
	} else {
		drawPonderWaves(screen, left+offsetX, top+offsetY, right+offsetX, bottom+offsetY, bgCol, s)
	}

	if bubbleType == kBubbleYell {
		gapStart, gapEnd := float32(-1), float32(-1)
		if !far && !noArrow {
			gapStart = float32(baseX-tailHalf) + fx
			gapEnd = float32(baseX+tailHalf) + fx
		}
		drawSpikes(screen, float32(left)+fx, float32(top)+fy, float32(right)+fx, float32(bottom)+fy, radius, 3*float32(s), borderCol, gapStart, gapEnd)
	} else if bubbleType == kBubbleMonster {
		gapStart, gapEnd := float32(-1), float32(-1)
		if !far && !noArrow {
			gapStart = float32(baseX-tailHalf) + fx
			gapEnd = float32(baseX+tailHalf) + fx
		}
		drawMonsterSpikes(screen, float32(left)+fx, float32(top)+fy, float32(right)+fx, float32(bottom)+fy, radius, 4*float32(s), borderCol, gapStart, gapEnd)
	}

	textTop := top + pad + offsetY
	textLeft := left + pad + offsetX
	for i, line := range lines {
		op := &text.DrawOptions{}
		op.GeoM.Translate(float64(textLeft), float64(textTop+i*lineHeight))
		op.ColorScale.ScaleWithColor(textCol)
		text.Draw(screen, line, font, op)
	}
}

// drawSpikes renders spiky triangles around the bubble rectangle to emphasize
// a shouted yell. Triangles are drawn pointing outward along each edge and
// around the rounded corners using the given border color. The spike length
// gently pulses over time to enhance the yelling effect. bottomGapStart and
// bottomGapEnd define a segment along the bottom edge where spikes should be
// omitted (e.g. where the tail arrow attaches).
func drawSpikes(screen *ebiten.Image, left, top, right, bottom, radius, size float32, col color.Color, bottomGapStart, bottomGapEnd float32) {
	bdR, bdG, bdB, bdA := col.RGBA()
	step := size
	phase := float64(time.Now().UnixNano()) / float64(time.Second) * 4
	spikeBase := size + size*0.3*float32(math.Sin(phase))

	drawOp := &vector.DrawPathOptions{AntiAlias: true}
	drawOp.ColorScale.Scale(float32(bdR)/0xffff, float32(bdG)/0xffff, float32(bdB)/0xffff, float32(bdA)/0xffff)

	drawTriangle := func(x1, y1, x2, y2, x3, y3 float32) {
		var p vector.Path
		p.MoveTo(x1, y1)
		p.LineTo(x2, y2)
		p.LineTo(x3, y3)
		p.Close()
		vector.FillPath(screen, &p, nil, drawOp)
	}

	startX := left + radius
	endX := right - radius
	for x := startX; x < endX; x += step {
		end := x + step
		mid := x + step/2
		if end > endX {
			end = endX
			mid = x + (end-x)/2
		}
		drawTriangle(x, top, mid, top-spikeBase, end, top)
	}

	if bottomGapStart < startX {
		bottomGapStart = startX
	}
	if bottomGapEnd < bottomGapStart {
		bottomGapEnd = bottomGapStart
	}
	if bottomGapEnd > endX {
		bottomGapEnd = endX
	}
	drawBottom := func(segStart, segEnd float32) {
		for x := segStart; x < segEnd; x += step {
			spike := size * (0.7 + 0.3*float32(math.Sin(phase+float64(x-startX))))
			end := x + step
			mid := x + step/2
			if end > segEnd {
				end = segEnd
				mid = x + (end-x)/2
			}
			drawTriangle(x, bottom, mid, bottom+spike, end, bottom)
		}
	}
	drawBottom(startX, bottomGapStart)
	drawBottom(bottomGapEnd, endX)

	startY := top + radius
	endY := bottom - radius
	for y := startY; y < endY; y += step {
		spike := size * (0.7 + 0.3*float32(math.Sin(phase+float64(y-startY))))
		end := y + step
		mid := y + step/2
		if end > endY {
			end = endY
			mid = y + (end-y)/2
		}

		drawTriangle(left, y, left-spike, mid, left, end)
		drawTriangle(right, y, right+spike, mid, right, end)
	}

	if radius <= 0 {
		return
	}
	corner := func(cx, cy float32, start, end float64) {
		stepAngle := float64(step) / float64(radius)
		for a := start; a < end; a += stepAngle {
			next := a + stepAngle
			if next > end {
				next = end
			}
			mid := a + (next-a)/2
			spike := size * (0.7 + 0.3*float32(math.Sin(phase+mid)))
			x1 := cx + radius*float32(math.Cos(a))
			y1 := cy + radius*float32(math.Sin(a))
			x2 := cx + radius*float32(math.Cos(next))
			y2 := cy + radius*float32(math.Sin(next))
			mx := cx + (radius+spike)*float32(math.Cos(mid))
			my := cy + (radius+spike)*float32(math.Sin(mid))

			drawTriangle(x1, y1, mx, my, x2, y2)
		}
	}

	corner(left+radius, top+radius, math.Pi, 1.5*math.Pi)
	corner(right-radius, top+radius, 1.5*math.Pi, 2*math.Pi)
	corner(right-radius, bottom-radius, 0, 0.5*math.Pi)
	corner(left+radius, bottom-radius, 0.5*math.Pi, math.Pi)
}

func drawMonsterSpikes(screen *ebiten.Image, left, top, right, bottom, radius, size float32, col color.Color, bottomGapStart, bottomGapEnd float32) {
	bdR, bdG, bdB, bdA := col.RGBA()
	step := size / 2
	phase := float64(time.Now().UnixNano()) / float64(time.Second)

	drawOp := &vector.DrawPathOptions{AntiAlias: true}
	drawOp.ColorScale.Scale(float32(bdR)/0xffff, float32(bdG)/0xffff, float32(bdB)/0xffff, float32(bdA)/0xffff)

	drawTriangle := func(x1, y1, x2, y2, x3, y3 float32) {
		var p vector.Path
		p.MoveTo(x1, y1)
		p.LineTo(x2, y2)
		p.LineTo(x3, y3)
		p.Close()
		vector.FillPath(screen, &p, nil, drawOp)
	}

	startX := left + radius
	endX := right - radius
	for x := startX; x < endX; x += step {
		spike := size * (0.7 + 0.3*float32(math.Sin(phase+float64(x-startX))))
		end := x + step
		mid := x + step/2
		if end > endX {
			end = endX
			mid = x + (end-x)/2
		}
		drawTriangle(x, top, mid, top-spike, end, top)
	}

	if bottomGapStart < startX {
		bottomGapStart = startX
	}
	if bottomGapEnd < bottomGapStart {
		bottomGapEnd = bottomGapStart
	}
	if bottomGapEnd > endX {
		bottomGapEnd = endX
	}
	drawBottom := func(segStart, segEnd float32) {
		for x := segStart; x < segEnd; x += step {
			spike := size * (0.7 + 0.3*float32(math.Sin(phase+float64(x-startX))))
			end := x + step
			mid := x + step/2
			if end > segEnd {
				end = segEnd
				mid = x + (end-x)/2
			}
			drawTriangle(x, bottom, mid, bottom+spike, end, bottom)
		}
	}
	drawBottom(startX, bottomGapStart)
	drawBottom(bottomGapEnd, endX)

	startY := top + radius
	endY := bottom - radius
	for y := startY; y < endY; y += step {
		spike := size * (0.7 + 0.3*float32(math.Sin(phase+float64(y-startY))))
		end := y + step
		mid := y + step/2
		if end > endY {
			end = endY
			mid = y + (end-y)/2
		}

		drawTriangle(left, y, left-spike, mid, left, end)
		drawTriangle(right, y, right+spike, mid, right, end)
	}

	if radius <= 0 {
		return
	}
	corner := func(cx, cy float32, start, end float64) {
		stepAngle := float64(step) / float64(radius)
		for a := start; a < end; a += stepAngle {
			next := a + stepAngle
			if next > end {
				next = end
			}
			mid := a + (next-a)/2
			spike := size * (0.7 + 0.3*float32(math.Sin(phase+mid)))
			x1 := cx + radius*float32(math.Cos(a))
			y1 := cy + radius*float32(math.Sin(a))
			x2 := cx + radius*float32(math.Cos(next))
			y2 := cy + radius*float32(math.Sin(next))
			mx := cx + (radius+spike)*float32(math.Cos(mid))
			my := cy + (radius+spike)*float32(math.Sin(mid))

			drawTriangle(x1, y1, mx, my, x2, y2)
		}
	}

	corner(left+radius, top+radius, math.Pi, 1.5*math.Pi)
	corner(right-radius, top+radius, 1.5*math.Pi, 2*math.Pi)
	corner(right-radius, bottom-radius, 0, 0.5*math.Pi)
	corner(left+radius, bottom-radius, 0.5*math.Pi, math.Pi)
}

func drawPonderWaves(screen *ebiten.Image, left, top, right, bottom int, col color.Color, bubbleScale float64) {
	colR, colG, colB, colA := col.RGBA()
	waveColor := color.RGBA64{R: uint16(colR), G: uint16(colG), B: uint16(colB), A: uint16(colA)}
	if bubbleScale <= 0 {
		bubbleScale = 0.1
	}
	s := float32(bubbleScale)
	radius := float32(8) * s
	var body vector.Path
	body.MoveTo(float32(left)+radius, float32(top))
	body.LineTo(float32(right)-radius, float32(top))
	body.Arc(float32(right)-radius, float32(top)+radius, radius, -math.Pi/2, 0, vector.Clockwise)
	body.LineTo(float32(right), float32(bottom)-radius)
	body.Arc(float32(right)-radius, float32(bottom)-radius, radius, 0, math.Pi/2, vector.Clockwise)
	body.LineTo(float32(left)+radius, float32(bottom))
	body.Arc(float32(left)+radius, float32(bottom)-radius, radius, math.Pi/2, math.Pi, vector.Clockwise)
	body.LineTo(float32(left), float32(top)+radius)
	body.Arc(float32(left)+radius, float32(top)+radius, radius, math.Pi, 3*math.Pi/2, vector.Clockwise)
	body.Close()
	bodyOp := &vector.DrawPathOptions{
		AntiAlias: true,
	}
	bodyOp.ColorScale.ScaleWithColor(waveColor)
	vector.FillPath(screen, &body, nil, bodyOp)

	r := float32(6) * s
	step := r * 1.2
	phase := float64(time.Now().UnixNano()) / float64(time.Second)
	corner := float32(10) * s
	angleStep := float64(step / corner)

	draw := func(cx, cy float32) {
		drawBubbleCircle(screen, cx, cy, r, waveColor)
	}

	// top edge
	for x := float32(left) + corner; x <= float32(right)-corner; x += step {
		offset := float32(math.Sin(phase+float64(x-float32(left))*0.1)) * r * 0.3
		draw(x, float32(top)+offset)
	}
	// top-right corner
	for a := -math.Pi / 2; a < 0; a += angleStep {
		cx := float32(right) - corner + float32(math.Cos(a))*corner
		cy := float32(top) + corner + float32(math.Sin(a))*corner
		nx := float32(math.Cos(a))
		ny := float32(math.Sin(a))
		offset := float32(math.Sin(phase+a)) * r * 0.3
		draw(cx+offset*nx, cy+offset*ny)
	}
	// right edge
	for y := float32(top) + corner; y <= float32(bottom)-corner; y += step {
		offset := float32(math.Sin(phase+float64(y-float32(top))*0.1)) * r * 0.3
		draw(float32(right)+offset, y)
	}
	// bottom-right corner
	for a := 0.0; a < math.Pi/2; a += angleStep {
		cx := float32(right) - corner + float32(math.Cos(a))*corner
		cy := float32(bottom) - corner + float32(math.Sin(a))*corner
		nx := float32(math.Cos(a))
		ny := float32(math.Sin(a))
		offset := float32(math.Sin(phase+a)) * r * 0.3
		draw(cx+offset*nx, cy+offset*ny)
	}
	// bottom edge
	for x := float32(right) - corner; x >= float32(left)+corner; x -= step {
		offset := float32(math.Sin(phase+float64(x-float32(left))*0.1)) * r * 0.3
		draw(x, float32(bottom)+offset)
	}
	// bottom-left corner
	for a := math.Pi / 2; a < math.Pi; a += angleStep {
		cx := float32(left) + corner + float32(math.Cos(a))*corner
		cy := float32(bottom) - corner + float32(math.Sin(a))*corner
		nx := float32(math.Cos(a))
		ny := float32(math.Sin(a))
		offset := float32(math.Sin(phase+a)) * r * 0.3
		draw(cx+offset*nx, cy+offset*ny)
	}
	// left edge
	for y := float32(bottom) - corner; y >= float32(top)+corner; y -= step {
		offset := float32(math.Sin(phase+float64(y-float32(top))*0.1)) * r * 0.3
		draw(float32(left)+offset, y)
	}
	// top-left corner
	for a := math.Pi; a < 3*math.Pi/2; a += angleStep {
		cx := float32(left) + corner + float32(math.Cos(a))*corner
		cy := float32(top) + corner + float32(math.Sin(a))*corner
		nx := float32(math.Cos(a))
		ny := float32(math.Sin(a))
		offset := float32(math.Sin(phase+a)) * r * 0.3
		draw(cx+offset*nx, cy+offset*ny)
	}
}

// drawBubbleCircle draws a filled circle used by the wavy ponder bubble edges.
func drawBubbleCircle(screen *ebiten.Image, cx, cy, radius float32, col color.RGBA64) {
	if col.A == 0 {
		return
	}
	var p vector.Path
	p.MoveTo(cx+radius, cy)
	p.Arc(cx, cy, radius, 0, 2*math.Pi, vector.Clockwise)
	p.Close()
	drawOp := &vector.DrawPathOptions{
		AntiAlias: true,
	}
	drawOp.ColorScale.ScaleWithColor(col)
	vector.FillPath(screen, &p, nil, drawOp)
}
