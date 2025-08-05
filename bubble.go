package main

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

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
// bottom-center of the balloon tail. The typ parameter is currently unused but
// retained for future compatibility with the original bubble images.
func drawBubble(screen *ebiten.Image, txt string, x, y int, typ int) {
	if txt == "" {
		return
	}

	sw, sh := gameAreaSizeX, gameAreaSizeY
	pad := 4 * scale

	maxLineWidth := sw / 8
	lines := wrapText(txt, nameFace, float64(maxLineWidth))
	metrics := nameFace.Metrics()
	lineHeight := int(math.Ceil(metrics.HAscent) + math.Ceil(metrics.HDescent) + math.Ceil(metrics.HLineGap))
	width := maxLineWidth + 2*pad
	height := lineHeight*len(lines) + 2*pad

	bottom := y - 10*scale
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

	box := ebiten.NewImage(width, height)
	box.Fill(color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xb3})
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(left), float64(top))
	screen.DrawImage(box, op)

	baseline := top + pad + int(math.Ceil(metrics.HAscent))
	textLeft := left + pad
	for i, line := range lines {
		op := &text.DrawOptions{}
		op.GeoM.Translate(float64(textLeft), float64(baseline+i*lineHeight))
		op.ColorScale.ScaleWithColor(color.Black)
		text.Draw(screen, line, nameFace, op)
	}
}
