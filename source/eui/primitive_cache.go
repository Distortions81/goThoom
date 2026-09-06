package eui

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	primitiveRoundRect uint8 = iota
	primitiveLine
	primitiveCheckmark
	primitiveTriangle
	maxPrimitiveEntries = 256
	maxPrimitiveBytes   = 2 << 20
	maxPrimitiveSide    = 256
)

// Keys describe pixel geometry only. Hover, accent, disabled color, and screen
// position all reuse the same white coverage mask.
type primitiveKey struct {
	kind          uint8
	width, height int
	radius        float32
	stroke        int
	points        [6]int
	filled, aa    bool
}

type primitiveMask struct {
	image *ebiten.Image
	used  uint64
}

type primitiveCache struct {
	entries                 map[primitiveKey]primitiveMask
	bytes                   int
	clock                   uint64
	hits, misses, evictions uint64
}

var uiPrimitives primitiveCache

func (key primitiveKey) allocationSize() (int, int) {
	if key.kind == primitiveRoundRect {
		return key.width + 2, key.height + 2
	}
	return key.width, key.height
}

func (key primitiveKey) drawable(mask *ebiten.Image) *ebiten.Image {
	if key.kind == primitiveRoundRect {
		return mask.SubImage(image.Rect(1, 1, key.width+1, key.height+1)).(*ebiten.Image)
	}
	return mask
}

func (cache *primitiveCache) clear() {
	for _, mask := range cache.entries {
		mask.image.Deallocate()
	}
	*cache = primitiveCache{}
}

func (cache *primitiveCache) mask(key primitiveKey) *ebiten.Image {
	cache.clock++
	if mask, ok := cache.entries[key]; ok {
		mask.used = cache.clock
		cache.entries[key] = mask
		cache.hits++
		return key.drawable(mask.image)
	}
	cache.misses++
	allocationW, allocationH := key.allocationSize()
	bytes := allocationW * allocationH * 4
	for len(cache.entries) >= maxPrimitiveEntries || cache.bytes+bytes > maxPrimitiveBytes {
		var oldestKey primitiveKey
		oldest := cache.clock
		for candidate, mask := range cache.entries {
			if mask.used < oldest {
				oldestKey, oldest = candidate, mask.used
			}
		}
		mask := cache.entries[oldestKey]
		mask.image.Deallocate()
		w, h := oldestKey.allocationSize()
		cache.bytes -= w * h * 4
		delete(cache.entries, oldestKey)
		cache.evictions++
	}
	mask := newManagedImage(allocationW, allocationH)
	switch key.kind {
	case primitiveRoundRect:
		drawRoundRectVector(mask, &roundRect{Position: point{1, 1}, Size: point{float32(key.width), float32(key.height)},
			Fillet: key.radius, Border: float32(key.stroke), Filled: key.filled, Color: ColorWhite})
	case primitiveLine:
		off := pixelOffset(float32(key.stroke))
		strokeLineFn(mask, float32(key.points[0])+off, float32(key.points[1])+off,
			float32(key.points[2])+off, float32(key.points[3])+off, float32(key.stroke), color.White, key.aa)
	case primitiveCheckmark:
		drawCheckmarkVector(mask, point{float32(key.points[0]), float32(key.points[1])},
			point{float32(key.points[2]), float32(key.points[3])}, point{float32(key.points[4]), float32(key.points[5])}, float32(key.stroke), ColorWhite)
	case primitiveTriangle:
		drawTriangleVector(mask, point{1, 1}, float32(key.width-2), ColorWhite)
	}
	if cache.entries == nil {
		cache.entries = make(map[primitiveKey]primitiveMask)
	}
	cache.entries[key] = primitiveMask{image: mask, used: cache.clock}
	cache.bytes += bytes
	return key.drawable(mask)
}

