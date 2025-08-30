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

// drawBubble renders a text bubble anchored so that (x, y) corresponds to the
// bottom-center point of the balloon tail. If the bubble would extend past the
// screen edges it is clamped while leaving the tail anchored at (x, y). If far
// is true the tail is omitted and (x, y) represents the bottom-center of the
// bubble itself. The tail can also be skipped explicitly via noArrow. The typ
// parameter is currently unused but retained for future compatibility with the
// original bubble images. The colors of the border, background, and text can be
// customized via borderCol, bgCol, and textCol respectively.
func drawBubble(screen *ebiten.Image, txt string, x, y int, typ int, far bool, noArrow bool, borderCol, bgCol, textCol color.Color) {
	if txt == "" {
		return
	}
	tailX, tailY := x, y

	sw := int(float64(gameAreaSizeX) * gs.GameScale)
	sh := int(float64(gameAreaSizeY) * gs.GameScale)
	if tailX < 0 || tailX >= sw || tailY < 0 || tailY >= sh {
		noArrow = true
	}
	pad := int((4 + 2) * gs.GameScale)
	tailHeight := int(10 * gs.GameScale)
	tailHalf := int(6 * gs.GameScale)
	bubbleType := typ & kBubbleTypeMask

	maxLineWidth := sw/4 - 2*pad
	font := bubbleFont
	if bubbleType == kBubbleWhisper {
		font = mainFont
	}
	width, lines := wrapText(txt, font, float64(maxLineWidth))
	metrics := font.Metrics()
	lineHeight := int(math.Ceil(metrics.HAscent) + math.Ceil(metrics.HDescent) + math.Ceil(metrics.HLineGap))
	width += 2 * pad
	height := lineHeight*len(lines) + 2*pad

	left, top, right, bottom := adjustBubbleRect(x, y, width, height, tailHeight, sw, sh, far || noArrow)
	baseX := left + width/2

	bgR, bgG, bgB, bgA := bgCol.RGBA()
	bdR, bdG, bdB, bdA := borderCol.RGBA()

	radius := float32(4 * gs.GameScale)
	if bubbleType == kBubblePonder {
		radius = float32(8 * gs.GameScale)
	}

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

	var tail vector.Path
	if !far && !noArrow {
        if bubbleType == kBubblePonder {
            r1 := float32(tailHalf)
            phase := float64(time.Now().UnixNano()) / float64(time.Second)
            offset1 := r1 * 0.3 * float32(math.Sin(phase))
            cx1 := float32(baseX)
            // Bias ponder tail circles closer to the mobile so the origin is
            // easier to see. Space the first (largest) circle at ~20% of the
            // way from the bubble bottom to the tail tip instead of directly
            // hugging the bubble.
            dist := float32(tailY - bottom)
            if dist < 0 { dist = 0 }
            cy1 := float32(bottom) + r1 + dist*0.2 - offset1
            tail.MoveTo(cx1+r1, cy1)
            tail.Arc(cx1, cy1, r1, 0, 2*math.Pi, vector.Clockwise)
            tail.Close()
            rMid := r1 * 0.6
            offsetMid := rMid * 0.5 * float32(math.Sin(phase+math.Pi/4))
            cxMid := float32(baseX+tailX) / 2
            // Place the middle circle at ~65% down the path toward the tail.
            cyMid := float32(bottom) + dist*0.65 - offsetMid
            tail.MoveTo(cxMid+rMid, cyMid)
            tail.Arc(cxMid, cyMid, rMid, 0, 2*math.Pi, vector.Clockwise)
            tail.Close()
            r2 := float32(tailHalf) / 2
            offset2 := r2 * 0.6 * float32(math.Sin(phase+math.Pi/2))
			cx2 := float32(tailX)
			cy2 := float32(tailY) - offset2
			tail.MoveTo(cx2+r2, cy2)
			tail.Arc(cx2, cy2, r2, 0, 2*math.Pi, vector.Clockwise)
			tail.Close()
		} else {
			tail.MoveTo(float32(baseX-tailHalf), float32(bottom))
			tail.LineTo(float32(tailX), float32(tailY))
			tail.LineTo(float32(baseX+tailHalf), float32(bottom))
			tail.Close()
		}
	}

	var vs []ebiten.Vertex
	var is []uint16
	if bubbleType != kBubblePonder {
		vs, is = body.AppendVerticesAndIndicesForFilling(nil, nil)
		op := &ebiten.DrawTrianglesOptions{ColorScaleMode: ebiten.ColorScaleModePremultipliedAlpha, AntiAlias: true}
		for i := range vs {
			vs[i].SrcX = 0
			vs[i].SrcY = 0
			vs[i].ColorR = float32(bgR) / 0xffff
			vs[i].ColorG = float32(bgG) / 0xffff
			vs[i].ColorB = float32(bgB) / 0xffff
			vs[i].ColorA = float32(bgA) / 0xffff
		}
		screen.DrawTriangles(vs, is, whiteImage, op)
	}
	if !far && !noArrow {
		vs, is = tail.AppendVerticesAndIndicesForFilling(vs[:0], is[:0])
		tailOp := &ebiten.DrawTrianglesOptions{
			ColorScaleMode: ebiten.ColorScaleModePremultipliedAlpha,
			AntiAlias:      true,
			Blend:          ebiten.BlendCopy,
		}
		for i := range vs {
			vs[i].SrcX = 0
			vs[i].SrcY = 0
			vs[i].ColorR = float32(bgR) / 0xffff
			vs[i].ColorG = float32(bgG) / 0xffff
			vs[i].ColorB = float32(bgB) / 0xffff
			vs[i].ColorA = float32(bgA) / 0xffff
		}
		screen.DrawTriangles(vs, is, whiteImage, tailOp)
	}
	if bubbleType != kBubblePonder {
		var outline vector.Path
		outline.MoveTo(float32(left)+radius, float32(top))
		outline.LineTo(float32(right)-radius, float32(top))
		outline.Arc(float32(right)-radius, float32(top)+radius, radius, -math.Pi/2, 0, vector.Clockwise)
		outline.LineTo(float32(right), float32(bottom)-radius)
		outline.Arc(float32(right)-radius, float32(bottom)-radius, radius, 0, math.Pi/2, vector.Clockwise)
		if !far && !noArrow {
			outline.LineTo(float32(baseX+tailHalf), float32(bottom))
			outline.LineTo(float32(tailX), float32(tailY))
			outline.LineTo(float32(baseX-tailHalf), float32(bottom))
		}
		outline.LineTo(float32(left)+radius, float32(bottom))
		outline.Arc(float32(left)+radius, float32(bottom)-radius, radius, math.Pi/2, math.Pi, vector.Clockwise)
		outline.LineTo(float32(left), float32(top)+radius)
		outline.Arc(float32(left)+radius, float32(top)+radius, radius, math.Pi, 3*math.Pi/2, vector.Clockwise)
		outline.Close()

		vs, is = outline.AppendVerticesAndIndicesForStroke(nil, nil, &vector.StrokeOptions{Width: float32(gs.GameScale)})
		op := &ebiten.DrawTrianglesOptions{ColorScaleMode: ebiten.ColorScaleModePremultipliedAlpha, AntiAlias: true}
		for i := range vs {
			vs[i].SrcX = 0
			vs[i].SrcY = 0
			vs[i].ColorR = float32(bdR) / 0xffff
			vs[i].ColorG = float32(bdG) / 0xffff
			vs[i].ColorB = float32(bdB) / 0xffff
			vs[i].ColorA = float32(bdA) / 0xffff
		}
		screen.DrawTriangles(vs, is, whiteImage, op)
	} else {
		drawPonderWaves(screen, left, top, right, bottom, bgCol)
	}

	if bubbleType == kBubbleYell {
		gapStart, gapEnd := float32(0), float32(0)
		if !far && !noArrow {
			gapStart = float32(baseX - tailHalf)
			gapEnd = float32(baseX + tailHalf)
		} else {
			gapStart, gapEnd = -1, -1
		}
		drawSpikes(screen, float32(left), float32(top), float32(right), float32(bottom), radius, float32(gs.GameScale*3), borderCol, gapStart, gapEnd)
	} else if bubbleType == kBubbleMonster {
		gapStart, gapEnd := float32(0), float32(0)
		if !far && !noArrow {
			gapStart = float32(baseX - tailHalf)
			gapEnd = float32(baseX + tailHalf)
		} else {
			gapStart, gapEnd = -1, -1
		}
		drawMonsterSpikes(screen, float32(left), float32(top), float32(right), float32(bottom), radius, float32(gs.GameScale*4), borderCol, gapStart, gapEnd)
	}

	textTop := top + pad
	textLeft := left + pad
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
	spike := size + size*0.3*float32(math.Sin(phase))
	op := &ebiten.DrawTrianglesOptions{ColorScaleMode: ebiten.ColorScaleModePremultipliedAlpha, AntiAlias: true}

	startX := left + radius
	endX := right - radius
	// top edge
	for x := startX; x < endX; x += step {
		end := x + step
		mid := x + step/2
		if end > endX {
			end = endX
			mid = x + (end-x)/2
		}

		var p vector.Path
		p.MoveTo(x, top)
		p.LineTo(mid, top-spike)
		p.LineTo(end, top)
		p.Close()
		vs, is := p.AppendVerticesAndIndicesForFilling(nil, nil)
		for i := range vs {
			vs[i].SrcX = 0
			vs[i].SrcY = 0
			vs[i].ColorR = float32(bdR) / 0xffff
			vs[i].ColorG = float32(bdG) / 0xffff
			vs[i].ColorB = float32(bdB) / 0xffff
			vs[i].ColorA = float32(bdA) / 0xffff
		}
		screen.DrawTriangles(vs, is, whiteImage, op)
	}

	// bottom edge (split around gap)
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
			end := x + step
			mid := x + step/2
			if end > segEnd {
				end = segEnd
				mid = x + (end-x)/2
			}

			var p vector.Path
			p.MoveTo(x, bottom)
			p.LineTo(mid, bottom+spike)
			p.LineTo(end, bottom)
			p.Close()
			vs, is := p.AppendVerticesAndIndicesForFilling(nil, nil)
			for i := range vs {
				vs[i].SrcX = 0
				vs[i].SrcY = 0
				vs[i].ColorR = float32(bdR) / 0xffff
				vs[i].ColorG = float32(bdG) / 0xffff
				vs[i].ColorB = float32(bdB) / 0xffff
				vs[i].ColorA = float32(bdA) / 0xffff
			}
			screen.DrawTriangles(vs, is, whiteImage, op)
		}
	}
	drawBottom(startX, bottomGapStart)
	drawBottom(bottomGapEnd, endX)

	startY := top + radius
	endY := bottom - radius
	// left and right edges
	for y := startY; y < endY; y += step {
		end := y + step
		mid := y + step/2
		if end > endY {
			end = endY
			mid = y + (end-y)/2
		}

		var p vector.Path
		p.MoveTo(left, y)
		p.LineTo(left-spike, mid)
		p.LineTo(left, end)
		p.Close()
		vs, is := p.AppendVerticesAndIndicesForFilling(nil, nil)
		for i := range vs {
			vs[i].SrcX = 0
			vs[i].SrcY = 0
			vs[i].ColorR = float32(bdR) / 0xffff
			vs[i].ColorG = float32(bdG) / 0xffff
			vs[i].ColorB = float32(bdB) / 0xffff
			vs[i].ColorA = float32(bdA) / 0xffff
		}
		screen.DrawTriangles(vs, is, whiteImage, op)

		p = vector.Path{}
		p.MoveTo(right, y)
		p.LineTo(right+spike, mid)
		p.LineTo(right, end)
		p.Close()
		vs, is = p.AppendVerticesAndIndicesForFilling(nil, nil)
		for i := range vs {
			vs[i].SrcX = 0
			vs[i].SrcY = 0
			vs[i].ColorR = float32(bdR) / 0xffff
			vs[i].ColorG = float32(bdG) / 0xffff
			vs[i].ColorB = float32(bdB) / 0xffff
			vs[i].ColorA = float32(bdA) / 0xffff
		}
		screen.DrawTriangles(vs, is, whiteImage, op)
	}

	if radius > 0 {
		corner := func(cx, cy float32, start, end float64) {
			stepAngle := float64(step) / float64(radius)
			for a := start; a < end; a += stepAngle {
				next := a + stepAngle
				if next > end {
					next = end
				}
				mid := a + (next-a)/2
				x1 := cx + radius*float32(math.Cos(a))
				y1 := cy + radius*float32(math.Sin(a))
				x2 := cx + radius*float32(math.Cos(next))
				y2 := cy + radius*float32(math.Sin(next))
				mx := cx + (radius+spike)*float32(math.Cos(mid))
				my := cy + (radius+spike)*float32(math.Sin(mid))

				var p vector.Path
				p.MoveTo(x1, y1)
				p.LineTo(mx, my)
				p.LineTo(x2, y2)
				p.Close()
				vs, is := p.AppendVerticesAndIndicesForFilling(nil, nil)
				for i := range vs {
					vs[i].SrcX = 0
					vs[i].SrcY = 0
					vs[i].ColorR = float32(bdR) / 0xffff
					vs[i].ColorG = float32(bdG) / 0xffff
					vs[i].ColorB = float32(bdB) / 0xffff
					vs[i].ColorA = float32(bdA) / 0xffff
				}
				screen.DrawTriangles(vs, is, whiteImage, op)
			}
		}

		corner(left+radius, top+radius, math.Pi, 1.5*math.Pi)
		corner(right-radius, top+radius, 1.5*math.Pi, 2*math.Pi)
		corner(right-radius, bottom-radius, 0, 0.5*math.Pi)
		corner(left+radius, bottom-radius, 0.5*math.Pi, math.Pi)
	}
}

