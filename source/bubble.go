package main

import (
	"gothoom/eui"
	"gothoom/internal/renderpool"
	"image"
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
var thoughtBubbleCompositeMask *ebiten.Image

var bubbleAnimationEpoch = time.Now()

type bubbleFaceCacheKey struct {
	face  text.Face
	scale float64
}

type bubbleTextLayoutCacheKey struct {
	text            string
	face            text.Face
	maxWidth        int
	lineHeight, pad int
	balanced        bool
}

type bubbleTextLayout struct {
	width, wrapWidth int
	lines            []string
}

type bubbleTextImageCacheKey struct {
	text       string
	face       text.Face
	maxWidth   int
	lineHeight int
	r, g, b, a uint32
}

type bubbleTextImageCacheEntry struct {
	image    *ebiten.Image
	bytes    int
	lastUsed uint64
}

type bubbleBodyImageCacheKey struct {
	gapX, gapHalf    int
	gapTop           bool
	decorationType   int
	scaleBits        uint32
	width, height    int
	radiusBits       uint32
	strokeWidthBits  uint32
	fillR, fillG     uint16
	fillB, fillA     uint16
	borderR, borderG uint16
	borderB, borderA uint16
}

type bubbleBodyImageCacheEntry struct {
	image    *ebiten.Image
	margin   int
	bytes    int
	lastUsed uint64
}

var scaledBubbleFaceCache = make(map[bubbleFaceCacheKey]text.Face)
var bubbleTextLayoutCache = make(map[bubbleTextLayoutCacheKey]bubbleTextLayout)
var bubbleTextImageCache = make(map[bubbleTextImageCacheKey]*bubbleTextImageCacheEntry)
var bubbleTextImageBytes int
var bubbleTextUseCounter uint64
var bubbleBodyImageCache = make(map[bubbleBodyImageCacheKey]*bubbleBodyImageCacheEntry)
var bubbleBodyImageBytes int
var bubbleBodyUseCounter uint64

// Idle allocations survive cache eviction so later bubbles can repaint them.
// Each cache keeps its own reserve to avoid a large body displacing text slots.
var bubbleTextTargets = renderpool.Pool{MaxFreeBytes: 16 << 20, NewImage: newBubbleRenderTarget, Deallocate: deallocateImage}
var bubbleBodyTargets = renderpool.Pool{MaxFreeBytes: 8 << 20, NewImage: newBubbleRenderTarget, Deallocate: deallocateImage}

func newBubbleRenderTarget(width, height int, unmanaged bool) *ebiten.Image {
	if unmanaged {
		return newUnmanagedImage(width, height)
	}
	return newManagedImage(width, height)
}

const maxBubbleTextLayouts = 512
const bubbleTextImageMargin = 1
const maxBubbleTextImageBytes = 32 << 20
const maxBubbleBodyImages = 256
const maxBubbleBodyImageBytes = 16 << 20

const ponderBubbleAnimationSpeed = 4.0
const bubbleMaxBodyWidthFraction = 0.45
const bubbleMaxBodyHeightFraction = 0.35
const bubbleMinimumFitFontSize = 4.0
const bubbleBodyAspectRatio = 2

const (
	bubblePosNone uint8 = iota
	bubblePosUpperLeft
	bubblePosUpperRight
	bubblePosLowerRight
	bubblePosLowerLeft
)

type bubbleMetrics struct {
	face                      text.Face
	pad, tailHeight, tailHalf int
	maxLineWidth, baseWidth   int
	lines                     []string
	lineHeight, width, height int
}

func bubbleBodySizeLimit(screenWidth, screenHeight int) image.Point {
	return image.Pt(
		max(1, int(math.Round(float64(screenWidth)*bubbleMaxBodyWidthFraction))),
		max(1, int(math.Round(float64(screenHeight)*bubbleMaxBodyHeightFraction))),
	)
}

func measureBubble(txt string, typ int, bubbleScale, fontScale float64, maxBodySize image.Point) bubbleMetrics {
	if bubbleScale <= 0 {
		bubbleScale = 0.1
	}
	if fontScale <= 0 {
		fontScale = 0.1
	}
	m := bubbleMetrics{}
	m.pad = max(1, int(math.Round(5*bubbleScale)))
	m.tailHeight = bubbleTailHeight(typ, bubbleScale)
	m.tailHalf = max(1, int(math.Round(3*bubbleScale)))
	if typ&kBubbleTypeMask == kBubbleWhisper {
		m.tailHalf = max(1, int(math.Round(2*bubbleScale)))
	}
	m.maxLineWidth = max(1, maxBodySize.X-2*m.pad)
	baseFace := bubbleFont
	if typ&kBubbleTypeMask == kBubbleWhisper || typ&kBubbleTypeMask == kBubbleRealAction || typ&kBubbleTypeMask == kBubblePlayerAction || typ&kBubbleTypeMask == kBubbleNarrate {
		baseFace = bubbleFontRegular
	}
	if baseFace == nil {
		if typ&kBubbleTypeMask == kBubbleWhisper {
			baseFace = bubbleFontRegular
		} else {
			baseFace = bubbleFont
		}
	}
	layoutAtScale := func(scale float64) bubbleMetrics {
		candidate := m
		candidate.face = scaledBubbleFace(baseFace, scale)
		if candidate.face == nil {
			candidate.face = baseFace
		}
		metrics := candidate.face.Metrics()
		candidate.lineHeight = max(1, int(math.Ceil(math.Ceil(metrics.HAscent)+math.Ceil(metrics.HDescent)+math.Ceil(metrics.HLineGap))))
		candidate.maxLineWidth, candidate.baseWidth, candidate.lines = cachedBalancedBubbleTextLayout(
			txt, candidate.face, candidate.maxLineWidth, candidate.lineHeight, candidate.pad,
		)
		candidate.width = candidate.baseWidth + 2*candidate.pad
		candidate.height = candidate.lineHeight*len(candidate.lines) + 2*candidate.pad
		return candidate
	}
	fits := func(candidate bubbleMetrics) bool {
		return candidate.width <= maxBodySize.X && candidate.height <= maxBodySize.Y
	}

	preferred := layoutAtScale(fontScale)
	if fits(preferred) {
		return preferred
	}

	minimumScale := fontScale
	if goFace, ok := baseFace.(*text.GoTextFace); ok && goFace.Size > bubbleMinimumFitFontSize {
		minimumScale *= bubbleMinimumFitFontSize / goFace.Size
	}
	minimum := layoutAtScale(minimumScale)
	if !fits(minimum) {
		// Normal server bubbles fit at the configured UI's font-size floor. Retain it for
		// malformed or newline-heavy text rather than making it illegible.
		return minimum
	}

	best := minimum
	low, high := minimumScale, fontScale
	for range 10 {
		mid := (low + high) / 2
		candidate := layoutAtScale(mid)
		if fits(candidate) {
			best = candidate
			low = mid
		} else {
			high = mid
		}
	}
	return best
}

func bubbleRectForPlacement(x, y int, m bubbleMetrics, placement uint8, noTail bool) image.Rectangle {
	if placement == bubblePosNone {
		bottom := y
		if !noTail {
			bottom -= m.tailHeight
		}
		left := x - m.width/2
		return image.Rect(left, bottom-m.height, left+m.width, bottom)
	}
	gap := m.tailHeight
	var left, top int
	switch placement {
	case bubblePosUpperLeft:
		left, top = x-gap-m.width, y-gap-m.height
	case bubblePosUpperRight:
		left, top = x+gap, y-gap-m.height
	case bubblePosLowerRight:
		left, top = x+gap, y+gap
	case bubblePosLowerLeft:
		left, top = x-gap-m.width, y+gap
	}
	return image.Rect(left, top, left+m.width, top+m.height)
}

func clampBubbleRect(rect image.Rectangle, sw, sh int) image.Rectangle {
	if rect.Min.X < 0 {
		rect = rect.Add(image.Pt(-rect.Min.X, 0))
	}
	if rect.Max.X > sw {
		rect = rect.Add(image.Pt(sw-rect.Max.X, 0))
	}
	if rect.Min.Y < 0 {
		rect = rect.Add(image.Pt(0, -rect.Min.Y))
	}
	if rect.Max.Y > sh {
		rect = rect.Add(image.Pt(0, sh-rect.Max.Y))
	}
	return rect
}

func ponderBubblePhase(elapsed time.Duration) float64 {
	return elapsed.Seconds() * ponderBubbleAnimationSpeed
}

func bubbleAnimationPhase(speed float64) float64 {
	if !gs.AnimatedChatBubbles {
		return 0
	}
	return time.Since(bubbleAnimationEpoch).Seconds() * speed
}

func ponderWaveOffset(phase, spatialPhase float64, radius float32) float32 {
	return float32(math.Sin(phase+spatialPhase)) * radius * 0.3
}

const (
	yellSpikeSize    = 5
	growlSpikeSize   = 6
	ponderLobeRadius = 8
)

// bubbleOverlapMargin reserves the complete footprint of the decorations.
func bubbleOverlapMargin(typ int, bubbleScale float64) int {
	if bubbleScale <= 0 {
		bubbleScale = 0.1
	}
	var extent float64
	switch typ & kBubbleTypeMask {
	case kBubbleThought:
		extent = 3
	case kBubblePonder:
		// Lobes move by up to 30% of their radius.
		extent = ponderLobeRadius * 1.3
	case kBubbleYell:
		// The strongest yell pulse reaches 130%.
		extent = yellSpikeSize * 1.3
	case kBubbleMonster:
		// Growl spikes reach their full configured size.
		extent = growlSpikeSize
	default:
		return 0
	}
	return max(1, int(math.Ceil(extent*bubbleScale)))
}

func bubbleOverlapRect(rect image.Rectangle, margin int) image.Rectangle {
	if margin <= 0 {
		return rect
	}
	return image.Rect(rect.Min.X-margin, rect.Min.Y-margin, rect.Max.X+margin, rect.Max.Y+margin)
}

var thoughtBubbleMaskBlend = ebiten.Blend{
	BlendOperationRGB:   ebiten.BlendOperationMax,
	BlendOperationAlpha: ebiten.BlendOperationMax,
}

func init() {
	whiteImage = newManagedImage(1, 1)
	whiteImage.Fill(color.White)

	blackImage = newManagedImage(1, 1)
	blackImage.Fill(color.Black)

	grayImage = newManagedImage(1, 1)
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

// bubbleColors follows the type distinctions in CL_Images 1–3, with paired
// neutral light/dark surfaces. The existing dark-bubbles preference selects
// the mode independently of the UI palette.
func bubbleColors(typ int) (border, bg, text color.Color) {
	alpha := uint8(math.Max(0, math.Min(1, gs.BubbleOpacity)) * 255)
	dark := gs.DarkBubblesAndNames
	if dark {
		bg = color.NRGBA{0x23, 0x26, 0x29, alpha}
		text = color.NRGBA{0xef, 0xf0, 0xf1, 0xff}
		border = color.NRGBA{0xb8, 0xbd, 0xc2, 0xff}
	} else {
		bg = color.NRGBA{0xee, 0xee, 0xec, alpha}
		text = color.NRGBA{0x20, 0x23, 0x26, 0xff}
		border = color.NRGBA{0x55, 0x59, 0x5c, 0xff}
	}
	switch typ & kBubbleTypeMask {
	case kBubbleWhisper:
		if dark {
			border = color.NRGBA{0x91, 0x9c, 0xa4, 0xff}
			bg = color.NRGBA{0x29, 0x2d, 0x31, alpha}
			text = color.NRGBA{0xd0, 0xd7, 0xdb, 0xff}
		} else {
			border = color.NRGBA{0x70, 0x7a, 0x80, 0xff}
			bg = color.NRGBA{0xe3, 0xe6, 0xe6, alpha}
			text = color.NRGBA{0x3b, 0x42, 0x46, 0xff}
		}
	case kBubbleYell:
		if dark {
			border = color.NRGBA{0xe8, 0xd5, 0x6a, 0xff}
		} else {
			border = color.NRGBA{0xc0, 0xa0, 0x00, 0xff}
		}
	case kBubbleThought:
		if dark {
			bg = color.NRGBA{0x2c, 0x29, 0x36, alpha}
			border = color.NRGBA{0xa9, 0x9b, 0xc5, 0xff}
		} else {
			bg = color.NRGBA{0xed, 0xe9, 0xf3, alpha}
			border = color.NRGBA{0x7c, 0x6c, 0x99, 0xff}
		}
	case kBubblePonder:
		border = color.Transparent
	case kBubbleRealAction:
		if dark {
			border = color.NRGBA{0x79, 0xac, 0xdf, 0xff}
		} else {
			border = color.NRGBA{0x32, 0x62, 0x97, 0xff}
		}
	case kBubblePlayerAction:
		if dark {
			border = color.NRGBA{0xde, 0x8d, 0x90, 0xff}
		} else {
			border = color.NRGBA{0x9a, 0x43, 0x48, 0xff}
		}
	case kBubbleNarrate:
		if dark {
			border = color.NRGBA{0x8c, 0xc4, 0x99, 0xff}
		} else {
			border = color.NRGBA{0x3e, 0x79, 0x4c, 0xff}
		}
	case kBubbleMonster:
		if dark {
			border = color.NRGBA{0xc7, 0xc5, 0xbf, 0xff}
		} else {
			border = color.NRGBA{0x55, 0x58, 0x5b, 0xff}
		}
	}
	return
}

// scaledBubbleFace returns a text face scaled by the provided factor. When the
// scale is 1 the original face is returned. The returned face shares the same
// font source but uses an adjusted size so text is rasterized at the desired
// resolution instead of being drawn and then scaled, which avoids blurring.
func scaledBubbleFace(face text.Face, scale float64) text.Face {
	if face == nil {
		return nil
	}
	if scale <= 0 {
		scale = 1
	}
	if math.Abs(scale-1) < 1e-9 {
		return face
	}
	key := bubbleFaceCacheKey{face: face, scale: scale}
	if cached := scaledBubbleFaceCache[key]; cached != nil {
		return cached
	}
	if gf, ok := face.(*text.GoTextFace); ok {
		scaled := *gf
		scaled.Size = gf.Size * scale
		cached := text.Face(&scaled)
		scaledBubbleFaceCache[key] = cached
		return cached
	}
	return face
}

func cachedBubbleTextLayout(txt string, face text.Face, maxWidth int) (int, []string) {
	key := bubbleTextLayoutCacheKey{text: txt, face: face, maxWidth: maxWidth}
	if cached, ok := bubbleTextLayoutCache[key]; ok {
		return cached.width, cached.lines
	}
	width, lines := wrapText(txt, face, float64(maxWidth))
	if len(bubbleTextLayoutCache) >= maxBubbleTextLayouts {
		clear(bubbleTextLayoutCache)
	}
	bubbleTextLayoutCache[key] = bubbleTextLayout{width: width, lines: lines}
	return width, lines
}

// bubbleAspectDistance measures aspect-ratio error proportionally. This makes
// ratios on either side of the target comparable: for a 2:1 target, 1:1 and
// 4:1 are equally far away. A simple absolute difference would prefer the
// square even though it is proportionally no closer to the requested shape.
func bubbleAspectDistance(width, height int) float64 {
	if width <= 0 || height <= 0 {
		return math.MaxFloat64
	}
	ratio := float64(width) / float64(height)
	target := float64(bubbleBodyAspectRatio)
	if ratio < target {
		return target / ratio
	}
	return ratio / target
}

func cachedBalancedBubbleTextLayout(txt string, face text.Face, maxWidth, lineHeight, pad int) (int, int, []string) {
	key := bubbleTextLayoutCacheKey{
		text: txt, face: face, maxWidth: maxWidth,
		lineHeight: lineHeight, pad: pad, balanced: true,
	}
	if cached, ok := bubbleTextLayoutCache[key]; ok {
		return cached.wrapWidth, cached.width, cached.lines
	}

	// Wrapping determines the body's natural aspect ratio. Sample the useful
	// width range and retain the layout closest to 2:1; do not pad either axis
	// merely to manufacture the ratio.
	step := max(1, maxWidth/64)
	bestDistance := math.MaxFloat64
	bestArea := math.MaxInt
	bestWrapWidth, bestWidth := maxWidth, 0
	var bestLines []string
	consider := func(wrapWidth int) {
		width, lines := wrapText(txt, face, float64(wrapWidth))
		bodyWidth := width + 2*pad
		bodyHeight := lineHeight*len(lines) + 2*pad
		distance := bubbleAspectDistance(bodyWidth, bodyHeight)
		area := bodyWidth * bodyHeight
		if distance < bestDistance || distance == bestDistance && area < bestArea {
			bestDistance = distance
			bestArea = area
			bestWrapWidth = wrapWidth
			bestWidth = width
			bestLines = lines
		}
	}
	for wrapWidth := step; wrapWidth < maxWidth; wrapWidth += step {
		consider(wrapWidth)
	}
	consider(maxWidth)

	if len(bubbleTextLayoutCache) >= maxBubbleTextLayouts {
		clear(bubbleTextLayoutCache)
	}
	bubbleTextLayoutCache[key] = bubbleTextLayout{width: bestWidth, wrapWidth: bestWrapWidth, lines: bestLines}
	return bestWrapWidth, bestWidth, bestLines
}

func clearBubbleTextCaches() {
	clear(scaledBubbleFaceCache)
	clear(bubbleTextLayoutCache)
	for _, entry := range bubbleTextImageCache {
		bubbleTextTargets.Release(entry.image)
	}
	clear(bubbleTextImageCache)
	bubbleTextImageBytes = 0
	bubbleTextUseCounter = 0
	for _, entry := range bubbleBodyImageCache {
		bubbleBodyTargets.Release(entry.image)
	}
	clear(bubbleBodyImageCache)
	bubbleBodyImageBytes = 0
	bubbleBodyUseCounter = 0
	bubbleTextTargets.Clear()
	bubbleBodyTargets.Clear()
}

func evictOldestBubbleTextImage() bool {
	var oldestKey bubbleTextImageCacheKey
	var oldest *bubbleTextImageCacheEntry
	for key, entry := range bubbleTextImageCache {
		if oldest == nil || entry.lastUsed < oldest.lastUsed {
			oldestKey = key
			oldest = entry
		}
	}
	if oldest == nil {
		return false
	}
	// Release ownership after the label's draw has been submitted. A reused
	// parent is cleared and repainted after that draw in Ebitengine's queue.
	bubbleTextTargets.Release(oldest.image)
	bubbleTextImageBytes -= oldest.bytes
	delete(bubbleTextImageCache, oldestKey)
	return true
}

func cachedBubbleTextImage(txt string, face text.Face, maxWidth, width, lineHeight int, lines []string, textCol color.Color) *ebiten.Image {
	r, g, b, a := textCol.RGBA()
	key := bubbleTextImageCacheKey{
		text: txt, face: face, maxWidth: maxWidth, lineHeight: lineHeight,
		r: r, g: g, b: b, a: a,
	}
	bubbleTextUseCounter++
	if cached := bubbleTextImageCache[key]; cached != nil {
		cached.lastUsed = bubbleTextUseCounter
		return cached.image
	}
	height := lineHeight * len(lines)
	if width < 1 || height < 1 {
		return nil
	}
	imageWidth := width + 2*bubbleTextImageMargin
	imageHeight := height + 2*bubbleTextImageMargin
	imageBytes := int(renderpool.BytesForSize(imageWidth, imageHeight, gs.PotatoGPU))
	if imageBytes > maxBubbleTextImageBytes {
		return nil
	}
	for len(bubbleTextImageCache) >= maxBubbleTextLayouts || bubbleTextImageBytes+imageBytes > maxBubbleTextImageBytes {
		if !evictOldestBubbleTextImage() {
			return nil
		}
	}
	img := bubbleTextTargets.Acquire(imageWidth, imageHeight, gs.PotatoGPU)
	imageBytes = int(bubbleTextTargets.Bytes(img))
	for bubbleTextImageBytes+imageBytes > maxBubbleTextImageBytes {
		if !evictOldestBubbleTextImage() {
			bubbleTextTargets.Release(img)
			return nil
		}
	}
	for index, line := range lines {
		op := &text.DrawOptions{}
		op.GeoM.Translate(bubbleTextImageMargin, float64(bubbleTextImageMargin+index*lineHeight))
		op.ColorScale.ScaleWithColor(textCol)
		text.Draw(img, line, face, op)
	}
	bubbleTextImageCache[key] = &bubbleTextImageCacheEntry{image: img, bytes: imageBytes, lastUsed: bubbleTextUseCounter}
	bubbleTextImageBytes += imageBytes
	return img
}

func bubbleRoundedRectPath(left, top, right, bottom int, radius float32) vector.Path {
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
	return body
}

func evictOldestBubbleBodyImage() bool {
	var oldestKey bubbleBodyImageCacheKey
	var oldest *bubbleBodyImageCacheEntry
	for key, entry := range bubbleBodyImageCache {
		if oldest == nil || entry.lastUsed < oldest.lastUsed {
			oldestKey = key
			oldest = entry
		}
	}
	if oldest == nil {
		return false
	}
	bubbleBodyTargets.Release(oldest.image)
	bubbleBodyImageBytes -= oldest.bytes
	delete(bubbleBodyImageCache, oldestKey)
	return true
}

// cachedBubbleBodyImage pre-renders the non-animated fill and outline into a
// pooled managed texture. Bubble bodies usually survive many display frames, so
// this replaces repeated path tessellation and two vector submissions with a
// single image draw after the first frame.
func cachedBubbleBodyImage(width, height int, radius, strokeWidth float32, fill, border color.RGBA64) (*ebiten.Image, int) {
	return cachedBubbleBodyWithGap(width, height, radius, strokeWidth, fill, border, 0, 0, false)
}

func cachedBubbleBodyWithGap(width, height int, radius, strokeWidth float32, fill, border color.RGBA64, gapX, gapHalf int, gapTop bool) (*ebiten.Image, int) {
	if width < 1 || height < 1 {
		return nil, 0
	}
	key := bubbleBodyImageCacheKey{
		width: width, height: height, gapX: gapX, gapHalf: gapHalf, gapTop: gapTop,
		radiusBits: math.Float32bits(radius), strokeWidthBits: math.Float32bits(strokeWidth),
		fillR: fill.R, fillG: fill.G, fillB: fill.B, fillA: fill.A,
		borderR: border.R, borderG: border.G, borderB: border.B, borderA: border.A,
	}
	margin := int(math.Ceil(float64(strokeWidth)/2)) + 2
	return cacheBubbleSurface(key, margin, func(img *ebiten.Image) {
		body := bubbleRoundedRectPath(margin, margin, margin+width, margin+height, radius)
		fillOp := &vector.DrawPathOptions{AntiAlias: true}
		fillOp.ColorScale.ScaleWithColor(fill)
		vector.FillPath(img, &body, nil, fillOp)
		strokeOp := &vector.StrokeOptions{Width: strokeWidth}
		drawOutline := &vector.DrawPathOptions{AntiAlias: true}
		drawOutline.ColorScale.ScaleWithColor(border)
		outline := bubbleOutlinePath(margin, margin, margin+width, margin+height, radius, margin+gapX, gapHalf, gapTop)
		vector.StrokePath(img, &outline, strokeOp, drawOutline)
	})
}

func cacheBubbleSurface(key bubbleBodyImageCacheKey, margin int, paint func(*ebiten.Image)) (*ebiten.Image, int) {
	if key.width < 1 || key.height < 1 {
		return nil, 0
	}
	bubbleBodyUseCounter++
	if cached := bubbleBodyImageCache[key]; cached != nil {
		cached.lastUsed = bubbleBodyUseCounter
		return cached.image, cached.margin
	}
	imageWidth := key.width + 2*margin
	imageHeight := key.height + 2*margin
	imageBytes := int(renderpool.BytesForSize(imageWidth, imageHeight, gs.PotatoGPU))
	if imageBytes > maxBubbleBodyImageBytes {
		return nil, 0
	}
	for len(bubbleBodyImageCache) >= maxBubbleBodyImages || bubbleBodyImageBytes+imageBytes > maxBubbleBodyImageBytes {
		if !evictOldestBubbleBodyImage() {
			return nil, 0
		}
	}

	img := bubbleBodyTargets.Acquire(imageWidth, imageHeight, gs.PotatoGPU)
	imageBytes = int(bubbleBodyTargets.Bytes(img))
	for bubbleBodyImageBytes+imageBytes > maxBubbleBodyImageBytes {
		if !evictOldestBubbleBodyImage() {
			bubbleBodyTargets.Release(img)
			return nil, 0
		}
	}
	paint(img)
	// Painting a decorated surface can also populate its plain body cache.
	// Account for those entries before publishing the completed surface.
	for len(bubbleBodyImageCache) >= maxBubbleBodyImages || bubbleBodyImageBytes+imageBytes > maxBubbleBodyImageBytes {
		if !evictOldestBubbleBodyImage() {
			bubbleBodyTargets.Release(img)
			return nil, 0
		}
	}

	bubbleBodyImageCache[key] = &bubbleBodyImageCacheEntry{
		image: img, margin: margin, bytes: imageBytes, lastUsed: bubbleBodyUseCounter,
	}
	bubbleBodyImageBytes += imageBytes
	return img, margin
}

// bubbleDrawRequest keeps a bubble's measured layout and final placement
// together so all tails can be drawn before any balloon bodies or text.
type bubbleDrawRequest struct {
	tailAnchor                image.Point
	hasTailAnchor             bool
	speakerRect               image.Rectangle
	txt                       string
	x, y, typ                 int
	far, noArrow              bool
	placement                 uint8
	bodyOffset                image.Point
	borderCol, bgCol, textCol color.Color
	bubbleScale               float64
	metrics                   bubbleMetrics
}

type bubbleDrawGeometry struct {
	request                  bubbleDrawRequest
	offsetX, offsetY         int
	tailX, tailY             int
	left, top, right, bottom int
	baseX, attachY           int
	bubbleType               int
	radius, scale            float32
	fillColor, borderColor   color.RGBA64
	noArrow                  bool
}

func drawBubbleBatch(screen *ebiten.Image, requests []bubbleDrawRequest) {
	for _, request := range requests {
		drawBubbleTail(screen, request)
	}
	for _, request := range requests {
		drawBubbleBody(screen, request)
	}
}

func bubbleDrawRect(bounds image.Rectangle, request bubbleDrawRequest) (image.Rectangle, bool, bool) {
	if request.txt == "" {
		return image.Rectangle{}, false, false
	}
	sw, sh := bounds.Dx(), bounds.Dy()
	if sw <= 0 || sh <= 0 {
		return image.Rectangle{}, false, false
	}
	noArrow := request.noArrow || !bubbleHasSpeakerTail(request.typ)
	anchor := image.Pt(request.x, request.y)
	if request.hasTailAnchor {
		anchor = request.tailAnchor
	}
	if anchor.X < 0 || anchor.X >= sw || anchor.Y < 0 || anchor.Y >= sh {
		noArrow = true
	}
	rect := bubbleRectForPlacement(request.x, request.y, request.metrics, request.placement, request.far || noArrow)
	rect = clampBubbleRect(rect, sw, sh)
	rect = clampBubbleRect(rect.Add(request.bodyOffset), sw, sh)
	rect, clear := clearBubbleSpeaker(rect, request.speakerRect, bubbleOverlapMargin(request.typ, request.bubbleScale), sw, sh)
	return rect, noArrow, clear
}

func prepareBubbleDraw(screen *ebiten.Image, request bubbleDrawRequest) (bubbleDrawGeometry, bool) {
	if screen == nil || request.txt == "" {
		return bubbleDrawGeometry{}, false
	}
	bounds := screen.Bounds()
	offsetX := bounds.Min.X
	offsetY := bounds.Min.Y
	sw := bounds.Dx()
	sh := bounds.Dy()
	if sw <= 0 || sh <= 0 {
		return bubbleDrawGeometry{}, false
	}
	if request.bubbleScale <= 0 {
		request.bubbleScale = 0.1
	}

	tailX, tailY := request.x, request.y
	if request.hasTailAnchor {
		tailX, tailY = request.tailAnchor.X, request.tailAnchor.Y
	}
	rect, noArrow, ok := bubbleDrawRect(bounds, request)
	if !ok {
		return bubbleDrawGeometry{}, false
	}
	m := request.metrics
	tailHalf := m.tailHalf
	bubbleType := request.typ & kBubbleTypeMask
	left, top, right, bottom := rect.Min.X, rect.Min.Y, rect.Max.X, rect.Max.Y
	baseX := left + m.width/2
	if request.placement == bubblePosUpperLeft || request.placement == bubblePosLowerLeft {
		baseX = right - tailHalf
	} else if request.placement != bubblePosNone {
		baseX = left + tailHalf
	}
	attachY := bottom
	if request.placement == bubblePosLowerLeft || request.placement == bubblePosLowerRight {
		attachY = top
	}

	if request.hasTailAnchor {
		inset := tailHalf + int(math.Ceil(3*request.bubbleScale))
		if right-left > 2*inset {
			baseX = max(left+inset, min(right-inset, tailX))
		}
		if tailY < top {
			attachY = top
		} else if tailY > bottom {
			attachY = bottom
		}
	}
	bgR, bgG, bgB, bgA := request.bgCol.RGBA()
	bdR, bdG, bdB, bdA := request.borderCol.RGBA()

	s := float32(request.bubbleScale)
	radius := 2 * s
	if bubbleType == kBubblePonder {
		radius = 8 * s
	} else if bubbleType == kBubbleRealAction || bubbleType == kBubblePlayerAction || bubbleType == kBubbleNarrate {
		radius = 0
	}
	return bubbleDrawGeometry{
		request: request, offsetX: offsetX, offsetY: offsetY,
		tailX: tailX, tailY: tailY, left: left, top: top, right: right, bottom: bottom,
		baseX: baseX, attachY: attachY, bubbleType: bubbleType, radius: radius, scale: s,
		fillColor:   color.RGBA64{R: uint16(bgR), G: uint16(bgG), B: uint16(bgB), A: uint16(bgA)},
		borderColor: color.RGBA64{R: uint16(bdR), G: uint16(bdG), B: uint16(bdB), A: uint16(bdA)},
		noArrow:     noArrow,
	}, true
}

func bubbleBackgroundTarget(screen *ebiten.Image, bubbleType int, fillColor color.RGBA64, region image.Rectangle) (*ebiten.Image, color.RGBA64, ebiten.Blend, bool) {
	backgroundTarget := screen
	backgroundBlend := ebiten.Blend{}
	compositeThought := bubbleType == kBubbleThought || bubbleType == kBubblePonder
	if compositeThought {
		backgroundTarget = thoughtBubbleMask(screen)
		region = region.Intersect(backgroundTarget.Bounds())
		if !region.Empty() {
			backgroundTarget.SubImage(region).(*ebiten.Image).Clear()
		}
		fillColor = color.RGBA64{R: 0xffff, G: 0xffff, B: 0xffff, A: 0xffff}
		backgroundBlend = thoughtBubbleMaskBlend
	}
	return backgroundTarget, fillColor, backgroundBlend, compositeThought
}

func bubbleCompositeRegion(g bubbleDrawGeometry, tail bool) image.Rectangle {
	left := g.left + g.offsetX
	top := g.top + g.offsetY
	right := g.right + g.offsetX
	bottom := g.bottom + g.offsetY
	margin := int(math.Ceil(float64(max(g.scale, 1)))) + 2
	if tail {
		left = min(left, g.tailX+g.offsetX)
		right = max(right, g.tailX+g.offsetX)
		top = min(top, g.tailY+g.offsetY)
		bottom = max(bottom, g.tailY+g.offsetY)
		margin += g.request.metrics.tailHalf
	} else {
		margin += bubbleOverlapMargin(g.request.typ, g.request.bubbleScale)
	}
	return image.Rect(left-margin, top-margin, right+margin, bottom+margin)
}

// drawBubbleTail draws only the pointer/ponder trail. drawSpeechBubbles calls
// this for every visible bubble before drawing any balloon body or text.
func drawBubbleTail(screen *ebiten.Image, request bubbleDrawRequest) {
	g, ok := prepareBubbleDraw(screen, request)
	if !ok || g.request.far || g.noArrow {
		return
	}
	fx, fy := float32(g.offsetX), float32(g.offsetY)
	tailHalf := g.request.metrics.tailHalf
	ponderPhase := bubbleAnimationPhase(ponderBubbleAnimationSpeed)
	var tail vector.Path
	if g.bubbleType == kBubblePonder {
		r1 := float32(tailHalf)
		offset1 := r1 * 0.3 * float32(math.Sin(ponderPhase))
		cx1 := float32(g.baseX) + fx
		cy1 := float32(g.attachY) + float32(g.tailY-g.attachY)*0.25 - offset1 + fy
		tail.MoveTo(cx1+r1, cy1)
		tail.Arc(cx1, cy1, r1, 0, 2*math.Pi, vector.Clockwise)
		tail.Close()
		rMid := r1 * 0.6
		offsetMid := rMid * 0.5 * float32(math.Sin(ponderPhase+math.Pi/4))
		cxMid := float32(g.baseX+g.tailX)/2 + fx
		cyMid := float32(g.attachY) + float32(g.tailY-g.attachY)*0.65 - offsetMid + fy
		tail.MoveTo(cxMid+rMid, cyMid)
		tail.Arc(cxMid, cyMid, rMid, 0, 2*math.Pi, vector.Clockwise)
		tail.Close()
		r2 := float32(tailHalf) / 2
		offset2 := r2 * 0.6 * float32(math.Sin(ponderPhase+math.Pi/2))
		cx2 := float32(g.tailX) + fx
		cy2 := float32(g.tailY) - offset2 + fy
		tail.MoveTo(cx2+r2, cy2)
		tail.Arc(cx2, cy2, r2, 0, 2*math.Pi, vector.Clockwise)
		tail.Close()
	} else {
		tail.MoveTo(float32(g.baseX-tailHalf)+fx, float32(g.attachY)+fy)
		tail.LineTo(float32(g.tailX)+fx, float32(g.tailY)+fy)
		tail.LineTo(float32(g.baseX+tailHalf)+fx, float32(g.attachY)+fy)
		tail.Close()
	}

	compositeRegion := bubbleCompositeRegion(g, true).Intersect(screen.Bounds())
	backgroundTarget, fillColor, backgroundBlend, compositeThought := bubbleBackgroundTarget(screen, g.bubbleType, g.fillColor, compositeRegion)
	tailOp := &vector.DrawPathOptions{AntiAlias: true, Blend: backgroundBlend}
	tailOp.ColorScale.ScaleWithColor(fillColor)
	vector.FillPath(backgroundTarget, &tail, nil, tailOp)
	if g.bubbleType != kBubblePonder && g.bubbleType != kBubbleThought {
		strokeOp := &vector.StrokeOptions{Width: max(1, g.scale)}
		drawOp := &vector.DrawPathOptions{AntiAlias: true}
		drawOp.ColorScale.ScaleWithColor(g.borderColor)
		var sides vector.Path
		sides.MoveTo(float32(g.baseX-tailHalf)+fx, float32(g.attachY)+fy)
		sides.LineTo(float32(g.tailX)+fx, float32(g.tailY)+fy)
		sides.LineTo(float32(g.baseX+tailHalf)+fx, float32(g.attachY)+fy)
		vector.StrokePath(screen, &sides, strokeOp, drawOp)
	}
	if compositeThought {
		compositeThoughtBubbleBackground(screen, backgroundTarget, g.request.bgCol, compositeRegion)
	}
}

// drawBubbleBody draws the balloon and its text after every tail is already
// behind the complete bubble layer.
func drawBubbleBody(screen *ebiten.Image, request bubbleDrawRequest) {
	g, ok := prepareBubbleDraw(screen, request)
	if !ok {
		return
	}
	m := g.request.metrics
	if g.bubbleType == kBubbleWhisper || !gs.AnimatedChatBubbles && (g.bubbleType == kBubbleThought || g.bubbleType == kBubblePonder || g.bubbleType == kBubbleYell || g.bubbleType == kBubbleMonster) {
		if img, margin := cachedStaticBubbleDecoration(g); img != nil {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(g.left+g.offsetX-margin), float64(g.top+g.offsetY-margin))
			screen.DrawImage(img, op)
		} else {
			drawBubbleDecoration(screen, g, nil)
		}
	} else {
		drawBubbleDecoration(screen, g, nil)
	}

	textHeight := m.lineHeight * len(m.lines)
	textTop := g.top + (m.height-textHeight)/2 + g.offsetY
	textLeft := g.left + (m.width-m.baseWidth)/2 + g.offsetX
	if textImage := cachedBubbleTextImage(g.request.txt, m.face, m.maxLineWidth, m.baseWidth, m.lineHeight, m.lines, g.request.textCol); textImage != nil {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(textLeft-bubbleTextImageMargin), float64(textTop-bubbleTextImageMargin))
		screen.DrawImage(textImage, op)
	} else {
		for i, line := range m.lines {
			op := &text.DrawOptions{}
			op.GeoM.Translate(float64(textLeft), float64(textTop+i*m.lineHeight))
			op.ColorScale.ScaleWithColor(g.request.textCol)
			text.Draw(screen, line, m.face, op)
		}
	}
}

