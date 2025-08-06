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

// adjustBubbleRect calculates the on-screen rectangle for a bubble and shifts
// the tail tip (x, y) if clamping is required. It returns the clamped
// rectangle along with the adjusted tail coordinates.
func adjustBubbleRect(x, y, width, height, tailHeight, sw, sh int, far bool) (left, top, right, bottom, ax, ay int) {
	bottom = y
	if !far {
		bottom = y - tailHeight
	}
	left = x - width/2
	top = bottom - height

	origLeft, origTop := left, top

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

	dx := left - origLeft
	dy := top - origTop
	ax = x + dx
	ay = y + dy

	right = left + width
	bottom = top + height
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
func bubbleColors(typ int) (border, bg, text color.Color) {
	switch typ & kBubbleTypeMask {
	case kBubbleWhisper:
		border = color.NRGBA{0xcc, 0xcc, 0xcc, 0xff}
		bg = color.NRGBA{0xf0, 0xf0, 0xf0, 0x80}
		text = color.NRGBA{0x33, 0x33, 0x33, 0xff}
	case kBubbleYell:
		border = color.NRGBA{0xff, 0x00, 0x00, 0xff}
		bg = color.NRGBA{0xff, 0xff, 0x00, 0x80}
		text = color.Black
	case kBubbleThought, kBubblePonder:
		border = color.NRGBA{0x00, 0x00, 0x80, 0xff}
		bg = color.NRGBA{0xee, 0xee, 0xff, 0x80}
		text = color.Black
	case kBubbleRealAction, kBubblePlayerAction, kBubbleNarrate:
		border = color.NRGBA{0x00, 0x80, 0x00, 0xff}
		bg = color.NRGBA{0xe0, 0xff, 0xe0, 0x80}
		text = color.Black
	case kBubbleMonster:
		border = color.NRGBA{0x80, 0x00, 0x80, 0xff}
		bg = color.NRGBA{0xff, 0xe0, 0xff, 0x80}
		text = color.Black
	default:
		border = color.White
		bg = color.NRGBA{0xff, 0xff, 0xff, 0x80}
		text = color.Black
	}
	return
}

// drawBubble renders a text bubble anchored so that (x, y) corresponds to the
// bottom-center of the balloon tail. If far is true the tail is omitted and
// (x, y) represents the bottom-center of the bubble itself. The typ parameter
// is currently unused but retained for future compatibility with the original
// bubble images. The colors of the border, background, and text can be
// customized via borderCol, bgCol, and textCol respectively.
func drawBubble(screen *ebiten.Image, txt string, x, y int, typ int, far bool, borderCol, bgCol, textCol color.Color) {
	if txt == "" {
		return
	}

	sw, sh := gameAreaSizeX, gameAreaSizeY
	pad := (4 + 2) * scale
	tailHeight := 10 * scale
	tailHalf := 6 * scale

	maxLineWidth := sw/4*scale - 2*pad
	width, lines := wrapText(txt, bubbleFont, float64(maxLineWidth))
	metrics := bubbleFont.Metrics()
	lineHeight := int(math.Ceil(metrics.HAscent) + math.Ceil(metrics.HDescent) + math.Ceil(metrics.HLineGap))
	width += 2 * pad
	height := lineHeight*len(lines) + 2*pad

	left, top, right, bottom, x, y := adjustBubbleRect(x, y, width, height, tailHeight, sw, sh, far)

	bgR, bgG, bgB, bgA := bgCol.RGBA()

	radius := float32(4 * scale)

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
	if !far {
		tail.MoveTo(float32(x-tailHalf), float32(bottom))
		tail.LineTo(float32(x), float32(y))
		tail.LineTo(float32(x+tailHalf), float32(bottom))
		tail.Close()
	}

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

	if !far {
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
	var outline vector.Path
	outline.MoveTo(float32(left)+radius, float32(top))
	outline.LineTo(float32(right)-radius, float32(top))
	outline.Arc(float32(right)-radius, float32(top)+radius, radius, -math.Pi/2, 0, vector.Clockwise)
	outline.LineTo(float32(right), float32(bottom)-radius)
	outline.Arc(float32(right)-radius, float32(bottom)-radius, radius, 0, math.Pi/2, vector.Clockwise)
	if !far {
		outline.LineTo(float32(x+tailHalf), float32(bottom))
		outline.LineTo(float32(x), float32(y))
		outline.LineTo(float32(x-tailHalf), float32(bottom))
	}
	outline.LineTo(float32(left)+radius, float32(bottom))
	outline.Arc(float32(left)+radius, float32(bottom)-radius, radius, math.Pi/2, math.Pi, vector.Clockwise)
	outline.LineTo(float32(left), float32(top)+radius)
	outline.Arc(float32(left)+radius, float32(top)+radius, radius, math.Pi, 3*math.Pi/2, vector.Clockwise)
	outline.Close()

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
