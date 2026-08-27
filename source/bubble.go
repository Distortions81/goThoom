package main

import (
	"gothoom/eui"
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
	text     string
	face     text.Face
	maxWidth int
}

type bubbleTextLayout struct {
	width int
	lines []string
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

var scaledBubbleFaceCache = make(map[bubbleFaceCacheKey]text.Face)
var bubbleTextLayoutCache = make(map[bubbleTextLayoutCacheKey]bubbleTextLayout)
var bubbleTextImageCache = make(map[bubbleTextImageCacheKey]*bubbleTextImageCacheEntry)
var bubbleTextImageBytes int
var bubbleTextUseCounter uint64

const maxBubbleTextLayouts = 512
const bubbleTextImageMargin = 1
const maxBubbleTextImageBytes = 32 << 20

const ponderBubbleAnimationSpeed = 4.0

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

func measureBubble(txt string, typ int, bubbleScale, fontScale float64) bubbleMetrics {
	if bubbleScale <= 0 {
		bubbleScale = 0.1
	}
	if fontScale <= 0 {
		fontScale = 0.1
	}
	m := bubbleMetrics{}
	m.pad = max(1, int(math.Round(6*bubbleScale)))
	m.tailHeight = max(1, int(math.Round(10*bubbleScale)))
	m.tailHalf = max(1, int(math.Round(6*bubbleScale)))
	m.maxLineWidth = max(1, int(math.Round(float64(gameAreaSizeX)/4*bubbleScale))-2*m.pad)
	m.face = bubbleFont
	if typ&kBubbleTypeMask == kBubbleWhisper {
		m.face = bubbleFontRegular
	}
	m.face = scaledBubbleFace(m.face, fontScale)
	if m.face == nil {
		if typ&kBubbleTypeMask == kBubbleWhisper {
			m.face = bubbleFontRegular
		} else {
			m.face = bubbleFont
		}
	}
	m.baseWidth, m.lines = cachedBubbleTextLayout(txt, m.face, m.maxLineWidth)
	m.width = int(math.Ceil(float64(m.baseWidth))) + 2*m.pad
	metrics := m.face.Metrics()
	m.lineHeight = max(1, int(math.Ceil(math.Ceil(metrics.HAscent)+math.Ceil(metrics.HDescent)+math.Ceil(metrics.HLineGap))))
	m.height = m.lineHeight*len(m.lines) + 2*m.pad
	return m
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

// bubbleColors selects the border, background, and text colors for a bubble
// based on its type. Alpha values are premultiplied to match Ebiten's color
// expectations.

func bubbleColors(typ int) (border, bg, text color.Color) {
	alpha := uint8(gs.BubbleOpacity * 255)
	if gs.DarkBubblesAndNames {
		bg = color.NRGBA{0x24, 0x24, 0x24, alpha}
		text = color.White
		switch typ & kBubbleTypeMask {
		case kBubbleWhisper:
			border = color.NRGBA{0x80, 0x80, 0x80, 0xff}
		case kBubbleYell:
			border = color.NRGBA{0xff, 0xff, 0x00, 0xff}
		case kBubbleThought:
			border = color.NRGBA{0x00, 0x00, 0x00, 0x00}
		case kBubblePonder:
			border = color.NRGBA{0x24, 0x24, 0x24, alpha}
		case kBubbleRealAction:
			border = color.NRGBA{0x00, 0x00, 0x80, 0xff}
		case kBubblePlayerAction:
			border = color.NRGBA{0x80, 0x00, 0x00, 0xff}
		case kBubbleNarrate:
			border = color.NRGBA{0x00, 0x80, 0x00, 0xff}
		case kBubbleMonster:
			border = color.NRGBA{0xd6, 0xd6, 0xd6, 0xff}
		default:
			border = color.White
		}
		return
	}
	switch typ & kBubbleTypeMask {
	case kBubbleWhisper:
		border = color.NRGBA{0x80, 0x80, 0x80, 0xff}
		bg = color.NRGBA{0x33, 0x33, 0x33, alpha}
		text = color.White
	case kBubbleYell:
		border = color.NRGBA{0xff, 0xff, 0x00, 0xff}
		bg = color.NRGBA{0xff, 0xff, 0xff, alpha}
		text = color.Black
	case kBubbleThought:
		border = color.NRGBA{0x00, 0x00, 0x00, 0x00}
		bg = color.NRGBA{0x80, 0x80, 0x80, alpha}
		text = color.Black
	case kBubblePonder:
		border = color.NRGBA{0xcc, 0xcc, 0xcc, alpha}
		bg = color.NRGBA{0xcc, 0xcc, 0xcc, alpha}
		text = color.Black
	case kBubbleRealAction:
		border = color.NRGBA{0x00, 0x00, 0x80, 0xff}
		bg = color.NRGBA{0xff, 0xff, 0xff, alpha}
		text = color.Black
	case kBubblePlayerAction:
		border = color.NRGBA{0x80, 0x00, 0x00, 0xff}
		bg = color.NRGBA{0xff, 0xff, 0xff, alpha}
		text = color.Black
	case kBubbleNarrate:
		border = color.NRGBA{0x00, 0x80, 0x00, 0xff}
		bg = color.NRGBA{0xff, 0xff, 0xff, alpha}
		text = color.Black
	case kBubbleMonster:
		border = color.NRGBA{0xd6, 0xd6, 0xd6, 0xff}
		bg = color.NRGBA{0x47, 0x47, 0x47, alpha}
		text = color.White
	default:
		border = color.White
		bg = color.NRGBA{0xff, 0xff, 0xff, alpha}
		text = color.Black
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

func clearBubbleTextCaches() {
	clear(scaledBubbleFaceCache)
	clear(bubbleTextLayoutCache)
	for _, entry := range bubbleTextImageCache {
		entry.image.Deallocate()
	}
	clear(bubbleTextImageCache)
	bubbleTextImageBytes = 0
	bubbleTextUseCounter = 0
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
	// Do not explicitly deallocate here: an older cached label might already be
	// queued as a source in this Draw. Dropping the reference lets Ebitengine's
	// deferred cleanup release it safely after submitted work completes.
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
	imageBytes := imageWidth * imageHeight * 4
	if imageBytes > maxBubbleTextImageBytes {
		return nil
	}
	for len(bubbleTextImageCache) >= maxBubbleTextLayouts || bubbleTextImageBytes+imageBytes > maxBubbleTextImageBytes {
		if !evictOldestBubbleTextImage() {
			return nil
		}
	}
	img := newManagedImage(imageWidth, imageHeight)
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

// drawBubble renders a text bubble anchored so that (x, y) corresponds to the
// bottom-center point of the balloon tail. If the bubble would extend past the
// screen edges it is clamped while leaving the tail anchored at (x, y). If far
// is true the tail is omitted and (x, y) represents the bottom-center of the
// bubble itself. The tail can also be skipped explicitly via noArrow. The typ
// parameter is currently unused but retained for future compatibility with the
// original bubble images. The colors of the border, background, and text can be
// customized via borderCol, bgCol, and textCol respectively. fontScale controls
// the font size so text is rasterized at native resolution for the current
// window scale.
func drawBubble(screen *ebiten.Image, txt string, x, y int, typ int, far bool, noArrow bool, placement uint8, borderCol, bgCol, textCol color.Color, bubbleScale, fontScale float64) {
	if txt == "" {
		return
	}
	bounds := screen.Bounds()
	offsetX := bounds.Min.X
	offsetY := bounds.Min.Y
	sw := bounds.Dx()
	sh := bounds.Dy()
	if sw <= 0 || sh <= 0 {
		return
	}
	if bubbleScale <= 0 {
		bubbleScale = 0.1
	}
	if fontScale <= 0 {
		fontScale = 0.1
	}

	tailX, tailY := x, y
	if tailX < 0 || tailX >= sw || tailY < 0 || tailY >= sh {
		noArrow = true
	}
	// Visual scale for bubbles independent of font size
	s := bubbleScale
	m := measureBubble(txt, typ, bubbleScale, fontScale)
	pad, tailHalf := m.pad, m.tailHalf
	bubbleType := typ & kBubbleTypeMask
	font, maxLineWidth, baseWidth, lines := m.face, m.maxLineWidth, m.baseWidth, m.lines
	lineHeight, width := m.lineHeight, m.width
	rect := bubbleRectForPlacement(x, y, m, placement, far || noArrow)
	rect = clampBubbleRect(rect, sw, sh)
	left, top, right, bottom := rect.Min.X, rect.Min.Y, rect.Max.X, rect.Max.Y
	baseX := left + width/2
	if placement == bubblePosUpperLeft || placement == bubblePosLowerLeft {
		baseX = right - tailHalf
	} else if placement != bubblePosNone {
		baseX = left + tailHalf
	}
	attachY := bottom
	if placement == bubblePosLowerLeft || placement == bubblePosLowerRight {
		attachY = top
	}

	bgR, bgG, bgB, bgA := bgCol.RGBA()
	bdR, bdG, bdB, bdA := borderCol.RGBA()

	radius := float32(4 * s)
	if bubbleType == kBubblePonder {
		radius = float32(8 * s)
	}

	fx := float32(offsetX)
	fy := float32(offsetY)

	var body vector.Path
	body.MoveTo(float32(left)+radius+fx, float32(top)+fy)
	body.LineTo(float32(right)-radius+fx, float32(top)+fy)
	body.Arc(float32(right)-radius+fx, float32(top)+radius+fy, radius, -math.Pi/2, 0, vector.Clockwise)
	body.LineTo(float32(right)+fx, float32(bottom)-radius+fy)
	body.Arc(float32(right)-radius+fx, float32(bottom)-radius+fy, radius, 0, math.Pi/2, vector.Clockwise)
	body.LineTo(float32(left)+radius+fx, float32(bottom)+fy)
	body.Arc(float32(left)+radius+fx, float32(bottom)-radius+fy, radius, math.Pi/2, math.Pi, vector.Clockwise)
	body.LineTo(float32(left)+fx, float32(top)+radius+fy)
	body.Arc(float32(left)+radius+fx, float32(top)+radius+fy, radius, math.Pi, 3*math.Pi/2, vector.Clockwise)
	body.Close()

	var tail vector.Path
	ponderPhase := bubbleAnimationPhase(ponderBubbleAnimationSpeed)
	if !far && !noArrow {
		if bubbleType == kBubblePonder {
			r1 := float32(tailHalf)
			offset1 := r1 * 0.3 * float32(math.Sin(ponderPhase))
			cx1 := float32(baseX) + fx
			cy1 := float32(attachY) + float32(tailY-attachY)*0.25 - offset1 + fy
			tail.MoveTo(cx1+r1, cy1)
			tail.Arc(cx1, cy1, r1, 0, 2*math.Pi, vector.Clockwise)
			tail.Close()
			rMid := r1 * 0.6
			offsetMid := rMid * 0.5 * float32(math.Sin(ponderPhase+math.Pi/4))
			cxMid := float32(baseX+tailX)/2 + fx
			cyMid := float32(attachY) + float32(tailY-attachY)*0.65 - offsetMid + fy
			tail.MoveTo(cxMid+rMid, cyMid)
			tail.Arc(cxMid, cyMid, rMid, 0, 2*math.Pi, vector.Clockwise)
			tail.Close()
			r2 := float32(tailHalf) / 2
			offset2 := r2 * 0.6 * float32(math.Sin(ponderPhase+math.Pi/2))
			cx2 := float32(tailX) + fx
			cy2 := float32(tailY) - offset2 + fy
			tail.MoveTo(cx2+r2, cy2)
			tail.Arc(cx2, cy2, r2, 0, 2*math.Pi, vector.Clockwise)
			tail.Close()
		} else {
			tail.MoveTo(float32(baseX-tailHalf)+fx, float32(attachY)+fy)
			tail.LineTo(float32(tailX)+fx, float32(tailY)+fy)
			tail.LineTo(float32(baseX+tailHalf)+fx, float32(attachY)+fy)
			tail.Close()
		}
	}

	fillColor := color.RGBA64{R: uint16(bgR), G: uint16(bgG), B: uint16(bgB), A: uint16(bgA)}
	borderColor := color.RGBA64{R: uint16(bdR), G: uint16(bdG), B: uint16(bdB), A: uint16(bdA)}
	backgroundTarget := screen
	backgroundBlend := ebiten.Blend{}
	compositeThought := bubbleType == kBubbleThought || bubbleType == kBubblePonder
	if compositeThought {
		backgroundTarget = thoughtBubbleMask(screen)
		backgroundTarget.Clear()
		fillColor = color.RGBA64{R: 0xffff, G: 0xffff, B: 0xffff, A: 0xffff}
		backgroundBlend = thoughtBubbleMaskBlend
	}

	if bubbleType != kBubblePonder {
		fillOp := &vector.DrawPathOptions{AntiAlias: true, Blend: backgroundBlend}
		fillOp.ColorScale.ScaleWithColor(fillColor)
		vector.FillPath(backgroundTarget, &body, nil, fillOp)
	}
	if !far && !noArrow {
		tailOp := &vector.DrawPathOptions{AntiAlias: true, Blend: backgroundBlend}
		tailOp.ColorScale.ScaleWithColor(fillColor)
		vector.FillPath(backgroundTarget, &tail, nil, tailOp)
	}
	if bubbleType != kBubblePonder {
		var outline vector.Path
		outline.MoveTo(float32(left)+radius+fx, float32(top)+fy)
		if !far && !noArrow && attachY == top {
			outline.LineTo(float32(baseX-tailHalf)+fx, float32(top)+fy)
			outline.LineTo(float32(tailX)+fx, float32(tailY)+fy)
			outline.LineTo(float32(baseX+tailHalf)+fx, float32(top)+fy)
		}
		outline.LineTo(float32(right)-radius+fx, float32(top)+fy)
		outline.Arc(float32(right)-radius+fx, float32(top)+radius+fy, radius, -math.Pi/2, 0, vector.Clockwise)
		outline.LineTo(float32(right)+fx, float32(bottom)-radius+fy)
		outline.Arc(float32(right)-radius+fx, float32(bottom)-radius+fy, radius, 0, math.Pi/2, vector.Clockwise)
		if !far && !noArrow && attachY == bottom {
			outline.LineTo(float32(baseX+tailHalf)+fx, float32(bottom)+fy)
			outline.LineTo(float32(tailX)+fx, float32(tailY)+fy)
			outline.LineTo(float32(baseX-tailHalf)+fx, float32(bottom)+fy)
		}
		outline.LineTo(float32(left)+radius+fx, float32(bottom)+fy)
		outline.Arc(float32(left)+radius+fx, float32(bottom)-radius+fy, radius, math.Pi/2, math.Pi, vector.Clockwise)
		outline.LineTo(float32(left)+fx, float32(top)+radius+fy)
		outline.Arc(float32(left)+radius+fx, float32(top)+radius+fy, radius, math.Pi, 3*math.Pi/2, vector.Clockwise)
		outline.Close()

		// Thicken outline a bit with scale
		strokeW := float32(math.Max(1, s))
		strokeOp := &vector.StrokeOptions{Width: strokeW}
		drawOutline := &vector.DrawPathOptions{AntiAlias: true}
		drawOutline.ColorScale.ScaleWithColor(borderColor)
		vector.StrokePath(screen, &outline, strokeOp, drawOutline)
	} else {
		drawPonderWaves(backgroundTarget, left+offsetX, top+offsetY, right+offsetX, bottom+offsetY, fillColor, s, ponderPhase, backgroundBlend)
	}

	if compositeThought {
		compositeThoughtBubbleBackground(screen, backgroundTarget, bgCol)
	}

	if bubbleType == kBubbleYell {
		gapStart, gapEnd := float32(-1), float32(-1)
		if !far && !noArrow {
			gapStart = float32(baseX-tailHalf) + fx
			gapEnd = float32(baseX+tailHalf) + fx
		}
		drawSpikes(screen, float32(left)+fx, float32(top)+fy, float32(right)+fx, float32(bottom)+fy, radius, 3*float32(s), borderCol, gapStart, gapEnd)
	} else if bubbleType == kBubbleMonster {
		gapStart, gapEnd := float32(-1), float32(-1)
		if !far && !noArrow {
			gapStart = float32(baseX-tailHalf) + fx
			gapEnd = float32(baseX+tailHalf) + fx
		}
		drawMonsterSpikes(screen, float32(left)+fx, float32(top)+fy, float32(right)+fx, float32(bottom)+fy, radius, 4*float32(s), borderCol, gapStart, gapEnd)
	}

	textTop := top + pad + offsetY
	textLeft := left + pad + offsetX
	if textImage := cachedBubbleTextImage(txt, font, maxLineWidth, baseWidth, lineHeight, lines, textCol); textImage != nil {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(textLeft-bubbleTextImageMargin), float64(textTop-bubbleTextImageMargin))
		screen.DrawImage(textImage, op)
	} else {
		for i, line := range lines {
			op := &text.DrawOptions{}
			op.GeoM.Translate(float64(textLeft), float64(textTop+i*lineHeight))
			op.ColorScale.ScaleWithColor(textCol)
			text.Draw(screen, line, font, op)
		}
	}
}

// drawSpikes renders spiky triangles around the bubble rectangle to emphasize
// a shouted yell. Triangles are drawn pointing outward along each edge and
// around the rounded corners using the given border color. The spike length
// gently pulses over time to enhance the yelling effect. bottomGapStart and
// bottomGapEnd define a segment along the bottom edge where spikes should be
// omitted (e.g. where the tail arrow attaches).
func drawSpikes(screen *ebiten.Image, left, top, right, bottom, radius, size float32, col color.Color, bottomGapStart, bottomGapEnd float32) {
	bdR, bdG, bdB, bdA := col.RGBA()
	step := size
	phase := bubbleAnimationPhase(4)
	spikeBase := size + size*0.3*float32(math.Sin(phase))

	drawOp := &vector.DrawPathOptions{AntiAlias: true}
	drawOp.ColorScale.Scale(float32(bdR)/0xffff, float32(bdG)/0xffff, float32(bdB)/0xffff, float32(bdA)/0xffff)

	drawTriangle := func(x1, y1, x2, y2, x3, y3 float32) {
		var p vector.Path
		p.MoveTo(x1, y1)
		p.LineTo(x2, y2)
		p.LineTo(x3, y3)
		p.Close()
		vector.FillPath(screen, &p, nil, drawOp)
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

	if bottomGapStart < startX {
		bottomGapStart = startX
	}
	if bottomGapEnd < bottomGapStart {
		bottomGapEnd = bottomGapStart
	}
	if bottomGapEnd > endX {
		bottomGapEnd = endX
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
	drawBottom(startX, bottomGapStart)
	drawBottom(bottomGapEnd, endX)

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

func drawMonsterSpikes(screen *ebiten.Image, left, top, right, bottom, radius, size float32, col color.Color, bottomGapStart, bottomGapEnd float32) {
	bdR, bdG, bdB, bdA := col.RGBA()
	step := size / 2
	phase := bubbleAnimationPhase(1)

	drawOp := &vector.DrawPathOptions{AntiAlias: true}
	drawOp.ColorScale.Scale(float32(bdR)/0xffff, float32(bdG)/0xffff, float32(bdB)/0xffff, float32(bdA)/0xffff)

	drawTriangle := func(x1, y1, x2, y2, x3, y3 float32) {
		var p vector.Path
		p.MoveTo(x1, y1)
		p.LineTo(x2, y2)
		p.LineTo(x3, y3)
		p.Close()
		vector.FillPath(screen, &p, nil, drawOp)
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

	if bottomGapStart < startX {
		bottomGapStart = startX
	}
	if bottomGapEnd < bottomGapStart {
		bottomGapEnd = bottomGapStart
	}
	if bottomGapEnd > endX {
		bottomGapEnd = endX
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
	drawBottom(startX, bottomGapStart)
	drawBottom(bottomGapEnd, endX)

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
	vector.FillPath(screen, &body, nil, bodyOp)

	r := float32(6) * s
	step := r * 1.2
	corner := float32(10) * s
	angleStep := float64(step / corner)

	draw := func(cx, cy float32) {
		drawBubbleCircle(screen, cx, cy, r, waveColor, blend)
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
	if thoughtBubbleCompositeMask == nil || thoughtBubbleCompositeMask.Bounds().Dx() != bounds.Dx() || thoughtBubbleCompositeMask.Bounds().Dy() != bounds.Dy() {
		if thoughtBubbleCompositeMask != nil {
			thoughtBubbleCompositeMask.Deallocate()
		}
		thoughtBubbleCompositeMask = newUnmanagedImage(bounds.Dx(), bounds.Dy())
	}
	return thoughtBubbleCompositeMask
}

func compositeThoughtBubbleBackground(screen, mask *ebiten.Image, background color.Color) {
	op := &ebiten.DrawImageOptions{}
	op.ColorScale.ScaleWithColor(background)
	screen.DrawImage(mask, op)
}

func clearThoughtBubbleMask() {
	if thoughtBubbleCompositeMask != nil {
		thoughtBubbleCompositeMask.Deallocate()
		thoughtBubbleCompositeMask = nil
	}
}