func drawBubbleDecoration(screen *ebiten.Image, g bubbleDrawGeometry, mask *ebiten.Image) {
	if g.bubbleType == kBubbleWhisper {
		drawWhisperBody(screen, g)
		return
	}
	fx, fy := float32(g.offsetX), float32(g.offsetY)
	ponderPhase := bubbleAnimationPhase(ponderBubbleAnimationSpeed)

	strokeW := max(1, g.scale)
	gapX, gapHalf, gapTop := bubbleOutlineGap(g)
	if g.bubbleType != kBubbleThought && g.bubbleType != kBubblePonder {
		if bodyImage, margin := cachedBubbleBodyWithGap(g.right-g.left, g.bottom-g.top, g.radius, strokeW, g.fillColor, g.borderColor, gapX, gapHalf, gapTop); bodyImage != nil {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(g.left+g.offsetX-margin), float64(g.top+g.offsetY-margin))
			screen.DrawImage(bodyImage, op)
		} else {
			body := bubbleRoundedRectPath(g.left+g.offsetX, g.top+g.offsetY, g.right+g.offsetX, g.bottom+g.offsetY, g.radius)
			fillOp := &vector.DrawPathOptions{AntiAlias: true}
			fillOp.ColorScale.ScaleWithColor(g.fillColor)
			vector.FillPath(screen, &body, nil, fillOp)
			strokeOp := &vector.StrokeOptions{Width: strokeW}
			drawOutline := &vector.DrawPathOptions{AntiAlias: true}
			drawOutline.ColorScale.ScaleWithColor(g.borderColor)
			outline := bubbleOutlinePath(g.left+g.offsetX, g.top+g.offsetY, g.right+g.offsetX, g.bottom+g.offsetY, g.radius, g.left+g.offsetX+gapX, gapHalf, gapTop)
			vector.StrokePath(screen, &outline, strokeOp, drawOutline)
		}
	} else {
		body := bubbleRoundedRectPath(g.left+g.offsetX, g.top+g.offsetY, g.right+g.offsetX, g.bottom+g.offsetY, g.radius)
		compositeRegion := bubbleCompositeRegion(g, false).Intersect(screen.Bounds())
		backgroundTarget, fillColor, backgroundBlend, compositeThought := mask, color.RGBA64{R: 0xffff, G: 0xffff, B: 0xffff, A: 0xffff}, thoughtBubbleMaskBlend, true
		if mask == nil {
			backgroundTarget, fillColor, backgroundBlend, compositeThought = bubbleBackgroundTarget(screen, g.bubbleType, g.fillColor, compositeRegion)
		}
		if g.bubbleType != kBubblePonder {
			fillOp := &vector.DrawPathOptions{AntiAlias: true, Blend: backgroundBlend}
			fillOp.ColorScale.ScaleWithColor(fillColor)
			vector.FillPath(backgroundTarget, &body, nil, fillOp)
			strokeOp := &vector.StrokeOptions{Width: strokeW}
			drawOutline := &vector.DrawPathOptions{AntiAlias: true}
			drawOutline.ColorScale.ScaleWithColor(g.borderColor)
			outline := bubbleOutlinePath(g.left+g.offsetX, g.top+g.offsetY, g.right+g.offsetX, g.bottom+g.offsetY, g.radius, g.left+g.offsetX+gapX, gapHalf, gapTop)
			vector.StrokePath(screen, &outline, strokeOp, drawOutline)
		} else {
			drawPonderWaves(backgroundTarget, g.left+g.offsetX, g.top+g.offsetY, g.right+g.offsetX, g.bottom+g.offsetY, fillColor, float64(g.scale), ponderPhase, backgroundBlend)
		}
		if compositeThought {
			compositeThoughtBubbleBackground(screen, backgroundTarget, g.request.bgCol, compositeRegion)
		}
	}

	if g.bubbleType == kBubbleThought {
		x, y := float32(g.left+g.right)/2+fx, float32(g.top)+fy
		var stone vector.Path
		stone.MoveTo(x, y-3*g.scale)
		stone.LineTo(x+1.5*g.scale, y)
		stone.LineTo(x, y+3*g.scale)
		stone.LineTo(x-1.5*g.scale, y)
		stone.Close()
		op := &vector.DrawPathOptions{AntiAlias: true}
		op.ColorScale.ScaleWithColor(g.borderColor)
		vector.FillPath(screen, &stone, nil, op)
	} else if g.bubbleType == kBubbleYell {
		drawSpikes(screen, float32(g.left)+fx, float32(g.top)+fy, float32(g.right)+fx, float32(g.bottom)+fy, g.radius, yellSpikeSize*g.scale, g.request.borderCol, g)
	} else if g.bubbleType == kBubbleMonster {
		drawMonsterSpikes(screen, float32(g.left)+fx, float32(g.top)+fy, float32(g.right)+fx, float32(g.bottom)+fy, g.radius, growlSpikeSize*g.scale, g.request.borderCol, g)
	}

}

