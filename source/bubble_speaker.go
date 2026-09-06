package main

import (
	"image"
	"math"
)

// Mobile artwork is centered on its world coordinate. Keep placement above
// the sprite while aiming the pointer at an approximate face location; source
// artwork has no mouth metadata, so facing and frame size guide the estimate.
func bubbleSpeakerAnchors(center image.Point, size, facing, tailHeight, clearance int) (upper, lower, mouth image.Point, sprite image.Rectangle) {
	half := size / 2
	sprite = image.Rect(center.X-half, center.Y-half, center.X-half+size, center.Y-half+size)
	upper = image.Pt(center.X, sprite.Min.Y-clearance+tailHeight)
	lower = image.Pt(center.X, sprite.Max.Y+clearance-tailHeight)
	angle := float64(facing%8) * math.Pi / 4
	mouth = image.Pt(center.X+roundToInt(math.Cos(angle)*float64(size)*.12), center.Y-roundToInt(float64(size)*.23)+roundToInt(math.Sin(angle)*float64(size)*.04))
	return
}

// Moving/crowded bubbles must not slide their bodies over their own speaker.
// Choose the nearest clear side that still fits the viewport.
func clearBubbleSpeaker(body, sprite image.Rectangle, margin, sw, sh int) (image.Rectangle, bool) {
	if sprite.Empty() || bubbleOverlapRect(body, margin).Intersect(sprite).Empty() {
		return body, true
	}
	candidates := [4]image.Point{
		{Y: sprite.Min.Y - margin - body.Max.Y},
		{X: sprite.Min.X - margin - body.Max.X},
		{X: sprite.Max.X + margin - body.Min.X},
		{Y: sprite.Max.Y + margin - body.Min.Y},
	}
	best, found := body, false
	distance := math.MaxInt
	for _, offset := range candidates {
		moved := clampBubbleRect(body.Add(offset), sw, sh)
		if !bubbleOverlapRect(moved, margin).Intersect(sprite).Empty() {
			continue
		}
		dx, dy := moved.Min.X-body.Min.X, moved.Min.Y-body.Min.Y
		if d := dx*dx + dy*dy; d < distance {
			best, found, distance = moved, true, d
		}
	}
	return best, found
}

func bubbleHasSpeakerTail(typ int) bool {
	switch typ & kBubbleTypeMask {
	case kBubbleThought, kBubbleRealAction, kBubblePlayerAction, kBubbleNarrate:
		return false
	default:
		return true
	}
}

func bubbleTailHeight(typ int, scale float64) int {
	if !bubbleHasSpeakerTail(typ) {
		return 0
	}
	return max(1, int(math.Round(10*scale)))
}
