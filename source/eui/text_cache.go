package eui

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

const maxUITextBytes = 8 << 20
const maxUITextEntries = 256

// Cache plain text independently of its parent window. Chat/status updates can
// repaint a whole pane while most of its lines are unchanged. Client faces
// vary by source (regular/bold/italic) and size; separately constructed
// Chat/Console faces can share entries. Fractional placement and tint are part
// of the key so overlapping/translucent glyphs retain their original coverage.
type uiTextKey struct {
	value          string
	source         *text.GoTextFaceSource
	size           float64
	layout         text.LayoutOptions
	phaseX, phaseY float64
	tint           ebiten.ColorScale
}
type uiTextImage struct {
	image  *ebiten.Image
	bounds image.Rectangle
	used   uint64
}
type uiTextCache struct {
	entries      map[uiTextKey]uiTextImage
	bytes        int
	clock        uint64
	hits, misses uint64
}

var plainUIText uiTextCache

func (cache *uiTextCache) clear() {
	for _, entry := range cache.entries {
		entry.image.Deallocate()
	}
	*cache = uiTextCache{}
}

// Only called with translation-only, nearest-filtered
// DrawOptions. Store actual glyph bounds, including overhangs and diacritics.
func drawCachedUIText(dst *ebiten.Image, value string, face text.Face, op *text.DrawOptions) {
	if value == "" {
		return
	}
	plain, ok := face.(*text.GoTextFace)
	if len(value) > 4096 || !ok {
		text.Draw(dst, value, face, op)
		return
	}
	x, y := op.GeoM.Apply(0, 0)
	ix, iy := math.Floor(x), math.Floor(y)
	key := uiTextKey{value: value, source: plain.Source, size: plain.Size, layout: op.LayoutOptions,
		phaseX: x - ix, phaseY: y - iy, tint: op.ColorScale}
	cache := &plainUIText
	cache.clock++
	entry, cached := cache.entries[key]
	if cached {
		cache.hits++
	} else {
		cache.misses++
		glyphs := text.AppendGlyphs(nil, value, face, &op.LayoutOptions)
		var bounds image.Rectangle
		for _, glyph := range glyphs {
			if glyph.Image == nil {
				continue
			}
			w, h := glyph.Image.Bounds().Dx(), glyph.Image.Bounds().Dy()
			gx, gy := glyph.X+key.phaseX, glyph.Y+key.phaseY
			bounds = bounds.Union(image.Rect(int(math.Floor(gx)), int(math.Floor(gy)), int(math.Ceil(gx))+w, int(math.Ceil(gy))+h))
		}
		if bounds.Empty() {
			return
		}
		// Large documents keep the clipped direct path and cannot exhaust VRAM.
		if bounds.Dx() > 1024 || bounds.Dy() > 256 {
			// Reuse the layout already computed above instead of shaping twice.
			drawOp := op.DrawImageOptions
			for _, glyph := range glyphs {
				if glyph.Image == nil {
					continue
				}
				drawOp.GeoM.Reset()
				drawOp.GeoM.Translate(glyph.X, glyph.Y)
				drawOp.GeoM.Concat(op.GeoM)
				dst.DrawImage(glyph.Image, &drawOp)
			}
			return
		}
		bytes := bounds.Dx() * bounds.Dy() * 4
		for len(cache.entries) >= maxUITextEntries || cache.bytes+bytes > maxUITextBytes {
			var oldestKey uiTextKey
			oldest := cache.clock
			for k, candidate := range cache.entries {
				if candidate.used < oldest {
					oldestKey, oldest = k, candidate.used
				}
			}
			victim := cache.entries[oldestKey]
			victim.image.Deallocate()
			cache.bytes -= victim.bounds.Dx() * victim.bounds.Dy() * 4
			delete(cache.entries, oldestKey)
		}
		entry = uiTextImage{image: newManagedImage(bounds.Dx(), bounds.Dy()), bounds: bounds}
		drawOp := op.DrawImageOptions
		for _, glyph := range glyphs {
			if glyph.Image == nil {
				continue
			}
			drawOp.GeoM.Reset()
			drawOp.GeoM.Translate(glyph.X+key.phaseX-float64(bounds.Min.X), glyph.Y+key.phaseY-float64(bounds.Min.Y))
			entry.image.DrawImage(glyph.Image, &drawOp)
		}
		if cache.entries == nil {
			cache.entries = make(map[uiTextKey]uiTextImage)
		}
		cache.bytes += bytes
	}
	entry.used = cache.clock
	cache.entries[key] = entry
	drawOp := ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
	drawOp.GeoM.Translate(ix+float64(entry.bounds.Min.X), iy+float64(entry.bounds.Min.Y))
	dst.DrawImage(entry.image, &drawOp)
}