// drawSpikes renders spiky triangles around the bubble rectangle to emphasize
// a shouted yell. Triangles are drawn pointing outward along each edge and
// around the rounded corners using the given border color. The spike length
// gently pulses over time. Omit spikes at the tail join on either edge.
func drawSpikes(screen *ebiten.Image, left, top, right, bottom, radius, size float32, col color.Color, g bubbleDrawGeometry) {
	bdR, bdG, bdB, bdA := col.RGBA()
	step := size
	phase := bubbleAnimationPhase(4)
	spikeBase := size + size*0.3*float32(math.Sin(phase))

	drawOp := &vector.DrawPathOptions{AntiAlias: true}
	drawOp.ColorScale.Scale(float32(bdR)/0xffff, float32(bdG)/0xffff, float32(bdB)/0xffff, float32(bdA)/0xffff)

	var p vector.Path
	defer func() { vector.FillPath(screen, &p, nil, drawOp) }()
	gapX, gapHalf, _ := bubbleOutlineGap(g)
	drawTriangle := func(x1, y1, x2, y2, x3, y3 float32) {
		if gapHalf > 0 && y1 == y3 && y1 == float32(g.attachY+g.offsetY) && x1 < left+float32(gapX+gapHalf) && x3 > left+float32(gapX-gapHalf) {
			return
		}
		p.MoveTo(x1, y1)
		p.LineTo(x2, y2)
		p.LineTo(x3, y3)
		p.Close()
	}

	startX := left + radius
	endX := right - radius
	for x := startX; x < endX; x += step {
		end := x + step
		mid := x + step/2
		if end > endX {
			end = endX
			mid = x + (end-x)/2
		}
		drawTriangle(x, top, mid, top-spikeBase, end, top)
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
			drawTriangle(x, bottom, mid, bottom+spike, end, bottom)
		}
	}
	drawBottom(startX, endX)

	startY := top + radius
	endY := bottom - radius
	for y := startY; y < endY; y += step {
		spike := size * (0.7 + 0.3*float32(math.Sin(phase+float64(y-startY))))
		end := y + step
		mid := y + step/2
		if end > endY {
			end = endY
			mid = y + (end-y)/2
		}

		drawTriangle(left, y, left-spike, mid, left, end)
		drawTriangle(right, y, right+spike, mid, right, end)
	}

	if radius <= 0 {
		return
	}
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

			drawTriangle(x1, y1, mx, my, x2, y2)
		}
	}

	corner(left+radius, top+radius, math.Pi, 1.5*math.Pi)
	corner(right-radius, top+radius, 1.5*math.Pi, 2*math.Pi)
	corner(right-radius, bottom-radius, 0, 0.5*math.Pi)
	corner(left+radius, bottom-radius, 0.5*math.Pi, math.Pi)
}