// drawMonsterSpikes renders uneven spikes around a bubble for monster speech.
// Each spike varies in length to create a more chaotic cartoon effect.
func drawMonsterSpikes(screen *ebiten.Image, left, top, right, bottom, radius, size float32, col color.Color, bottomGapStart, bottomGapEnd float32) {
	bdR, bdG, bdB, bdA := col.RGBA()
	step := size / 2
	phase := float64(time.Now().UnixNano()) / float64(time.Second)
	op := &ebiten.DrawTrianglesOptions{ColorScaleMode: ebiten.ColorScaleModePremultipliedAlpha, AntiAlias: true}

	startX := left + radius
	endX := right - radius
	// top edge
	for x := startX; x < endX; x += step {
		spike := size * (0.7 + 0.3*float32(math.Sin(phase+float64(x-startX))))
		end := x + step
		mid := x + step/2
		if end > endX {
			end = endX
			mid = x + (end-x)/2
		}

		var p vector.Path
		p.MoveTo(x, top)
		p.LineTo(mid, top-spike)
		p.LineTo(end, top)
		p.Close()
		vs, is := p.AppendVerticesAndIndicesForFilling(nil, nil)
		for i := range vs {
			vs[i].SrcX = 0
			vs[i].SrcY = 0
			vs[i].ColorR = float32(bdR) / 0xffff
			vs[i].ColorG = float32(bdG) / 0xffff
			vs[i].ColorB = float32(bdB) / 0xffff
			vs[i].ColorA = float32(bdA) / 0xffff
		}
		screen.DrawTriangles(vs, is, whiteImage, op)
	}

	// bottom edge (split around gap)
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

			var p vector.Path
			p.MoveTo(x, bottom)
			p.LineTo(mid, bottom+spike)
			p.LineTo(end, bottom)
			p.Close()
			vs, is := p.AppendVerticesAndIndicesForFilling(nil, nil)
			for i := range vs {
				vs[i].SrcX = 0
				vs[i].SrcY = 0
				vs[i].ColorR = float32(bdR) / 0xffff
				vs[i].ColorG = float32(bdG) / 0xffff
				vs[i].ColorB = float32(bdB) / 0xffff
				vs[i].ColorA = float32(bdA) / 0xffff
			}
			screen.DrawTriangles(vs, is, whiteImage, op)
		}
	}
	drawBottom(startX, bottomGapStart)
	drawBottom(bottomGapEnd, endX)

	startY := top + radius
	endY := bottom - radius
	// left and right edges
	for y := startY; y < endY; y += step {
		spike := size * (0.7 + 0.3*float32(math.Sin(phase+float64(y-startY))))
		end := y + step
		mid := y + step/2
		if end > endY {
			end = endY
			mid = y + (end-y)/2
		}

		var p vector.Path
		p.MoveTo(left, y)
		p.LineTo(left-spike, mid)
		p.LineTo(left, end)
		p.Close()
		vs, is := p.AppendVerticesAndIndicesForFilling(nil, nil)
		for i := range vs {
			vs[i].SrcX = 0
			vs[i].SrcY = 0
			vs[i].ColorR = float32(bdR) / 0xffff
			vs[i].ColorG = float32(bdG) / 0xffff
			vs[i].ColorB = float32(bdB) / 0xffff
			vs[i].ColorA = float32(bdA) / 0xffff
		}
		screen.DrawTriangles(vs, is, whiteImage, op)

		p = vector.Path{}
		p.MoveTo(right, y)
		p.LineTo(right+spike, mid)
		p.LineTo(right, end)
		p.Close()
		vs, is = p.AppendVerticesAndIndicesForFilling(nil, nil)
		for i := range vs {
			vs[i].SrcX = 0
			vs[i].SrcY = 0
			vs[i].ColorR = float32(bdR) / 0xffff
			vs[i].ColorG = float32(bdG) / 0xffff
			vs[i].ColorB = float32(bdB) / 0xffff
			vs[i].ColorA = float32(bdA) / 0xffff
		}
		screen.DrawTriangles(vs, is, whiteImage, op)
	}

	if radius > 0 {
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

				var p vector.Path
				p.MoveTo(x1, y1)
				p.LineTo(mx, my)
				p.LineTo(x2, y2)
				p.Close()
				vs, is := p.AppendVerticesAndIndicesForFilling(nil, nil)
				for i := range vs {
					vs[i].SrcX = 0
					vs[i].SrcY = 0
					vs[i].ColorR = float32(bdR) / 0xffff
					vs[i].ColorG = float32(bdG) / 0xffff
					vs[i].ColorB = float32(bdB) / 0xffff
					vs[i].ColorA = float32(bdA) / 0xffff
				}
				screen.DrawTriangles(vs, is, whiteImage, op)
			}
		}

		corner(left+radius, top+radius, math.Pi, 1.5*math.Pi)
		corner(right-radius, top+radius, 1.5*math.Pi, 2*math.Pi)
		corner(right-radius, bottom-radius, 0, 0.5*math.Pi)
		corner(left+radius, bottom-radius, 0.5*math.Pi, math.Pi)
	}
}