// Large rectangles and straight lines keep fixed end/corner pieces and stretch
// only their uniform middle bands. Small controls use a single image quad.
func drawPrimitiveMask(dst, mask *ebiten.Image, x, y, width, height, cornerX, cornerY int, tint color.Color) {
	bounds := mask.Bounds()
	draw := func(src image.Rectangle, x, y, w, h int) {
		if src.Empty() || w <= 0 || h <= 0 {
			return
		}
		part := mask.SubImage(src).(*ebiten.Image)
		op := ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
		op.GeoM.Scale(float64(w)/float64(src.Dx()), float64(h)/float64(src.Dy()))
		op.GeoM.Translate(float64(x), float64(y))
		op.ColorScale.ScaleWithColor(tint)
		dst.DrawImage(part, &op)
	}
	if width == bounds.Dx() && height == bounds.Dy() {
		draw(bounds, x, y, width, height)
		return
	}
	cx, cy := min(cornerX, bounds.Dx()/2), min(cornerY, bounds.Dy()/2)
	sx := [4]int{0, cx, bounds.Dx() - cx, bounds.Dx()}
	sy := [4]int{0, cy, bounds.Dy() - cy, bounds.Dy()}
	dx := [4]int{x, x + cx, x + width - cx, x + width}
	dy := [4]int{y, y + cy, y + height - cy, y + height}
	for row := range 3 {
		for column := range 3 {
			draw(image.Rect(sx[column], sy[row], sx[column+1], sy[row+1]).Add(bounds.Min), dx[column], dy[row], dx[column+1]-dx[column], dy[row+1]-dy[row])
		}
	}
}

func roundedPixel(v float32) int { return int(math.Round(float64(v))) }

// Pixel-aligned square frames need no raster cache entry: four non-overlapping
// quads use Ebitengine's shared white pixel, including translucent frames.
func drawPixelFrame(dst *ebiten.Image, x, y, w, h, stroke float32, tint color.Color) bool {
	if stroke <= 0 {
		return true
	}
	x, y, w, h = x-stroke/2, y-stroke/2, w+stroke, h+stroke
	for _, v := range []float32{x, y, w, h, stroke} {
		if v != float32(roundedPixel(v)) {
			return false
		}
	}
	if w <= 2*stroke || h <= 2*stroke {
		drawFilledRect(dst, x, y, w, h, tint, false)
		return true
	}
	drawFilledRect(dst, x, y, w, stroke, tint, false)
	drawFilledRect(dst, x, y+h-stroke, w, stroke, tint, false)
	drawFilledRect(dst, x, y+stroke, stroke, h-2*stroke, tint, false)
	drawFilledRect(dst, x+w-stroke, y+stroke, stroke, h-2*stroke, tint, false)
	return true
}

func drawCachedRoundRect(dst *ebiten.Image, rect *roundRect) bool {
	if rect.Fillet <= 0 {
		if rect.Filled {
			drawFilledRect(dst, rect.Position.X, rect.Position.Y, rect.Size.X, rect.Size.Y, color.RGBA(rect.Color), false)
		}
		border := float32(roundedPixel(rect.Border))
		if border > 0 {
			inset := border / 2
			return drawPixelFrame(dst, float32(roundedPixel(rect.Position.X))+inset, float32(roundedPixel(rect.Position.Y))+inset,
				max(0, float32(roundedPixel(rect.Size.X))-border), max(0, float32(roundedPixel(rect.Size.Y))-border), border, color.RGBA(rect.Color))
		}
		return true
	}
	x, y := roundedPixel(rect.Position.X), roundedPixel(rect.Position.Y)
	w, h := roundedPixel(rect.Position.X+rect.Size.X)-x, roundedPixel(rect.Position.Y+rect.Size.Y)-y
	border := roundedPixel(rect.Border)
	if w <= 0 || h <= 0 || (!rect.Filled && border <= 0) {
		return true
	}
	// Match the vector path's inset, clamping, and rounding before choosing
	// corner dimensions. This also keeps circular radio buttons unchanged.
	inset := float32(0)
	if !rect.Filled {
		inset = float32(border) / 2
	} else {
		border = 0
	}
	radius := float32(roundedPixel(max(0, min(max(0, rect.Fillet-inset), (float32(w)-2*inset)/2, (float32(h)-2*inset)/2)))) + inset
	corner := int(math.Ceil(float64(max(radius, float32(border))))) + 1
	if corner > (maxPrimitiveSide-1)/2 {
		return false
	}
	key := primitiveKey{kind: primitiveRoundRect, width: min(w, 2*corner+1), height: min(h, 2*corner+1), radius: radius, stroke: border, filled: rect.Filled}
	if w <= 64 && h <= 64 {
		key.width, key.height = w, h
	}
	mask := uiPrimitives.mask(key)
	drawPrimitiveMask(dst, mask, x, y, w, h, corner, corner, color.RGBA(rect.Color))
	return true
}