func drawMonsterSpikes(screen *ebiten.Image, left, top, right, bottom, radius, size float32, col color.Color, g bubbleDrawGeometry) {
	bdR, bdG, bdB, bdA := col.RGBA()
	step := size * 1.1
	phase := bubbleAnimationPhase(1)

	drawOp := &vector.DrawPathOptions{AntiAlias: true}
	drawOp.ColorScale.Scale(float32(bdR)/0xffff, float32(bdG)/0xffff, float32(bdB)/0xffff, float32(bdA)/0xffff)

	var p vector.Path
	defer func() { vector.FillPath(screen, &p, nil, drawOp) }()
	gapX, gapHalf, _ := bubbleOutlineGap(g)
	drawTriangle := func(x1, y1, x2, y2, x3, y3 float32) {
		if gapHalf > 0 && y1 == y3 && y1 == float32(g.attachY+g.offsetY) && x1 < left+float32(gapX+gapHalf) && x3 > left+float32(gapX-gapHalf) {
			return
		}
		p.MoveTo(x1, y1)
		p.LineTo(x2, y2)
		p.LineTo(x3, y3)
		p.Close()
	}

	startX := left + radius
	endX := right - radius
	for x := startX; x < endX; x += step {
		spike := size * (0.7 + 0.3*float32(math.Sin(phase+float64(x-startX))))
		end := x + step
		mid := x + step/2
		if end > endX {
			end = endX
			mid = x + (end-x)/2
		}
		drawTriangle(x, top, mid, top-spike, end, top)
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
			drawTriangle(x, bottom, mid, bottom+spike, end, bottom)
		}
	}
	drawBottom(startX, endX)

	startY := top + radius
	endY := bottom - radius
	for y := startY; y < endY; y += step {
		spike := size * (0.7 + 0.3*float32(math.Sin(phase+float64(y-startY))))
		end := y + step
		mid := y + step/2
		if end > endY {
			end = endY
			mid = y + (end-y)/2
		}

		drawTriangle(left, y, left-spike, mid, left, end)
		drawTriangle(right, y, right+spike, mid, right, end)
	}

	if radius <= 0 {
		return
	}
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

			drawTriangle(x1, y1, mx, my, x2, y2)
		}
	}

	corner(left+radius, top+radius, math.Pi, 1.5*math.Pi)
	corner(right-radius, top+radius, 1.5*math.Pi, 2*math.Pi)
	corner(right-radius, bottom-radius, 0, 0.5*math.Pi)
	corner(left+radius, bottom-radius, 0.5*math.Pi, math.Pi)
}

