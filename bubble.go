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

// drawBubble renders a text bubble anchored so that (x, y) corresponds to the
// bottom-center of the balloon tail. If far is true the tail is omitted and
// (x, y) represents the bottom-center of the bubble itself. The typ parameter
// is currently unused but retained for future compatibility with the original
// bubble images.
func drawBubble(screen *ebiten.Image, txt string, x, y int, typ int, far bool) {
	if txt == "" {
		return
	}

	sw, sh := gameAreaSizeX, gameAreaSizeY
	pad := 4 * scale
	tailHeight := 10 * scale
	tailHalf := 6 * scale

	maxLineWidth := sw / 4 * scale
	width, lines := wrapText(txt, bubbleFont, float64(maxLineWidth))
	metrics := bubbleFont.Metrics()
	lineHeight := int(math.Ceil(metrics.HAscent) + math.Ceil(metrics.HDescent) + math.Ceil(metrics.HLineGap))
	height := lineHeight*len(lines) + 2*pad

	bottom := y
	if !far {
		bottom = y - tailHeight
	}
	left := x - width/2
	top := bottom - height

	// Ensure the bubble remains fully on screen.
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

	col := color.NRGBA{R: 255, G: 255, B: 255, A: 128}
	r, g, b, a := col.RGBA()
	if !far {
		vs := []ebiten.Vertex{
			{DstX: float32(x), DstY: float32(y), SrcX: 0, SrcY: 0, ColorR: float32(r) / 0xffff, ColorG: float32(g) / 0xffff, ColorB: float32(b) / 0xffff, ColorA: float32(a) / 0xffff},
			{DstX: float32(x - tailHalf), DstY: float32(bottom), SrcX: 0, SrcY: 0, ColorR: float32(r) / 0xffff, ColorG: float32(g) / 0xffff, ColorB: float32(b) / 0xffff, ColorA: float32(a) / 0xffff},
			{DstX: float32(x + tailHalf), DstY: float32(bottom), SrcX: 0, SrcY: 0, ColorR: float32(r) / 0xffff, ColorG: float32(g) / 0xffff, ColorB: float32(b) / 0xffff, ColorA: float32(a) / 0xffff},
		}
		is := []uint16{0, 1, 2}
		op := &ebiten.DrawTrianglesOptions{ColorScaleMode: ebiten.ColorScaleModePremultipliedAlpha}
		screen.DrawTriangles(vs, is, whiteImage, op)
	}

	radius := float32(4 * scale)
	var rectPath vector.Path
	rectPath.MoveTo(float32(left)+radius, float32(top))
	rectPath.LineTo(float32(left+width)-radius, float32(top))
	rectPath.Arc(float32(left+width)-radius, float32(top)+radius, radius, -math.Pi/2, 0, vector.Clockwise)
	rectPath.LineTo(float32(left+width), float32(top+height)-radius)
	rectPath.Arc(float32(left+width)-radius, float32(top+height)-radius, radius, 0, math.Pi/2, vector.Clockwise)
	rectPath.LineTo(float32(left)+radius, float32(top+height))
	rectPath.Arc(float32(left)+radius, float32(top+height)-radius, radius, math.Pi/2, math.Pi, vector.Clockwise)
	rectPath.LineTo(float32(left), float32(top)+radius)
	rectPath.Arc(float32(left)+radius, float32(top)+radius, radius, math.Pi, 3*math.Pi/2, vector.Clockwise)
	rectPath.Close()
	vs, is := rectPath.AppendVerticesAndIndicesForFilling(nil, nil)
	for i := range vs {
		vs[i].SrcX = 0
		vs[i].SrcY = 0
		vs[i].ColorR = float32(r) / 0xffff
		vs[i].ColorG = float32(g) / 0xffff
		vs[i].ColorB = float32(b) / 0xffff
		vs[i].ColorA = float32(a) / 0xffff
	}
	op := &ebiten.DrawTrianglesOptions{ColorScaleMode: ebiten.ColorScaleModePremultipliedAlpha}
	screen.DrawTriangles(vs, is, whiteImage, op)

	baseline := top + pad + int(math.Ceil(metrics.HAscent))
	textLeft := left + pad
	for i, line := range lines {
		op := &text.DrawOptions{}
		op.GeoM.Translate(float64(textLeft), float64(baseline+i*lineHeight))
		op.ColorScale.ScaleWithColor(color.Black)
		text.Draw(screen, line, bubbleFont, op)
	}
}
