package main

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

var whiteImage = ebiten.NewImage(1, 1)

func init() {
	whiteImage.Fill(color.White)
}

// adjustBubbleRect calculates the on-screen rectangle for a bubble and clamps
// it to the screen dimensions. The tail tip coordinates are left untouched so
// the caller can draw an arrow pointing at the original target.
func adjustBubbleRect(x, y, width, height, tailHeight, sw, sh int, far bool) (left, top, right, bottom int) {
	bottom = y
	if !far {
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

// bubbleArrowBase returns the point on the rectangle [left, top, right, bottom]
// where a line from (tx, ty) through the rectangle's centre intersects the
// rectangle. The returned side indicates which edge was hit: 0=top, 1=right,
// 2=bottom, 3=left.
func bubbleArrowBase(tx, ty, left, top, right, bottom int, radius float64) (bx, by int, side int) {
	cx := float64(left+right) / 2
	cy := float64(top+bottom) / 2
	dx := float64(tx) - cx
	dy := float64(ty) - cy

	tMin := math.Inf(1)
	var ix, iy float64

	if dx != 0 {
		k := (float64(left) - cx) / dx
		y := cy + k*dy
		if k > 0 && y >= float64(top) && y <= float64(bottom) && k < tMin {
			tMin = k
			ix = float64(left)
			iy = y
			side = 3 // left
		}
		k = (float64(right) - cx) / dx
		y = cy + k*dy
		if k > 0 && y >= float64(top) && y <= float64(bottom) && k < tMin {
			tMin = k
			ix = float64(right)
			iy = y
			side = 1 // right
		}
	}
	if dy != 0 {
		k := (float64(top) - cy) / dy
		x := cx + k*dx
		if k > 0 && x >= float64(left) && x <= float64(right) && k < tMin {
			tMin = k
			ix = x
			iy = float64(top)
			side = 0 // top
		}
		k = (float64(bottom) - cy) / dy
		x = cx + k*dx
		if k > 0 && x >= float64(left) && x <= float64(right) && k < tMin {
			tMin = k
			ix = x
			iy = float64(bottom)
			side = 2 // bottom
		}
	}

	switch side {
	case 0, 2:
		minX := float64(left) + radius
		maxX := float64(right) - radius
		if ix < minX {
			ix = minX
		}
		if ix > maxX {
			ix = maxX
		}
	case 1, 3:
		minY := float64(top) + radius
		maxY := float64(bottom) - radius
		if iy < minY {
			iy = minY
		}
		if iy > maxY {
			iy = maxY
		}
	}

	bx = int(math.Round(ix))
	by = int(math.Round(iy))
	return
}

// Bubble dimensions and text widths derived from the original Macintosh client.
// Sizes are in pixels at scale 1.
const (
	bubbleTextSmallWidth  = 56
	bubbleTextMediumWidth = 90
	bubbleTextLargeWidth  = 136

	bubbleSmallWidth   = 84
	bubbleSmallHeight  = 33
	bubbleMediumWidth  = 116
	bubbleMediumHeight = 43
	bubbleLargeWidth   = 164
	bubbleLargeHeight  = 53
)

// gBubbleMap previously mapped bubble types to columns in the bubble sprite
// sheet. It's retained here for reference while bubble images are disabled.
// var gBubbleMap = []int{
//      0, // kBubbleNormal
//      1, // kBubbleWhisper
//      2, // kBubbleYell
//      3, // kBubbleThought
//      4, // kBubbleRealAction
//      5, // kBubbleMonster
//      4, // kBubblePlayerAction - same graphic as real action
//      3, // kBubblePonder - same graphic as thought
//      4, // kBubbleNarrate - same graphic as real action
// }

// bubbleColors selects the border, background, and text colors for a bubble
// based on its type. Alpha values are premultiplied to match Ebiten's color
// expectations.
const ba = 0xc4

func bubbleColors(typ int) (border, bg, text color.Color) {
	switch typ & kBubbleTypeMask {
	case kBubbleWhisper:
		border = color.NRGBA{0x80, 0x80, 0x80, 0xff}
		bg = color.NRGBA{0x33, 0x33, 0x33, ba}
		text = color.White
	case kBubbleYell:
		border = color.NRGBA{0xff, 0xff, 0x00, 0xff}
		bg = color.White
		text = color.Black
	case kBubbleThought, kBubblePonder:
		border = color.NRGBA{0x00, 0x00, 0x00, 0x00}
		bg = color.NRGBA{0x80, 0x80, 0x80, ba}
		text = color.Black
	case kBubbleRealAction:
		border = color.NRGBA{0x00, 0x00, 0x80, 0xff}
		bg = color.NRGBA{0xff, 0xff, 0xff, ba}
		text = color.Black
	case kBubblePlayerAction:
		border = color.NRGBA{0x80, 0x00, 0x00, 0xff}
		bg = color.NRGBA{0xff, 0xff, 0xff, ba}
		text = color.Black
	case kBubbleNarrate:
		border = color.NRGBA{0x00, 0x80, 0x00, 0xff}
		bg = color.NRGBA{0xff, 0xff, 0xff, ba}
		text = color.Black
	case kBubbleMonster:
		border = color.NRGBA{0xd6, 0xd6, 0xd6, 0xff}
		bg = color.NRGBA{0x47, 0x47, 0x47, ba}
		text = color.White
	default:
		border = color.White
		bg = color.NRGBA{0xff, 0xff, 0xff, ba}
		text = color.Black
	}
	return
}

// drawBubble renders a text bubble anchored so that (x, y) corresponds to the
// bottom-center of the balloon tail. If far is true the tail is omitted and
// (x, y) represents the bottom-center of the bubble itself. The tail can also
// be skipped explicitly via noArrow. The typ parameter
// is currently unused but retained for future compatibility with the original
// bubble images. The colors of the border, background, and text can be
// customized via borderCol, bgCol, and textCol respectively.
func drawBubble(screen *ebiten.Image, txt string, x, y int, typ int, far bool, noArrow bool, borderCol, bgCol, textCol color.Color) {
	if txt == "" {
		return
	}
	y -= 35

	tipX, tipY := x, y

	sw, sh := gameAreaSizeX*scale, gameAreaSizeY*scale
	pad := (4 + 2) * scale
	tailHeight := 10 * scale
	tailHalf := 6 * scale

	maxLineWidth := sw/4 - 2*pad
	width, lines := wrapText(txt, bubbleFont, float64(maxLineWidth))
	metrics := bubbleFont.Metrics()
	lineHeight := int(math.Ceil(metrics.HAscent) + math.Ceil(metrics.HDescent) + math.Ceil(metrics.HLineGap))
	width += 2 * pad
	height := lineHeight*len(lines) + 2*pad

	left, top, right, bottom := adjustBubbleRect(x, y, width, height, tailHeight, sw, sh, far)

	bgR, bgG, bgB, bgA := bgCol.RGBA()

	radius := float32(4 * scale)
	drawTail := !far && !noArrow && !(tipX >= left && tipX <= right && tipY >= top && tipY <= bottom)

	var bx, by int
	var side int
	var nx, ny float64
	if drawTail {
		bx, by, side = bubbleArrowBase(tipX, tipY, left, top, right, bottom, float64(radius))
		dx := float64(tipX - bx)
		dy := float64(tipY - by)
		l := math.Hypot(dx, dy)
		if l != 0 {
			nx = dy / l * float64(tailHalf)
			ny = -dx / l * float64(tailHalf)
		}
	}

	var body vector.Path
	body.MoveTo(float32(left)+radius, float32(top))
	if drawTail && side == 0 {
		body.LineTo(float32(float64(bx)-nx), float32(top))
		body.LineTo(float32(tipX), float32(tipY))
		body.LineTo(float32(float64(bx)+nx), float32(top))
	}
	body.LineTo(float32(right)-radius, float32(top))
	body.Arc(float32(right)-radius, float32(top)+radius, radius, -math.Pi/2, 0, vector.Clockwise)
	if drawTail && side == 1 {
		y1 := float32(float64(by) + ny)
		y2 := float32(float64(by) - ny)
		if y1 > y2 {
			body.LineTo(float32(right), y2)
			body.LineTo(float32(tipX), float32(tipY))
			body.LineTo(float32(right), y1)
		} else {
			body.LineTo(float32(right), y1)
			body.LineTo(float32(tipX), float32(tipY))
			body.LineTo(float32(right), y2)
		}
	}
	body.LineTo(float32(right), float32(bottom)-radius)
	body.Arc(float32(right)-radius, float32(bottom)-radius, radius, 0, math.Pi/2, vector.Clockwise)
	if drawTail && side == 2 {
		body.LineTo(float32(float64(bx)+nx), float32(bottom))
		body.LineTo(float32(tipX), float32(tipY))
		body.LineTo(float32(float64(bx)-nx), float32(bottom))
	}
	body.LineTo(float32(left)+radius, float32(bottom))
	body.Arc(float32(left)+radius, float32(bottom)-radius, radius, math.Pi/2, math.Pi, vector.Clockwise)
	if drawTail && side == 3 {
		y1 := float32(float64(by) - ny)
		y2 := float32(float64(by) + ny)
		if y1 > y2 {
			body.LineTo(float32(left), y1)
			body.LineTo(float32(tipX), float32(tipY))
			body.LineTo(float32(left), y2)
		} else {
			body.LineTo(float32(left), y2)
			body.LineTo(float32(tipX), float32(tipY))
			body.LineTo(float32(left), y1)
		}
	}
	body.LineTo(float32(left), float32(top)+radius)
	body.Arc(float32(left)+radius, float32(top)+radius, radius, math.Pi, 3*math.Pi/2, vector.Clockwise)
	body.Close()

	vs, is := body.AppendVerticesAndIndicesForFilling(nil, nil)
	for i := range vs {
		vs[i].SrcX = 0
		vs[i].SrcY = 0
		vs[i].ColorR = float32(bgR) / 0xffff
		vs[i].ColorG = float32(bgG) / 0xffff
		vs[i].ColorB = float32(bgB) / 0xffff
		vs[i].ColorA = float32(bgA) / 0xffff
	}
	op := &ebiten.DrawTrianglesOptions{ColorScaleMode: ebiten.ColorScaleModePremultipliedAlpha}
	screen.DrawTriangles(vs, is, whiteImage, op)

	if drawTail {
		var tail vector.Path
		tail.MoveTo(float32(float64(bx)-nx), float32(float64(by)-ny))
		tail.LineTo(float32(tipX), float32(tipY))
		tail.LineTo(float32(float64(bx)+nx), float32(float64(by)+ny))
		tail.Close()

		vs, is = tail.AppendVerticesAndIndicesForFilling(vs[:0], is[:0])
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

	bdR, bdG, bdB, bdA := borderCol.RGBA()
	outline := body
	vs, is = outline.AppendVerticesAndIndicesForStroke(nil, nil, &vector.StrokeOptions{Width: float32(scale)})
	for i := range vs {
		vs[i].SrcX = 0
		vs[i].SrcY = 0
		vs[i].ColorR = float32(bdR) / 0xffff
		vs[i].ColorG = float32(bdG) / 0xffff
		vs[i].ColorB = float32(bdB) / 0xffff
		vs[i].ColorA = float32(bdA) / 0xffff
	}
	screen.DrawTriangles(vs, is, whiteImage, op)

	textTop := top + pad
	textLeft := left + pad
	for i, line := range lines {
		op := &text.DrawOptions{}
		op.GeoM.Translate(float64(textLeft), float64(textTop+i*lineHeight))
		op.ColorScale.ScaleWithColor(textCol)
		text.Draw(screen, line, bubbleFont, op)
	}
}