func drawPonderWaves(screen *ebiten.Image, left, top, right, bottom int, col color.Color, bubbleScale, phase float64, blend ebiten.Blend) {
	colR, colG, colB, colA := col.RGBA()
	waveColor := color.RGBA64{R: uint16(colR), G: uint16(colG), B: uint16(colB), A: uint16(colA)}
	if bubbleScale <= 0 {
		bubbleScale = 0.1
	}
	s := float32(bubbleScale)
	radius := float32(8) * s
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
	bodyOp := &vector.DrawPathOptions{
		AntiAlias: true,
		Blend:     blend,
	}
	bodyOp.ColorScale.ScaleWithColor(waveColor)
	defer func() { vector.FillPath(screen, &body, nil, bodyOp) }()

	r := float32(ponderLobeRadius) * s
	step := r * 1.65
	corner := float32(12) * s
	angleStep := float64(step / corner)

	draw := func(cx, cy float32) {
		body.MoveTo(cx+r, cy)
		body.Arc(cx, cy, r, 0, 2*math.Pi, vector.Clockwise)
		body.Close()
	}

	// top edge
	for x := float32(left) + corner; x <= float32(right)-corner; x += step {
		offset := ponderWaveOffset(phase, float64(x-float32(left))*0.1, r)
		draw(x, float32(top)+offset)
	}
	// top-right corner
	for a := -math.Pi / 2; a < 0; a += angleStep {
		cx := float32(right) - corner + float32(math.Cos(a))*corner
		cy := float32(top) + corner + float32(math.Sin(a))*corner
		nx := float32(math.Cos(a))
		ny := float32(math.Sin(a))
		offset := ponderWaveOffset(phase, a, r)
		draw(cx+offset*nx, cy+offset*ny)
	}
	// right edge
	for y := float32(top) + corner; y <= float32(bottom)-corner; y += step {
		offset := ponderWaveOffset(phase, float64(y-float32(top))*0.1, r)
		draw(float32(right)+offset, y)
	}
	// bottom-right corner
	for a := 0.0; a < math.Pi/2; a += angleStep {
		cx := float32(right) - corner + float32(math.Cos(a))*corner
		cy := float32(bottom) - corner + float32(math.Sin(a))*corner
		nx := float32(math.Cos(a))
		ny := float32(math.Sin(a))
		offset := ponderWaveOffset(phase, a, r)
		draw(cx+offset*nx, cy+offset*ny)
	}
	// bottom edge
	for x := float32(right) - corner; x >= float32(left)+corner; x -= step {
		offset := ponderWaveOffset(phase, float64(x-float32(left))*0.1, r)
		draw(x, float32(bottom)+offset)
	}
	// bottom-left corner
	for a := math.Pi / 2; a < math.Pi; a += angleStep {
		cx := float32(left) + corner + float32(math.Cos(a))*corner
		cy := float32(bottom) - corner + float32(math.Sin(a))*corner
		nx := float32(math.Cos(a))
		ny := float32(math.Sin(a))
		offset := ponderWaveOffset(phase, a, r)
		draw(cx+offset*nx, cy+offset*ny)
	}
	// left edge
	for y := float32(bottom) - corner; y >= float32(top)+corner; y -= step {
		offset := ponderWaveOffset(phase, float64(y-float32(top))*0.1, r)
		draw(float32(left)+offset, y)
	}
	// top-left corner
	for a := math.Pi; a < 3*math.Pi/2; a += angleStep {
		cx := float32(left) + corner + float32(math.Cos(a))*corner
		cy := float32(top) + corner + float32(math.Sin(a))*corner
		nx := float32(math.Cos(a))
		ny := float32(math.Sin(a))
		offset := ponderWaveOffset(phase, a, r)
		draw(cx+offset*nx, cy+offset*ny)
	}
}