// drawPonderWaves renders the ponder bubble's body and a subtle animated wavy
// border made of small circles. Drawing both here ensures consistent
// compositing and color/alpha handling.
func drawPonderWaves(screen *ebiten.Image, left, top, right, bottom int, col color.Color) {
	cr, cg, cb, ca := col.RGBA()
	radius := float32(8 * gs.GameScale)
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

	vs, is := body.AppendVerticesAndIndicesForFilling(nil, nil)
	for i := range vs {
		vs[i].SrcX = 0
		vs[i].SrcY = 0
		vs[i].ColorR = float32(cr) / 0xffff
		vs[i].ColorG = float32(cg) / 0xffff
		vs[i].ColorB = float32(cb) / 0xffff
		vs[i].ColorA = float32(ca) / 0xffff
	}
	op := &ebiten.DrawTrianglesOptions{
		ColorScaleMode: ebiten.ColorScaleModePremultipliedAlpha,
		AntiAlias:      true,
		Blend:          ebiten.BlendCopy,
	}
	screen.DrawTriangles(vs, is, whiteImage, op)

	r := float32(6 * gs.GameScale)
	step := r * 1.2
	phase := float64(time.Now().UnixNano()) / float64(time.Second)
	corner := float32(10 * gs.GameScale)
	angleStep := float64(step / corner)

	draw := func(cx, cy float32) {
		drawBubbleCircle(screen, cx, cy, r, col)
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
func drawBubbleCircle(screen *ebiten.Image, cx, cy, radius float32, col color.Color) {
	r, g, b, a := col.RGBA()
	var p vector.Path
	p.MoveTo(cx+radius, cy)
	p.Arc(cx, cy, radius, 0, 2*math.Pi, vector.Clockwise)
	p.Close()
	vs, is := p.AppendVerticesAndIndicesForFilling(nil, nil)
	for i := range vs {
		vs[i].SrcX = 0
		vs[i].SrcY = 0
		vs[i].ColorR = float32(r) / 0xffff
		vs[i].ColorG = float32(g) / 0xffff
		vs[i].ColorB = float32(b) / 0xffff
		vs[i].ColorA = float32(a) / 0xffff
	}
	op := &ebiten.DrawTrianglesOptions{
		ColorScaleMode: ebiten.ColorScaleModePremultipliedAlpha,
		AntiAlias:      true,
		Blend:          ebiten.BlendCopy,
	}
	screen.DrawTriangles(vs, is, whiteImage, op)
}
