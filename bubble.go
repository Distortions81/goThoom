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
// bottom-center of the balloon tail. The typ parameter is the bubble type value
// from the server draw state.
func drawBubble(screen *ebiten.Image, txt string, x, y int, typ int) {
	if txt == "" {
		return
	}

	// Determine bubble size by wrapping the text with a small width.
	lines := wrapText(txt, nameFace, bubbleTextSmallWidth*float64(scale))
	bw, bh := bubbleSmallWidth, bubbleSmallHeight
	tw := bubbleTextSmallWidth
	if len(lines) > 2 {
		lines = wrapText(txt, nameFace, bubbleTextMediumWidth*float64(scale))
		bw, bh = bubbleMediumWidth, bubbleMediumHeight
		tw = bubbleTextMediumWidth
		if len(lines) > 3 {
			lines = wrapText(txt, nameFace, bubbleTextLargeWidth*float64(scale))
			bw, bh = bubbleLargeWidth, bubbleLargeHeight
			tw = bubbleTextLargeWidth
		}
	}

	// Original bubble image rendering is temporarily disabled.
	// col := 0
	// if t := typ & kBubbleTypeMask; t >= 0 && t < len(gBubbleMap) {
	//      col = gBubbleMap[t]
	// }
	// img := loadBubbleImage(id, col)
	// if img != nil {
	//      op := &ebiten.DrawImageOptions{}
	//      op.Filter = drawFilter
	//      op.GeoM.Scale(float64(scale), float64(scale))
	//      op.GeoM.Translate(float64(x-bw*scale/2), float64(y-bh*scale))
	//      screen.DrawImage(img, op)
	// }

	// Draw a semi-transparent white box instead of the bubble image.
	w, h := bw*scale, bh*scale
	box := ebiten.NewImage(w, h)
	box.Fill(color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xb3}) // 30% transparent
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x-w/2), float64(y-h))
	screen.DrawImage(box, op)

	metrics := nameFace.Metrics()
	lineHeight := int(math.Ceil(metrics.HAscent + metrics.HDescent + metrics.HLineGap))
	baseline := y - bh*scale + 4*scale + int(math.Ceil(metrics.HAscent))
	left := x - tw*scale/2

	for i, line := range lines {
		op := &text.DrawOptions{}
		op.GeoM.Translate(float64(left), float64(baseline+i*lineHeight))
		op.ColorScale.ScaleWithColor(color.Black)
		text.Draw(screen, line, nameFace, op)
	}
}
