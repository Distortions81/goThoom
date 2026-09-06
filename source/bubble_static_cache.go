package main

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// Freeze geometry only when animation is disabled. Position and text are not
// part of the key: moving bubbles and different captions can share the surface.
// Entries use the existing bounded body cache and pooled render targets.
func cachedStaticBubbleDecoration(g bubbleDrawGeometry) (*ebiten.Image, int) {
	gapX, gapHalf, gapTop := bubbleOutlineGap(g)
	key := bubbleBodyImageCacheKey{
		gapX: gapX, gapHalf: gapHalf, gapTop: gapTop,
		decorationType: g.bubbleType, scaleBits: math.Float32bits(g.scale),
		width: g.right - g.left, height: g.bottom - g.top,
		radiusBits: math.Float32bits(g.radius), strokeWidthBits: math.Float32bits(max(1, g.scale)),
		fillR: g.fillColor.R, fillG: g.fillColor.G, fillB: g.fillColor.B, fillA: g.fillColor.A,
		borderR: g.borderColor.R, borderG: g.borderColor.G, borderB: g.borderColor.B, borderA: g.borderColor.A,
	}
	margin := bubbleOverlapMargin(g.bubbleType, float64(g.scale)) + int(math.Ceil(float64(max(1, g.scale))/2)) + 2
	return cacheBubbleSurface(key, margin, func(img *ebiten.Image) {
		local := g
		local.left, local.top = margin, margin
		local.right, local.bottom = margin+key.width, margin+key.height
		local.offsetX, local.offsetY = 0, 0
		local.baseX = margin + gapX
		local.attachY = local.bottom
		if gapTop {
			local.attachY = local.top
		}
		// A small pooled mask keeps cold cache misses from resizing the
		// full-world mask used by moving thought/ponder tails.
		var mask *ebiten.Image
		if g.bubbleType == kBubbleThought || g.bubbleType == kBubblePonder {
			mask = bubbleBodyTargets.Acquire(img.Bounds().Dx(), img.Bounds().Dy(), gs.PotatoGPU)
			defer bubbleBodyTargets.Release(mask)
		}
		drawBubbleDecoration(img, local, mask)
	})
}