// drawBubbleCircle draws a filled circle used by the wavy ponder bubble edges.
func drawBubbleCircle(screen *ebiten.Image, cx, cy, radius float32, col color.RGBA64, blend ebiten.Blend) {
	if col.A == 0 {
		return
	}
	var p vector.Path
	p.MoveTo(cx+radius, cy)
	p.Arc(cx, cy, radius, 0, 2*math.Pi, vector.Clockwise)
	p.Close()
	drawOp := &vector.DrawPathOptions{
		AntiAlias: true,
		Blend:     blend,
	}
	drawOp.ColorScale.ScaleWithColor(col)
	vector.FillPath(screen, &p, nil, drawOp)
}

func thoughtBubbleMask(screen *ebiten.Image) *ebiten.Image {
	bounds := screen.Bounds()
	if thoughtBubbleCompositeMask == nil || thoughtBubbleCompositeMask.Bounds() != bounds {
		if thoughtBubbleCompositeMask != nil {
			thoughtBubbleCompositeMask.Deallocate()
		}
		thoughtBubbleCompositeMask = newUnmanagedImageWithBounds(bounds)
	}
	return thoughtBubbleCompositeMask
}

func compositeThoughtBubbleBackground(screen, mask *ebiten.Image, background color.Color, region image.Rectangle) {
	region = region.Intersect(mask.Bounds()).Intersect(screen.Bounds())
	if region.Empty() {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.ColorScale.ScaleWithColor(background)
	op.GeoM.Translate(float64(region.Min.X), float64(region.Min.Y))
	screen.DrawImage(mask.SubImage(region).(*ebiten.Image), op)
}

func clearThoughtBubbleMask() {
	if thoughtBubbleCompositeMask != nil {
		thoughtBubbleCompositeMask.Deallocate()
		thoughtBubbleCompositeMask = nil
	}
}