// Receives the same snapped endpoints as vector.StrokeLine. Axis-aligned
// tracks share short strips across all lengths; small diagonals share masks.
func drawCachedLine(dst *ebiten.Image, x0, y0, x1, y1, width float32, tint color.Color, aa bool) bool {
	stroke := int(width)
	if stroke <= 0 {
		return true
	}
	if stroke > 32 {
		return false
	}
	off := pixelOffset(width)
	xa, ya, xb, yb := roundedPixel(x0-off), roundedPixel(y0-off), roundedPixel(x1-off), roundedPixel(y1-off)
	pad := (stroke+1)/2 + 1
	minX, minY := min(xa, xb), min(ya, yb)
	w, h := max(xa, xb)-minX, max(ya, yb)-minY
	mw, mh := w, h
	if ya == yb {
		mw = min(w, 2*pad+1)
		xa, xb = minX, minX+mw
	} else if xa == xb {
		mh = min(h, 2*pad+1)
		ya, yb = minY, minY+mh
	} else if w > 64 || h > 64 {
		return false
	}
	key := primitiveKey{kind: primitiveLine, width: mw + 2*pad + 1, height: mh + 2*pad + 1, stroke: stroke, aa: aa,
		points: [6]int{xa - minX + pad, ya - minY + pad, xb - minX + pad, yb - minY + pad}}
	mask := uiPrimitives.mask(key)
	drawPrimitiveMask(dst, mask, minX-pad, minY-pad, w+2*pad+1, h+2*pad+1, pad+1, pad+1, tint)
	return true
}

func drawCachedCheckmark(dst *ebiten.Image, start, mid, end point, width float32, tint Color) bool {
	stroke := roundedPixel(width)
	if stroke <= 0 {
		return true
	}
	pad := (stroke+1)/2 + 1
	x0, y0, x1, y1, x2, y2 := roundedPixel(start.X), roundedPixel(start.Y), roundedPixel(mid.X), roundedPixel(mid.Y), roundedPixel(end.X), roundedPixel(end.Y)
	x, y := min(x0, x1, x2)-pad, min(y0, y1, y2)-pad
	w, h := max(x0, x1, x2)-x+pad+1, max(y0, y1, y2)-y+pad+1
	if w > 64 || h > 64 {
		return false
	}
	key := primitiveKey{kind: primitiveCheckmark, width: w, height: h, stroke: stroke,
		points: [6]int{x0 - x, y0 - y, x1 - x, y1 - y, x2 - x, y2 - y}}
	mask := uiPrimitives.mask(key)
	drawPrimitiveMask(dst, mask, x, y, w, h, 0, 0, color.RGBA(tint))
	return true
}

func drawCachedTriangle(dst *ebiten.Image, pos point, size float32, tint Color) bool {
	s := roundedPixel(size)
	if s <= 0 || s > 64 {
		return false
	}
	key := primitiveKey{kind: primitiveTriangle, width: s + 2, height: s + 2}
	mask := uiPrimitives.mask(key)
	drawPrimitiveMask(dst, mask, roundedPixel(pos.X)-1, roundedPixel(pos.Y)-1, s+2, s+2, 0, 0, color.RGBA(tint))
	return true
}
