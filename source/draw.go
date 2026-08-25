package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image/color"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// frameDescriptor describes an on-screen descriptor.
type frameDescriptor struct {
	Index  uint8
	Type   uint8
	PictID uint16
	Name   string
	Colors []byte
	Plane  int
}

type framePicture struct {
	PictID       uint16
	H, V         int16
	PrevH, PrevV int16
	Plane        int
	Moving       bool
	Background   bool
	Owned        bool
	Again        bool
	obscuredPrev bool
	obscuredNow  bool
}

type frameMobile struct {
	Index   uint8
	State   uint8
	H, V    int16
	Colors  uint8
	Persist bool
	// Cached name tag image (for d.Name != "").
	nameTag    *ebiten.Image
	nameTagW   int
	nameTagH   int
	nameTagKey nameTagKey
}

type nameTagKey struct {
	Text          string
	Colors        uint8
	Type          uint8
	HealthOptions uint16
	Opacity       uint8
	FontGen       uint32
	Style         uint8
	Dead          bool
	FrameColor    color.RGBA
}

func reuseCachedNameTag(m *frameMobile, previous map[uint8]frameMobile, key nameTagKey) bool {
	prev, ok := previous[m.Index]
	if ok && prev.nameTag != nil && prev.nameTagKey == key {
		m.nameTag = prev.nameTag
		m.nameTagW = prev.nameTagW
		m.nameTagH = prev.nameTagH
		m.nameTagKey = prev.nameTagKey
		return true
	}
	return reuseSharedNameTag(m, key)
}

const (
	styleRegular uint8 = iota
	styleBold
	styleItalic
	styleBoldItalic
)

const poseDead = 32
const maxInterpPixels = 64
const maxMobileInterpPixels = 64
const maxPersistImageSize = 512
const minMovingPicturePixels = 12000

// percent of area that must be outside the field to count as "on the edge"
const edgeOutsidePercent = 70
const secondBestShiftRatio = 0.4

// sanity limits for parsed counts to avoid excessive allocations or
// obviously corrupt packets.
const (
	maxDescriptors = 512
	maxPictures    = 512
	maxMobiles     = 512
	maxBubbles     = 128
)

var mobileSizeFunc = mobileSize

var skipPictShift = map[uint16]struct{}{
	3037: {},
}

var pictureSemiTransparent = func(id uint16) bool {
	return clImages != nil && clImages.IsSemiTransparent(uint32(id))
}

var pictureSize = func(id uint16) (int, int) {
	if clImages == nil {
		return 0, 0
	}
	return clImages.Size(uint32(id))
}

// pictureCloudMotionEnabled identifies sprites, such as clouds, that move
// with the camera but also drift independently. Clouds render after mobiles,
// so the automatic path requires a positive plane as well as large dimensions
// and a semi-transparent blend flag.
func pictureCloudMotionEnabled(p framePicture) bool {
	if p.Plane <= 0 {
		return false
	}
	w, h := pictureSize(p.PictID)
	large := w >= 128 || h >= 128 || nonTransparentPixels(p.PictID) >= minMovingPicturePixels
	return large && pictureSemiTransparent(p.PictID)
}

func pictureMotionInterpolationEnabled(p framePicture) bool {
	return gs.smoothMoving || pictureCloudMotionEnabled(p)
}

func pictureMotionBlockedAtEdge(p framePicture, cloudMotion bool) bool {
	return !cloudMotion && pictureOnEdge(p)
}

// pictureExcludedFromShift keeps independently drifting pictures from
// influencing the shared camera-motion estimate.
func pictureExcludedFromShift(p framePicture) bool {
	if _, skip := skipPictShift[p.PictID]; skip {
		return true
	}
	return pictureCloudMotionEnabled(p)
}

type pictureShiftScratch struct {
	counts     map[[2]int]int
	idxMap     map[[2]int]map[int]struct{}
	curIdx     map[uint16][]int
	curIDs     []uint16
	pixelCache map[uint16]int
	idxSetPool []map[int]struct{}
}

type picturePositionMatch struct {
	current  int
	previous int
	distance int
}

type picturePositionScratch struct {
	candidates []picturePositionMatch
	matches    []int
	used       []bool
}

var picturePositionScratchPool = sync.Pool{
	New: func() any { return new(picturePositionScratch) },
}

func acquirePicturePositionScratch(currentCount, previousCount int) *picturePositionScratch {
	scratch := picturePositionScratchPool.Get().(*picturePositionScratch)
	scratch.candidates = scratch.candidates[:0]
	if cap(scratch.matches) < currentCount {
		scratch.matches = make([]int, currentCount)
	} else {
		scratch.matches = scratch.matches[:currentCount]
	}
	for i := range scratch.matches {
		scratch.matches[i] = -1
	}
	if cap(scratch.used) < previousCount {
		scratch.used = make([]bool, previousCount)
	} else {
		scratch.used = scratch.used[:previousCount]
		clear(scratch.used)
	}
	return scratch
}

func releasePicturePositionScratch(scratch *picturePositionScratch) {
	picturePositionScratchPool.Put(scratch)
}

// matchPicturePositions pairs each current picture with at most one previous
// picture of the same ID. Matching is global and camera-adjusted, rather than
// depending on draw order, so identical nearby sprites cannot steal one
// another's previous location.
func matchPicturePositions(prev, cur []framePicture, shiftX, shiftY, max int, again int) *picturePositionScratch {
	scratch := acquirePicturePositionScratch(len(cur), len(prev))
	maxDistance := max * max
	for current := again; current < len(cur); current++ {
		for previous := range prev {
			if prev[previous].Again || prev[previous].PictID != cur[current].PictID {
				continue
			}
			dh := int(cur[current].H) - int(prev[previous].H) - shiftX
			dv := int(cur[current].V) - int(prev[previous].V) - shiftY
			distance := dh*dh + dv*dv
			if distance <= maxDistance {
				scratch.candidates = append(scratch.candidates, picturePositionMatch{current, previous, distance})
			}
		}
	}
	slices.SortFunc(scratch.candidates, func(a, b picturePositionMatch) int {
		if a.distance != b.distance {
			return cmpInt(a.distance, b.distance)
		}
		if a.current != b.current {
			return cmpInt(a.current, b.current)
		}
		return cmpInt(a.previous, b.previous)
	})
	for _, candidate := range scratch.candidates {
		if scratch.matches[candidate.current] >= 0 {
			continue
		}
		if scratch.used[candidate.previous] {
			continue
		}
		scratch.matches[candidate.current] = candidate.previous
		scratch.used[candidate.previous] = true
	}
	return scratch
}

func cmpInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func newPictureShiftScratch() *pictureShiftScratch {
	return &pictureShiftScratch{
		counts:     make(map[[2]int]int),
		idxMap:     make(map[[2]int]map[int]struct{}),
		curIdx:     make(map[uint16][]int),
		pixelCache: make(map[uint16]int),
	}
}

func (s *pictureShiftScratch) reset() {
	for k := range s.counts {
		delete(s.counts, k)
	}
	for key, inner := range s.idxMap {
		for idx := range inner {
			delete(inner, idx)
		}
		s.idxSetPool = append(s.idxSetPool, inner)
		delete(s.idxMap, key)
	}
	for _, id := range s.curIDs {
		s.curIdx[id] = s.curIdx[id][:0]
	}
	s.curIDs = s.curIDs[:0]
	for k := range s.pixelCache {
		delete(s.pixelCache, k)
	}
}

var pictureShiftScratchPool = sync.Pool{
	New: func() any {
		return newPictureShiftScratch()
	},
}

type drawParseScratch struct {
	descriptors         []frameDescriptor
	pictures            []framePicture
	mobiles             []frameMobile
	bubbles             []bubble
	pictureShiftIndices []int
	mobilePresent       map[uint8]struct{}
}

func clearMap[K comparable, V any](m map[K]V) {
	for k := range m {
		delete(m, k)
	}
}

func newDrawParseScratch() *drawParseScratch {
	return &drawParseScratch{
		mobilePresent: make(map[uint8]struct{}),
	}
}

func (s *drawParseScratch) reset() {
	s.descriptors = s.descriptors[:0]
	s.pictures = s.pictures[:0]
	s.mobiles = s.mobiles[:0]
	s.bubbles = s.bubbles[:0]
	s.pictureShiftIndices = s.pictureShiftIndices[:0]
	if s.mobilePresent == nil {
		s.mobilePresent = make(map[uint8]struct{})
	} else {
		clearMap(s.mobilePresent)
	}
}

var drawParseScratchPool = sync.Pool{
	New: func() any {
		return newDrawParseScratch()
	},
}

func acquireDrawParseScratch() *drawParseScratch {
	scratch, _ := drawParseScratchPool.Get().(*drawParseScratch)
	if scratch == nil {
		scratch = newDrawParseScratch()
	} else {
		scratch.reset()
	}
	return scratch
}

func releaseDrawParseScratch(scratch *drawParseScratch) {
	scratch.reset()
	drawParseScratchPool.Put(scratch)
}

func sortPictures(pics []framePicture) {
	slices.SortFunc(pics, func(a, b framePicture) int {
		if a.Plane != b.Plane {
			return cmpInt(a.Plane, b.Plane)
		}
		if a.V != b.V {
			return cmpInt(int(a.V), int(b.V))
		}
		return cmpInt(int(a.H), int(b.H))
	})
}

func sortMobiles(mobs []frameMobile) {
	slices.SortFunc(mobs, func(a, b frameMobile) int {
		if a.V != b.V {
			return cmpInt(int(a.V), int(b.V))
		}
		return cmpInt(int(a.H), int(b.H))
	})
}

func sortMobilesNameTags(mobs []frameMobile) {
	slices.SortFunc(mobs, func(a, b frameMobile) int {
		if a.H != b.H {
			return cmpInt(int(b.H), int(a.H))
		}
		return cmpInt(int(a.V), int(b.V))
	})
}

// bitReader helps decode the packed picture fields.
type bitReader struct {
	data   []byte
	bitPos int
}

func (br *bitReader) readBits(n int) (uint32, bool) {
	var v uint32
	for n > 0 {
		if br.bitPos/8 >= len(br.data) {
			return v, false
		}
		b := br.data[br.bitPos/8]
		remain := 8 - br.bitPos%8
		take := remain
		if take > n {
			take = n
		}
		shift := remain - take
		v = (v << take) | uint32((b>>shift)&((1<<take)-1))
		br.bitPos += take
		n -= take
	}
	return v, true
}

func signExtend(v uint32, bits int) int16 {
	if v&(1<<(bits-1)) != 0 {
		v |= ^uint32(0) << bits
	}
	return int16(int32(v))
}

// picturesSummary returns a compact string of picture IDs and coordinates for
// debugging. At most the first 8 entries are included.
func picturesSummary(pics []framePicture) string {
	const max = 8
	var buf bytes.Buffer
	for i, p := range pics {
		if i >= max {
			buf.WriteString("...")
			break
		}
		fmt.Fprintf(&buf, "%d:(%d,%d) ", p.PictID, p.H, p.V)
	}
	return buf.String()
}

// drawScriptOverlays renders per-script overlay operations onto the provided
// world view image. Coordinates are in world units with a top-left origin
// matching the base game area (0..gameAreaSizeX, 0..gameAreaSizeY). The
// provided scale converts world units to worldView pixels.
func drawScriptOverlays(worldView *ebiten.Image, scale float64) {
	if worldView == nil || scale <= 0 {
		return
	}
	overlayMu.RLock()
	// Snapshot to avoid holding the lock while drawing
	snap := make([]overlayOp, 0, 64)
	for _, ops := range scriptOverlayOps {
		snap = append(snap, ops...)
	}
	overlayMu.RUnlock()
	if len(snap) == 0 {
		return
	}
	origin := worldView.Bounds().Min

	// Sort to help Ebiten batch identical draw operations.
	slices.SortFunc(snap, func(a, b overlayOp) int {
		if a.kind != b.kind {
			return cmpInt(a.kind, b.kind)
		}
		if a.kind == 2 {
			return cmpInt(int(a.id), int(b.id))
		}
		return 0
	})

	var rects rectBatch
	for _, op := range snap {
		switch op.kind {
		case 0: // rect
			if op.w > 0 && op.h > 0 {
				rects.Add(
					float32(float64(origin.X)+float64(op.x)*scale),
					float32(float64(origin.Y)+float64(op.y)*scale),
					float32(float64(op.w)*scale),
					float32(float64(op.h)*scale),
					color.RGBA{op.r, op.g, op.b, op.a},
				)
			}
		case 1: // text
			if op.text != "" {
				opts := &text.DrawOptions{}
				opts.GeoM.Translate(float64(origin.X)+float64(op.x)*scale, float64(origin.Y)+float64(op.y)*scale)
				// Use mainFont (native scale), color with RGBA
				opts.ColorScale.ScaleWithColor(color.RGBA{op.r, op.g, op.b, op.a})
				text.Draw(worldView, op.text, mainFont, opts)
			}
		case 2: // image by ID
			if img := loadImage(op.id); img != nil {
				di := acquireDrawOpts()
				di.Filter = ebiten.FilterLinear
				di.GeoM.Scale(scale, scale)
				di.GeoM.Translate(float64(origin.X)+float64(op.x)*scale, float64(origin.Y)+float64(op.y)*scale)
				worldView.DrawImage(img, di)
				releaseDrawOpts(di)
			}
		}
	}
	rects.Draw(worldView)
}

var (
	pixelCountMu    sync.RWMutex
	pixelCountCache = make(map[uint16]int)
)

// nonTransparentPixels returns the number of non-transparent pixels for the
// given picture ID. When CL_Images data is available it avoids GPU readbacks
// by using the decoded pixel data directly and falls back to a generic path
// otherwise.
func nonTransparentPixels(id uint16) int {
	pixelCountMu.RLock()
	if c, ok := pixelCountCache[id]; ok {
		pixelCountMu.RUnlock()
		return c
	}
	pixelCountMu.RUnlock()

	count := 0
	if clImages != nil {
		count = clImages.NonTransparentPixels(uint32(id))
	} else if img := loadImage(id); img != nil {
		bounds := img.Bounds()
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				_, _, _, a := img.At(x, y).RGBA()
				if a != 0 {
					count++
				}
			}
		}
	}

	pixelCountMu.Lock()
	pixelCountCache[id] = count
	pixelCountMu.Unlock()
	return count
}

// pictureOnEdge reports whether the picture overlaps the visible game field and
// whether its bounding box touches or extends past the field boundaries.
// Pictures entirely outside the field are not considered to be on the edge.
// It returns true only when one or two edges are crossed and at least 70%
// of the picture lies outside the field.
func pictureOnEdge(p framePicture) bool {
	if !pictureVisible(p) {
		return false
	}
	w, h := clImages.Size(uint32(p.PictID))
	if w > maxPersistImageSize || h > maxPersistImageSize {
		return false
	}
	halfW := w / 2
	halfH := h / 2
	left := int(p.H) - halfW
	right := int(p.H) + halfW
	top := int(p.V) - halfH
	bottom := int(p.V) + halfH
	if right < -fieldCenterX || left > fieldCenterX ||
		bottom < -fieldCenterY || top > fieldCenterY {
		return false
	}
	edgeLeft := left <= -fieldCenterX
	edgeRight := right >= fieldCenterX
	edgeTop := top <= -fieldCenterY
	edgeBottom := bottom >= fieldCenterY

	edgeCount := 0
	if edgeLeft {
		edgeCount++
	}
	if edgeRight {
		edgeCount++
	}
	if edgeTop {
		edgeCount++
	}
	if edgeBottom {
		edgeCount++
	}
	if edgeCount == 0 || edgeCount > 2 {
		return false
	}

	interLeft := left
	if interLeft < -fieldCenterX {
		interLeft = -fieldCenterX
	}
	interRight := right
	if interRight > fieldCenterX {
		interRight = fieldCenterX
	}
	interTop := top
	if interTop < -fieldCenterY {
		interTop = -fieldCenterY
	}
	interBottom := bottom
	if interBottom > fieldCenterY {
		interBottom = fieldCenterY
	}
	visibleW := interRight - interLeft
	visibleH := interBottom - interTop
	if visibleW <= 0 || visibleH <= 0 {
		return false
	}
	totalArea := w * h
	visibleArea := visibleW * visibleH
	offArea := totalArea - visibleArea
	// Require at least edgeOutsidePercent% of the picture outside the field
	if offArea*100 < totalArea*edgeOutsidePercent {
		return false
	}
	return true
}

// pictureVisible reports whether the picture overlaps the visible field.
func pictureVisible(p framePicture) bool {
	if clImages == nil {
		return false
	}
	w, h := clImages.Size(uint32(p.PictID))
	halfW := w / 2
	halfH := h / 2
	// Check if the bounding box intersects the field rectangle.
	if int(p.H)+halfW < -fieldCenterX ||
		int(p.H)-halfW > fieldCenterX ||
		int(p.V)+halfH < -fieldCenterY ||
		int(p.V)-halfH > fieldCenterY {
		return false
	}
	return true
}

// mobileOnEdge reports whether the mobile overlaps the visible game field and
// whether its bounding box touches or extends past the field boundaries. It
// returns true only when one or two edges are crossed and at least edgeOutsidePercent of the
// mobile lies outside the field.
func mobileOnEdge(m frameMobile, d frameDescriptor) bool {
	size := mobileSizeFunc(d.PictID)
	if size <= 0 || size > maxPersistImageSize {
		return false
	}
	half := size / 2
	left := int(m.H) - half
	right := int(m.H) + half
	top := int(m.V) - half
	bottom := int(m.V) + half
	if right < -fieldCenterX || left > fieldCenterX ||
		bottom < -fieldCenterY || top > fieldCenterY {
		return false
	}
	edgeLeft := left <= -fieldCenterX
	edgeRight := right >= fieldCenterX
	edgeTop := top <= -fieldCenterY
	edgeBottom := bottom >= fieldCenterY

	edgeCount := 0
	if edgeLeft {
		edgeCount++
	}
	if edgeRight {
		edgeCount++
	}
	if edgeTop {
		edgeCount++
	}
	if edgeBottom {
		edgeCount++
	}
	if edgeCount == 0 || edgeCount > 2 {
		return false
	}

	interLeft := left
	if interLeft < -fieldCenterX {
		interLeft = -fieldCenterX
	}
	interRight := right
	if interRight > fieldCenterX {
		interRight = fieldCenterX
	}
	interTop := top
	if interTop < -fieldCenterY {
		interTop = -fieldCenterY
	}
	interBottom := bottom
	if interBottom > fieldCenterY {
		interBottom = fieldCenterY
	}
	visibleW := interRight - interLeft
	visibleH := interBottom - interTop
	if visibleW <= 0 || visibleH <= 0 {
		return false
	}
	totalArea := size * size
	visibleArea := visibleW * visibleH
	offArea := totalArea - visibleArea
	// Require at least edgeOutsidePercent% of the mobile outside the field
	if offArea*100 < totalArea*edgeOutsidePercent {
		return false
	}
	return true
}

// buildNameTagImage creates a cached image for a mobile name tag using the
// current font and settings. Returns the image and its width/height in pixels.
// A non-zero frameClr draws the optional player-label frame.
func buildNameTagImage(name string, colorCode, descriptorType, opacity, style uint8, dead bool, frameClr color.RGBA) (*ebiten.Image, int, int) {
	if name == "" {
		return nil, 0, 0
	}
	textClr, bgClr, _ := mobileNameColors(colorCode, descriptorType)
	if dead {
		textClr = color.RGBA{0x90, 0x90, 0x90, 0xff}
	}
	face := mainFont
	switch style {
	case styleBold:
		face = mainFontBold
	case styleItalic:
		face = mainFontItalic
	case styleBoldItalic:
		face = mainFontBoldItalic
	}
	w, h := text.Measure(name, face, 0)
	iw := int(math.Ceil(w))
	ih := int(math.Ceil(h))
	if iw <= 0 || ih <= 0 {
		iw, ih = 1, 1
	}
	img := newImage(iw+5, ih)
	// Fill background
	op := acquireDrawOpts()
	op.GeoM.Scale(float64(iw+5), float64(ih))
	op.ColorScale.ScaleWithColor(bgClr)
	op.ColorScale.ScaleAlpha(float32(opacity) / 255)
	img.DrawImage(whiteImage, op)
	releaseDrawOpts(op)
	if frameClr.A > 0 {
		// Border aligned to the filled background.
		width := float32(iw + 5)
		height := float32(ih)
		vector.FillRect(img, 0, 0, width, 1, frameClr, false)
		vector.FillRect(img, 0, height-1, width, 1, frameClr, false)
		vector.FillRect(img, 0, 0, 1, height, frameClr, false)
		vector.FillRect(img, width-1, 0, 1, height, frameClr, false)
	}
	// Text
	opTxt := &text.DrawOptions{}
	opTxt.GeoM.Translate(2, 2)
	opTxt.ColorScale.ScaleWithColor(textClr)
	text.Draw(img, name, face, opTxt)
	return img, iw + 5, ih
}

func nameHealthBarOffsets(nameHeight, barHeight int, above bool) (nameY, barY int) {
	if barHeight > 0 && above {
		return barHeight, 0
	}
	return 0, nameHeight
}

func nameHealthOptionsKey() uint16 {
	if gs.NameHealthBarModern {
		if gs.DarkBubblesAndNames {
			return 3
		}
		return 1
	}
	return 0
}

// pictureShift returns the (dx, dy) movement that most on-screen pictures agree on
// between two consecutive frames. Pictures are matched by PictID (duplicates
// included) and weighted by their non-transparent pixel counts. The returned
// slice contains the indexes within the current frame that contributed to the
// winning movement. The boolean result is false when no majority offset is
// found. max sets the maximum allowed pixel delta for the returned shift.
func pictureShift(prev, cur []framePicture, max int) (int, int, []int, bool) {
	return pictureShiftInto(prev, cur, max, nil)
}

func pictureShiftInto(prev, cur []framePicture, max int, idxs []int) (int, int, []int, bool) {
	idxs = idxs[:0]
	if len(prev) == 0 || len(cur) == 0 {
		//logDebug("pictureShift: no data prev=%d cur=%d", len(prev), len(cur))
		return 0, 0, idxs, false
	}

	scratch := pictureShiftScratchPool.Get().(*pictureShiftScratch)
	scratch.reset()
	defer pictureShiftScratchPool.Put(scratch)

	const maxWeight = 100000

	counts := scratch.counts
	idxMap := scratch.idxMap
	curIdx := scratch.curIdx
	pixelCache := scratch.pixelCache
	total := 0
	maxInt := int(^uint(0) >> 1)

	// Build a map from PictID to indexes in the current frame to avoid
	// repeatedly scanning the entire list for matches.
	for i, c := range cur {
		if pictureExcludedFromShift(c) {
			continue
		}
		if len(curIdx[c.PictID]) == 0 {
			scratch.curIDs = append(scratch.curIDs, c.PictID)
		}
		curIdx[c.PictID] = append(curIdx[c.PictID], i)
	}

	// Cache pixel counts locally so that each PictID is computed at most once
	// per pictureShift invocation.
	for _, p := range prev {
		if pictureExcludedFromShift(p) {
			continue
		}
		bestDist := maxInt
		var bestDx, bestDy int
		bestIdx := -1
		matched := false
		for _, j := range curIdx[p.PictID] {
			c := cur[j]
			dx := int(c.H) - int(p.H)
			dy := int(c.V) - int(p.V)
			dist := dx*dx + dy*dy
			if dist < bestDist {
				bestDist = dist
				bestDx = dx
				bestDy = dy
				bestIdx = j
				matched = true
			}
		}
		if matched {
			pixels, ok := pixelCache[p.PictID]
			if !ok {
				pixels = nonTransparentPixels(p.PictID)
				pixelCache[p.PictID] = pixels
			}
			if pixels > maxWeight {
				pixels = maxWeight
			}
			key := [2]int{bestDx, bestDy}
			counts[key] += pixels
			set := idxMap[key]
			if set == nil {
				if n := len(scratch.idxSetPool); n > 0 {
					set = scratch.idxSetPool[n-1]
					scratch.idxSetPool = scratch.idxSetPool[:n-1]
				} else {
					set = make(map[int]struct{})
				}
				idxMap[key] = set
			}
			set[bestIdx] = struct{}{}
			total += pixels
		}
	}
	if total == 0 {
		logDebug("pictureShift: no matching pairs")
		return 0, 0, idxs, false
	}

	best := [2]int{}
	second := [2]int{}
	bestCount := 0
	secondCount := 0
	for k, c := range counts {
		if c > bestCount {
			second = best
			secondCount = bestCount
			best = k
			bestCount = c
		} else if c > secondCount {
			second = k
			secondCount = c
		}
	}

	usedSecond := false
	if best == ([2]int{}) && secondCount >= int(secondBestShiftRatio*float64(total)) {
		best = second
		bestCount = secondCount
		usedSecond = true
	}

	//logDebug("pictureShift: counts=%v best=%v count=%d total=%d", counts, best, bestCount, total)
	if !usedSecond && bestCount*2 <= total {
		logDebug("pictureShift: no majority best=%d total=%d", bestCount, total)
		return 0, 0, idxs, false
	}
	if best[0]*best[0]+best[1]*best[1] > max*max {
		logDebug("pictureShift: motion too large (%d,%d)", best[0], best[1])
		return 0, 0, idxs, false
	}

	// Collect candidate background indices for the winning motion.
	// Filter out tiny sprites so we don't pin small pictures to
	// the screen background when the camera pans. Pixel weights above
	// maxWeight (100k) are clamped so a single large background doesn't
	// dominate motion detection.
	const minBackgroundPixels = 900
	for idx := range idxMap[best] {
		if idx >= 0 && idx < len(cur) {
			// Use cached counts when possible; fall back to a fresh query.
			pixels := 0
			if p, ok := pixelCache[cur[idx].PictID]; ok {
				pixels = p
			} else {
				pixels = nonTransparentPixels(cur[idx].PictID)
			}
			if pixels > maxWeight {
				pixels = maxWeight
			}
			if pixels >= minBackgroundPixels {
				idxs = append(idxs, idx)
			}
		}
	}
	return best[0], best[1], idxs, true
}

// drawStateEncrypted controls whether incoming draw state packets need to be
// decrypted using SimpleEncrypt before parsing. By default frames from the
// live server arrive unencrypted; set this flag to true only when handling
// SimpleEncrypt-obfuscated data.
var drawStateEncrypted = false

var drawStateScratch = struct {
	mu  sync.Mutex
	buf []byte
}{}

// handleDrawState decodes the packed draw state message. It decrypts the
// payload when drawStateEncrypted is true.
//
// When buildCache is false, the draw state is parsed without rebuilding the
// render cache. This is useful for fast-forward operations where intermediate
// frames do not need a fully prepared cache.
func handleDrawState(m []byte, buildCache bool) {
	frameCounter++

	if len(m) < 11 { // 2 byte tag + 9 bytes minimum
		return
	}

	var (
		data   []byte
		unlock func()
	)

	if drawStateEncrypted {
		need := len(m) - 2
		if need < 0 {
			need = 0
		}
		drawStateScratch.mu.Lock()
		if cap(drawStateScratch.buf) < need {
			drawStateScratch.buf = make([]byte, need)
		}
		drawStateScratch.buf = drawStateScratch.buf[:need]
		copy(drawStateScratch.buf, m[2:])
		data = drawStateScratch.buf
		simpleEncrypt(data)
		unlock = drawStateScratch.mu.Unlock
	} else {
		data = m[2:]
	}
	if unlock != nil {
		defer unlock()
	}

	ack, resend, err := parseDrawState(data, buildCache)
	if err != nil {
		logWarn("parseDrawState failed: %v", err)
		logDebugPacket(fmt.Sprintf("parseDrawState error: %v", err), data)
		if ackFrame > 0 {
			resendFrame = ackFrame + 1
		} else {
			resendFrame = 0
		}
		return
	}
	ackFrame = ack
	resendFrame = resend
}

// handleInvCmdFull resets and rebuilds the inventory from a full list command.
func handleInvCmdFull(data []byte) ([]byte, bool) {
	if len(data) < 1 {
		logError("inventory: full cmd missing count")
		return nil, false
	}
	itemCount := int(data[0])
	data = data[1:]
	bytesNeeded := (itemCount+7)>>3 + itemCount*2
	if len(data) < bytesNeeded {
		logError("inventory: full cmd truncated")
		return nil, false
	}
	equipBytes := (itemCount + 7) >> 3
	equips := data[:equipBytes]
	ids := make([]uint16, itemCount)
	for i := 0; i < itemCount; i++ {
		ids[i] = binary.BigEndian.Uint16(data[equipBytes+i*2:])
	}
	// Inventory equip flags are transmitted with the most significant bit of each
	// byte corresponding to the first item (big-endian bit order). Reverse the
	// bit numbering so index 0 maps to bit 7, index 1 to bit 6, and so on.
	eq := make([]bool, itemCount)
	for i := 0; i < itemCount; i++ {
		if equips[i/8]&(1<<uint(i%8)) != 0 {
			eq[i] = true
		}
	}
	setFullInventory(ids, eq)
	return data[bytesNeeded:], true
}

// handleInvCmdOther interprets add/delete/equip/name inventory commands.
func handleInvCmdOther(cmd int, data []byte) ([]byte, bool) {
	logDebug("inventory cmd=%v data=%v", cmd, data)

	base := cmd &^ kInvCmdIndex
	switch base {
	case 'd':
		logDebug("inventory: ignoring opcode 'd'")
		return data, true
	}
	if len(data) < 2 {
		logError("inventory: cmd %x missing id", cmd)
		return nil, false
	}
	id := binary.BigEndian.Uint16(data[:2])
	data = data[2:]
	idx := -1
	if cmd&kInvCmdIndex != 0 {
		if len(data) < 1 {
			logError("inventory: cmd %x missing index", cmd)
			return nil, false
		}
		// Server sends 1-based index; convert to 0-based for local arrays.
		idx = int(data[0]) - 1
		data = data[1:]
	}
	var name string
	if base == kInvCmdAdd || base == kInvCmdAddEquip || base == kInvCmdName {
		nidx := bytes.IndexByte(data, 0)
		if nidx < 0 {
			logError("inventory: cmd %x missing name", cmd)
			return nil, false
		}
		raw := append([]byte(nil), data[:nidx]...)
		raw = stripBEPPTags(raw)
		name = strings.TrimSpace(decodeServerText(raw))
		data = data[nidx+1:]
	}
	switch base {
	case kInvCmdAdd:
		addInventoryItem(id, idx, name, false)
	case kInvCmdAddEquip:
		addInventoryItem(id, idx, name, true)
	case kInvCmdDelete:
		removeInventoryItem(id, idx)
	case kInvCmdEquip:
		equipInventoryItem(id, idx, true)
	case kInvCmdUnequip:
		equipInventoryItem(id, idx, false)
	case kInvCmdName:
		renameInventoryItem(id, idx, name)
	default:
		logError("inventory: unknown command %v", cmd)
	}
	return data, true
}

// parseInventory walks the inventory command stream and returns the remaining
// slice and success flag.
func parseInventory(data []byte) ([]byte, bool) {
	if len(data) == 0 {
		return data, true
	}
	cmd := int(data[0])
	data = data[1:]
	if cmd == kInvCmdNone {
		return data, true
	}

	cmdCount := 1
	if cmd == kInvCmdMultiple {
		if len(data) < 2 {
			logDebug("inventory: truncated multiple cmd cmdCount=%d cmd=%#x rem=% x", cmdCount, cmd, data)
			return nil, false
		}
		cmdCount = int(data[0])
		cmd = int(data[1])
		data = data[2:]
	}

	for i := 0; i < cmdCount; i++ {
		switch cmd {
		case kInvCmdFull:
			var ok bool
			before := data
			data, ok = handleInvCmdFull(data)
			if !ok {
				logDebug("inventory: cmd %#x failed at %d/%d rem=% x", cmd, i+1, cmdCount, before)
				return nil, false
			}
		case kInvCmdNone:
			// nothing
		case kInvCmdFull | kInvCmdIndex, kInvCmdNone | kInvCmdIndex:
			if len(data) < 1 {
				logDebug("inventory: cmd %#x truncated at %d/%d rem=% x", cmd, i+1, cmdCount, data)
				return nil, false
			}
			data = data[1:]
		default:
			var ok bool
			before := data
			data, ok = handleInvCmdOther(cmd, data)
			if !ok {
				logDebug("inventory: cmd %#x failed at %d/%d rem=% x", cmd, i+1, cmdCount, before)
				return nil, false
			}
		}
		if len(data) > 0 {
			cmd = int(data[0])
			data = data[1:]
		} else {
			cmd = kInvCmdNone
		}
	}
	// After processing known commands a single trailing opcode may remain.
	// Some captures include an undocumented 0x64 ('d') value.  Treat it as
	// padding and ignore any other unknown values while logging at debug
	// level to aid future reverse-engineering efforts.
	switch cmd {
	case kInvCmdNone:
	case kInvCmdNone | kInvCmdIndex:
		if len(data) < 1 {
			logDebug("inventory: trailing cmd %#x truncated cmdCount=%d rem=% x", cmd, cmdCount, data)
			return nil, false
		}
		data = data[1:]
	case kInvCmdLegacyPadding:
		// ignore legacy padding byte
	case 'd':
		// observed but undocumented opcode
		logDebug("inventory: ignoring opcode 'd'")
	default:
		logDebug("inventory: ignoring trailing cmd %d", cmd)
	}
	for len(data) > 0 && (data[0] == 0 || data[0] == kInvCmdLegacyPadding) {
		data = data[1:]
	}
	inventoryDirty = true
	return data, true
}

// parseDrawState decodes the draw state data. It returns an error when the
// packet appears malformed, indicating the parsing stage that failed.
//
// When buildCache is false, state is updated without rebuilding the render
// cache.
func parseDrawState(data []byte, buildCache bool) (int32, int32, error) {
	stage := "header"
	if len(data) < 9 {
		return 0, 0, errors.New(stage)
	}

	ackCmd := data[0]
	ack := int32(binary.BigEndian.Uint32(data[1:5]))
	resend := int32(binary.BigEndian.Uint32(data[5:9]))
	dropped := 0
	if movieMode {
		dropped = movieDropped
	} else {
		dropped = updateFrameCounters(ack)
	}
	extra := dropped
	if extra > 2 {
		extra = 2
	}
	p := 9

	scratch := acquireDrawParseScratch()
	descs := scratch.descriptors[:0]
	pics := scratch.pictures[:0]
	mobiles := scratch.mobiles[:0]
	bubbles := scratch.bubbles[:0]
	shiftIndices := scratch.pictureShiftIndices[:0]
	present := scratch.mobilePresent
	if present == nil {
		present = make(map[uint8]struct{})
		scratch.mobilePresent = present
	} else {
		clearMap(present)
	}
	defer func() {
		scratch.descriptors = descs[:0]
		scratch.pictures = pics[:0]
		scratch.mobiles = mobiles[:0]
		scratch.bubbles = bubbles[:0]
		scratch.pictureShiftIndices = shiftIndices[:0]
		clearMap(present)
		releaseDrawParseScratch(scratch)
	}()

	stage = "descriptor count"
	if len(data) <= p {
		return ack, resend, errors.New(stage)
	}
	descCount := int(data[p])
	p++
	if descCount > maxDescriptors {
		return ack, resend, errors.New(stage)
	}
	stage = "descriptor"
	if descCount > cap(descs) {
		descs = make([]frameDescriptor, 0, descCount)
	}
	for i := 0; i < descCount && p < len(data); i++ {
		if p+4 > len(data) {
			return ack, resend, errors.New(stage)
		}
		d := frameDescriptor{}
		d.Index = data[p]
		d.Type = data[p+1]
		d.PictID = binary.BigEndian.Uint16(data[p+2:])
		p += 4
		if idx := bytes.IndexByte(data[p:], 0); idx >= 0 {
			d.Name = utfFold(decodeServerText(data[p : p+idx]))
			p += idx + 1
			if d.Name == playerName {
				playerIndex = d.Index
			}
			if wasmPrivacyActive() {
				d.Name = ""
			}
		} else {
			return ack, resend, errors.New(stage)
		}
		if p >= len(data) {
			return ack, resend, errors.New(stage)
		}
		cnt := int(data[p])
		p++
		if p+cnt > len(data) {
			return ack, resend, errors.New(stage)
		}
		d.Colors = append([]byte(nil), data[p:p+cnt]...)
		p += cnt
		if clImages != nil {
			d.Plane = clImages.Plane(uint32(d.PictID))
		}
		// Skip NPCs entirely for player list scanning. Only update
		// appearance and queue info requests when not in movie mode to
		// avoid side effects during playback.
		if d.Type != kDescNPC && d.Name != "" {
			if !movieMode {
				updatePlayerAppearance(d.Name, d.PictID, d.Colors, false)
				// Opportunistically request full info for visible players.
				queueInfoRequest(d.Name)
			}
		}
		descs = append(descs, d)
	}

	stage = "stats"
	if len(data) < p+7 {
		return ack, resend, errors.New(stage)
	}
	hp := int(data[p])
	hpMax := int(data[p+1])
	sp := int(data[p+2])
	spMax := int(data[p+3])
	bal := int(data[p+4])
	balMax := int(data[p+5])
	lighting := data[p+6]
	gNight.SetFlags(uint(lighting))
	p += 7

	stage = "picture count"
	if len(data) <= p {
		return ack, resend, errors.New(stage)
	}
	pictCount := int(data[p])
	p++
	pictAgain := 0
	stage = "picture header"
	if pictCount == 255 {
		if len(data) < p+2 {
			return ack, resend, errors.New(stage)
		}
		pictAgain = int(data[p])
		pictCount = int(data[p+1])
		p += 2
	}
	stage = "picture count"
	if pictAgain+pictCount > maxPictures {
		return ack, resend, errors.New(stage)
	}

	stage = "pictures"
	totalPics := pictAgain + pictCount
	if totalPics > cap(pics) {
		pics = make([]framePicture, 0, totalPics)
	}
	pictureIDBits := 14
	pictureCoordBits := 11
	// Movies through v366 used the original 12/10/10 packed picture format.
	// Live frames and newer movies use the expanded 14/11/11 format.
	if movieMode && movieVersion <= 366 {
		pictureIDBits = 12
		pictureCoordBits = 10
	}
	br := bitReader{data: data[p:]}
	for i := 0; i < pictCount; i++ {
		idBits, ok := br.readBits(pictureIDBits)
		if !ok {
			return ack, resend, errors.New("truncated picture bit stream")
		}
		hBits, ok := br.readBits(pictureCoordBits)
		if !ok {
			return ack, resend, errors.New("truncated picture bit stream")
		}
		vBits, ok := br.readBits(pictureCoordBits)
		if !ok {
			return ack, resend, errors.New("truncated picture bit stream")
		}
		id := uint16(idBits)
		h := signExtend(hBits, pictureCoordBits)
		v := signExtend(vBits, pictureCoordBits)
		plane := 0
		if clImages != nil {
			plane = clImages.Plane(uint32(id))
		}
		pics = append(pics, framePicture{PictID: id, H: h, V: v, Plane: plane})
	}
	p += br.bitPos / 8
	if br.bitPos%8 != 0 {
		p++
	}

	stage = "mobile count"
	if len(data) <= p {
		return ack, resend, errors.New(stage)
	}
	mobileCount := int(data[p])
	p++
	if mobileCount > maxMobiles {
		return ack, resend, errors.New(stage)
	}
	stage = "mobiles"
	if mobileCount > cap(mobiles) {
		mobiles = make([]frameMobile, 0, mobileCount)
	}
	for i := 0; i < mobileCount && p+7 <= len(data); i++ {
		m := frameMobile{}
		m.Index = data[p]
		m.State = data[p+1]
		m.H = int16(binary.BigEndian.Uint16(data[p+2:]))
		m.V = int16(binary.BigEndian.Uint16(data[p+4:]))
		m.Colors = data[p+6]
		p += 7
		mobiles = append(mobiles, m)
	}
	if len(mobiles) != mobileCount {
		return ack, resend, errors.New(stage)
	}

	stage = "state size"
	if len(data) < p+2 {
		return ack, resend, errors.New(stage)
	}
	stateLen := int(binary.BigEndian.Uint16(data[p:]))
	p += 2
	if len(data) < p+stateLen {
		return ack, resend, errors.New(stage)
	}
	stateData := data[p : p+stateLen]

	stateMu.Lock()
	state.logicalFrame = frameCounter
	state.ackCmd = ackCmd
	state.dropped = extra
	state.lightingFlags = lighting
	state.prevHP = state.hp
	state.prevHPMax = state.hpMax
	state.prevSP = state.sp
	state.prevSPMax = state.spMax
	state.prevBalance = state.balance
	state.prevBalanceMax = state.balanceMax
	state.hp = hp
	state.hpMax = hpMax
	state.sp = sp
	state.spMax = spMax
	state.balance = bal
	state.balanceMax = balMax
	changed := false
	if mobileFrameBlendingEnabled() && !seekingMov {
		if len(descs) > 0 {
			changed = true
		}
		if len(mobiles) != len(state.mobiles) {
			changed = true
		} else {
			for _, m := range mobiles {
				if pm, ok := state.mobiles[m.Index]; !ok || pm.State != m.State {
					changed = true
					break
				}
			}
		}
		if changed {
			if state.prevDescs == nil {
				state.prevDescs = make(map[uint8]frameDescriptor, len(state.descriptors))
			} else {
				clearMap(state.prevDescs)
			}
			for idx, d := range state.descriptors {
				state.prevDescs[idx] = d
			}
		}
	}
	// retain previously drawn pictures when the packet specifies pictAgain
	prevPics := state.pictures
	again := pictAgain
	if again > len(prevPics) {
		again = len(prevPics)
	}
	newPicCount := again + pictCount
	newPics := state.prevPictures
	if cap(newPics) < newPicCount {
		newPics = make([]framePicture, newPicCount)
	} else {
		newPics = newPics[:newPicCount]
	}
	copy(newPics, prevPics[:again])
	copy(newPics[again:], pics)
	for i := 0; i < again && i < len(newPics); i++ {
		newPics[i].Again = true
	}
	for i := again; i < len(newPics); i++ {
		newPics[i].Again = false
	}
	maxInterp := maxInterpPixels * (extra + 1)
	dx, dy, bgIdxs, ok := pictureShiftInto(prevPics, newPics, maxInterp, shiftIndices)
	shiftIndices = bgIdxs
	if gs.MotionSmoothing && !seekingMov {
		if gs.smoothMoving {
			logDebug("interp pictures again=%d prev=%d cur=%d shift=(%d,%d) ok=%t", again, len(prevPics), len(newPics), dx, dy, ok)
			if !ok {
				logDebug("prev pics: %v", picturesSummary(prevPics))
				logDebug("new  pics: %v", picturesSummary(newPics))
			}
		}
		if ok {
			state.picShiftX = dx
			state.picShiftY = dy
		} else {
			state.picShiftX = 0
			state.picShiftY = 0
		}
	} else {
		state.picShiftX = 0
		state.picShiftY = 0
	}
	if !ok {
		prevPics = nil
		again = 0
		if cap(newPics) < len(pics) {
			newPics = make([]framePicture, len(pics))
		} else {
			newPics = newPics[:len(pics)]
		}
		copy(newPics, pics)
		clearMap(state.prevDescs)
		clearMap(state.prevMobiles)
		state.prevPictures = nil
		state.prevTime = time.Time{}
		state.curTime = time.Time{}
		logDebug("pictureShift failed; bypassing interpolation")
	}
	if state.descriptors == nil {
		state.descriptors = make(map[uint8]frameDescriptor)
	}
	for _, d := range descs {
		state.descriptors[d.Index] = d
	}
	for i := range prevPics {
		prevPics[i].Owned = false
	}
	positionScratch := matchPicturePositions(prevPics, newPics, state.picShiftX, state.picShiftY, maxInterp, again)
	positionMatches := positionScratch.matches
	for i := range newPics {
		cloudMotion := pictureCloudMotionEnabled(newPics[i])
		if _, skip := skipPictShift[newPics[i].PictID]; skip {
			newPics[i].PrevH = newPics[i].H
			newPics[i].PrevV = newPics[i].V
		} else if cloudMotion {
			// Do not draw a newly appeared cloud at the prior camera position.
			// A matching old position is assigned below.
			newPics[i].PrevH = newPics[i].H
			newPics[i].PrevV = newPics[i].V
		} else {
			newPics[i].PrevH = int16(int(newPics[i].H) - state.picShiftX)
			newPics[i].PrevV = int16(int(newPics[i].V) - state.picShiftY)
		}
		moving := true
		var owner *framePicture
		if i < again {
			moving = false
			owner = &prevPics[i]
		} else if j := positionMatches[i]; j >= 0 {
			pp := &prevPics[j]
			if int(pp.H)+state.picShiftX == int(newPics[i].H) &&
				int(pp.V)+state.picShiftY == int(newPics[i].V) {
				moving = false
				owner = pp
			}
		}
		if cloudMotion && owner != nil {
			// Continue a cloud from its fresh previous packet position, including
			// camera motion. Keep it marked moving so drawPicture uses PrevH/V.
			moving = true
			newPics[i].PrevH = owner.H
			newPics[i].PrevV = owner.V
		}
		if moving && pictureMotionBlockedAtEdge(newPics[i], cloudMotion) {
			moving = false
		}
		if moving && pictureMotionInterpolationEnabled(newPics[i]) {
			if j := positionMatches[i]; j >= 0 {
				previous := &prevPics[j]
				newPics[i].PrevH = previous.H
				newPics[i].PrevV = previous.V
				previous.Owned = true
			}
		} else if owner != nil {
			owner.Owned = true
		}
		newPics[i].Moving = moving
		newPics[i].Background = false
	}
	releasePicturePositionScratch(positionScratch)
	for _, idx := range bgIdxs {
		if idx >= 0 && idx < len(newPics) {
			newPics[idx].Moving = false
			newPics[idx].Background = true
		}
	}

	// Carry over previous-frame ground sprites that are missing this frame.
	// Advance them by the detected picture shift and keep them one extra
	// update to prevent edge flicker during camera motion.
	//
	// Previously this only triggered when a sprite was classified as
	// "on edge" (>=70% off-screen). That was too strict and could miss
	// legitimate ground tiles. Now we carry if the previous sprite was
	// marked Background and still visible, or fall back to the stricter
	// edge test for unclassified cases.
	if (state.picShiftX != 0 || state.picShiftY != 0) && len(prevPics) > 0 {
		for _, pp := range prevPics {
			if pp.Owned {
				continue // already matched/present
			}
			// Only carry ground/static sprites (not marked moving)
			if pp.Moving {
				continue
			}
			// Don't include these
			if pp.Again {
				continue
			}
			// Carry when the previous sprite was background and still visible,
			// otherwise fall back to stricter edge classification.
			if !pictureVisible(pp) {
				continue
			}
			if !pp.Background {
				if !pictureOnEdge(pp) {
					continue
				}
			}
			// Skip very large images to avoid overdraw surprises.
			if clImages != nil {
				if w, h := clImages.Size(uint32(pp.PictID)); w > maxPersistImageSize || h > maxPersistImageSize {
					continue
				}
			}
			oldH, oldV := pp.H, pp.V
			// Advance by detected picture shift for this frame.
			pp.H = int16(int(pp.H) + state.picShiftX)
			pp.V = int16(int(pp.V) + state.picShiftY)
			pp.PrevH = oldH
			pp.PrevV = oldV
			pp.Moving = false
			pp.Background = true
			pp.Again = true
			newPics = append(newPics, pp)
		}
	}

	// Save previous pictures for pinning/interpolation decisions
	state.prevPictures = prevPics
	state.pictures = newPics

	needPrev := gs.MotionSmoothing && !seekingMov && ok
	needAnimUpdate := gs.MotionSmoothing && ok && !seekingMov
	if needAnimUpdate {
		// Use the latest measured server interval; do not reuse the previous
		// cur-prev duration as that can get stuck until a hard reset (e.g.,
		// background change) occurs.
		frameMu.Lock()
		interval := frameInterval
		frameMu.Unlock()
		if interval <= 0 {
			interval = time.Second / 5
		}
		interval *= time.Duration(extra + 1)
		state.prevTime = time.Now()
		state.curTime = state.prevTime.Add(interval)
	}

	// Carry over previous-frame mobiles that disappear at the edge to avoid
	// premature culling from interpolation.
	if len(state.mobiles) > 0 {
		clearMap(present)
		for _, m := range mobiles {
			present[m.Index] = struct{}{}
		}
		for idx, pm := range state.mobiles {
			if pm.Persist {
				continue
			}
			if _, ok := present[idx]; ok {
				continue
			}
			if d, ok := state.descriptors[idx]; ok && mobileOnEdge(pm, d) {
				pm.H = int16(int(pm.H) + state.picShiftX)
				pm.V = int16(int(pm.V) + state.picShiftY)
				pm.Persist = true
				mobiles = append(mobiles, pm)
			}
		}
	}
	markPlayersOnScreen(mobiles, state.descriptors, time.Now())

	previousMobiles := state.mobiles
	nextMobiles := state.mobileScratch
	if nextMobiles == nil {
		nextMobiles = make(map[uint8]frameMobile, len(mobiles))
	} else {
		clearMap(nextMobiles)
	}
	for _, m := range mobiles {
		if d, ok := state.descriptors[m.Index]; ok && d.Name != "" && !(gs.HideSelfNameTag && strings.EqualFold(d.Name, playerName)) {
			style := styleRegular
			dead := m.State == poseDead
			playersMu.RLock()
			if p, ok := players[d.Name]; ok {
				if p.Sharing && p.Sharee {
					style = styleBoldItalic
				} else if p.Sharing {
					style = styleBold
				} else if p.Sharee {
					style = styleItalic
				}
			}
			playersMu.RUnlock()
			opacity := uint8(gs.NameBgOpacity*255 + 0.5)
			key := makeNameTagKey(d.Name, m.Colors, d.Type, opacity, style, dead)
			if !reuseCachedNameTag(&m, previousMobiles, key) {
				img, iw, ih := sharedNameTagImage(key)
				m.nameTag = img
				m.nameTagW = iw
				m.nameTagH = ih
				m.nameTagKey = key
			}
		}
		nextMobiles[m.Index] = m
	}
	state.mobiles = nextMobiles
	if needPrev {
		state.mobileScratch = state.prevMobiles
		state.prevMobiles = previousMobiles
	} else {
		state.mobileScratch = previousMobiles
	}
	if gs.FadeObscuringPictures {
		cachePictureObscuring(state.pictures, mobiles, state.descriptors, state.prevMobiles, state.logicalFrame)
	}
	// Populate prevMobiles only when pictureShift succeeds so interpolation of
	// mobiles and pinned effects is skipped on failure.
	// Prepare render caches now that state has been updated when requested.
	if buildCache {
		prepareRenderCacheLocked()
	}
	//ack := state.ackCmd
	//light := state.lightingFlags
	stateMu.Unlock()

	/*
	   logDebug("draw state cmd=%d ack=%d resend=%d light=%#x desc=%d pict=%d again=%d mobile=%d state=%d",
	           ackCmd, ack, resend, light, len(descs), len(pics), pictAgain, len(mobiles), len(stateData))
	*/

	stage = "info strings"
	// Server sends a zero-terminated info-text blob which may contain
	// multiple CR-separated lines. Consume the first C string, then
	// defensively skip any additional stray C strings until what looks
	// like a valid bubble count (<= maxBubbles) is encountered.
	if len(stateData) == 0 {
		return ack, resend, errors.New(stage)
	}
	if idx := bytes.IndexByte(stateData, 0); idx >= 0 {
		if idx > 0 {
			handleInfoText(stateData[:idx])
		}
		stateData = stateData[idx+1:]
	} else {
		return ack, resend, errors.New(stage)
	}
	for len(stateData) > 0 {
		if int(stateData[0]) <= maxBubbles {
			break
		}
		// Treat preceding bytes as another info text C string.
		if idx := bytes.IndexByte(stateData, 0); idx >= 0 {
			if idx > 0 {
				handleInfoText(stateData[:idx])
			}
			stateData = stateData[idx+1:]
			continue
		}
		// No terminating zero found; give up.
		return ack, resend, errors.New(stage)
	}

	stage = "bubble count"
	if len(stateData) == 0 {
		return ack, resend, errors.New(stage)
	}
	bubbleCount := int(stateData[0])
	stateData = stateData[1:]
	if bubbleCount > maxBubbles {
		return ack, resend, errors.New(stage)
	}
	stage = "bubble"
	for i := 0; i < bubbleCount && len(stateData) > 0; i++ {
		off := len(data) - len(stateData)
		if len(stateData) < 2 {
			return ack, resend, fmt.Errorf("bubble=%d off=%d len=%d", i, off, len(stateData))
		}
		idx := stateData[0]
		typ := int(stateData[1])
		p := 2
		if typ&kBubbleNotCommon != 0 {
			if len(stateData) < p+1 {
				return ack, resend, fmt.Errorf("bubble=%d off=%d len=%d", i, off, len(stateData))
			}
			p++
		}
		var h, v int16
		if typ&kBubbleFar != 0 {
			if len(stateData) < p+4 {
				return ack, resend, fmt.Errorf("bubble=%d off=%d len=%d", i, off, len(stateData))
			}
			h = int16(binary.BigEndian.Uint16(stateData[p:]))
			v = int16(binary.BigEndian.Uint16(stateData[p+2:]))
			p += 4
		}
		if len(stateData) <= p {
			return ack, resend, fmt.Errorf("bubble=%d off=%d len=%d", i, off, len(stateData))
		}
		end := bytes.IndexByte(stateData[p:], 0)
		if end < 0 {
			return ack, resend, fmt.Errorf("bubble=%d off=%d len=%d", i, off, len(stateData))
		}
		bubbleData := stateData[:p+end+1]
		if verb, txt, bubbleName, lang, code, bubbleType, target := decodeBubble(bubbleData); txt != "" || code != kBubbleCodeKnown {
			name := bubbleName
			if target == thinkNone {
				if bubbleName == ThinkUnknownName {
					name = "Someone"
				} else {
					stateMu.Lock()
					if d, ok := state.descriptors[idx]; ok {
						if bubbleName != "" {
							if d.Name != "" {
								name = d.Name
							} else {
								d.Name = bubbleName
								name = bubbleName
							}
						} else {
							name = d.Name
						}
					}
					stateMu.Unlock()
				}
			} else if bubbleName == ThinkUnknownName {
				name = "Someone"
			}
			if verb == "thinks" && idx == playerIndex && bubbleName != "" {
				stateMu.Lock()
				for i, d := range state.descriptors {
					if d.Name == bubbleName {
						idx = i
						break
					}
				}
				stateMu.Unlock()
			}
			filterName := name
			if bubbleName == ThinkUnknownName {
				stateMu.Lock()
				if d, ok := state.descriptors[idx]; ok && d.Name != "" {
					filterName = d.Name
				}
				stateMu.Unlock()
			}
			skipRender := false
			if filterName != "" {
				playersMu.RLock()
				if p, ok := players[filterName]; ok && (p.Blocked || p.Ignored) {
					skipRender = true
				}
				playersMu.RUnlock()
			}
			showBubble := gs.SpeechBubbles && txt != "" && !blockBubbles && verb != "thinks"
			if showBubble && !skipRender {
				typeOK := true
				switch bubbleType {
				case kBubbleNormal:
					typeOK = gs.BubbleNormal
				case kBubbleWhisper:
					typeOK = gs.BubbleWhisper
				case kBubbleYell:
					typeOK = gs.BubbleYell
				case kBubbleThought:
					typeOK = gs.BubbleThought
				case kBubbleRealAction:
					typeOK = gs.BubbleRealAction
				case kBubbleMonster:
					typeOK = gs.BubbleMonster
				case kBubblePlayerAction:
					typeOK = gs.BubblePlayerAction
				case kBubblePonder:
					typeOK = gs.BubblePonder
				case kBubbleNarrate:
					typeOK = gs.BubbleNarrate
				}
				originOK := true
				switch {
				case idx == playerIndex:
					originOK = gs.BubbleSelf
				case bubbleType == kBubbleMonster:
					originOK = gs.BubbleMonsters
				case bubbleType == kBubbleNarrate:
					originOK = gs.BubbleNarration
				default:
					originOK = gs.BubbleOtherPlayers
				}
				showBubble = typeOK && originOK
			}
			if showBubble && !skipRender {
				words := len(strings.Fields(txt))
				lifeSeconds := gs.BubbleBaseLife + float64(words)*gs.BubbleLifePerWord
				life := int(lifeSeconds * float64(1000/framems))
				if life < 1 {
					life = 1
				}
				b := bubble{Index: idx, Text: txt, Type: typ, CreatedFrame: frameCounter, LifeFrames: life}
				switch bubbleType {
				case kBubbleRealAction, kBubblePlayerAction, kBubbleNarrate:
					b.NoArrow = true
				}
				if typ&kBubbleFar != 0 {
					b.H, b.V = h, v
					b.Far = true
				}
				bubbles = append(bubbles, b)
			}
			var msg string
			switch {
			case typ&kBubbleTypeMask == kBubbleNarrate:
				if name != "" {
					msg = fmt.Sprintf("(%v): %v", name, txt)
				} else {
					msg = txt
				}
			case verb == bubbleVerbVerbatim:
				msg = txt
			case verb == bubbleVerbParentheses:
				msg = fmt.Sprintf("(%v)", txt)
			default:
				if name != "" {
					if verb == "thinks" {
						switch target {
						case thinkToYou:
							msg = fmt.Sprintf("%v thinks to you, %v", bubbleName, txt)
						case thinkToClan:
							msg = fmt.Sprintf("%v thinks to your clan, %v", bubbleName, txt)
						case thinkToGroup:
							msg = fmt.Sprintf("%v thinks to a group, %v", bubbleName, txt)
						default:
							msg = fmt.Sprintf("%v thinks, %v", bubbleName, txt)
						}
						if !skipRender {
							showThinkMessage(msg)
						}
					} else if typ&kBubbleNotCommon != 0 {
						langWord := lang
						lw := strings.ToLower(langWord)
						if langWord == "" || strings.HasPrefix(lw, "unknown") {
							langWord = "an unknown language"
						}
						if code == kBubbleCodeKnown {
							if langWord == "Common" {
								msg = fmt.Sprintf("%v %v %v", name, verb, txt)
							} else {
								msg = fmt.Sprintf("%v %v in %v, %v", name, verb, langWord, txt)
							}
						} else if typ&kBubbleTypeMask == kBubbleYell {
							switch code {
							case kBubbleUnknownShort:
								msg = fmt.Sprintf("%v %v, %v", name, verb, txt)
							case kBubbleUnknownMedium:
								msg = fmt.Sprintf("%v %v in %v, %v", name, verb, langWord, txt)
							case kBubbleUnknownLong:
								msg = fmt.Sprintf("%v %v in %v, %v", name, verb, langWord, txt)
							default:
								msg = fmt.Sprintf("%v %v in %v, %v", name, verb, langWord, txt)
							}
						} else {
							var unknown string
							switch code {
							case kBubbleUnknownShort:
								unknown = "something short"
							case kBubbleUnknownMedium:
								unknown = "something medium"
							case kBubbleUnknownLong:
								unknown = "something long"
							default:
								unknown = "something"
							}
							msg = fmt.Sprintf("%v %v %v in %v", name, verb, unknown, langWord)
						}
					} else {
						msg = fmt.Sprintf("%v %v, %v", name, verb, txt)
					}
				} else {
					if txt != "" {
						msg = "* " + txt
					}
				}
			}
			messageType := messageTextTypeForBubble(bubbleType)
			if gs.MessagesToConsole || !isChatBubble(bubbleType) {
				serverConsoleMessageTyped(msg, messageType)
			} else {
				chatMessageTyped(msg, messageType)
			}
		}
		stateData = stateData[p+end+1:]
	}

	if len(bubbles) > 0 {
		stateMu.Lock()
		state.bubbles = append(state.bubbles, bubbles...)
		stateMu.Unlock()
	}

	stage = "sound count"
	if len(stateData) < 1 {
		return ack, resend, errors.New(stage)
	}
	soundCount := int(stateData[0])
	stateData = stateData[1:]
	stage = "sounds"
	if len(stateData) < soundCount*2 {
		return ack, resend, errors.New(stage)
	}
	var newSounds []uint16
	var numNewSounds int

	for i := 0; i < soundCount && i < maxSounds; i++ {
		id := binary.BigEndian.Uint16(stateData[:2])
		stateData = stateData[2:]

		if gs.ThrottleSounds {
			var found bool
			for _, item := range prevSounds {
				if item == id {
					found = true
					break
				}
			}
			if found {
				continue
			}

			for _, item := range prev2Sounds {
				if item == id {
					found = true
					break
				}
			}
			if found {
				continue
			}
		}

		//Prevent duplicate sounds
		if numNewSounds > 0 {
			for _, item := range newSounds {
				if item != id {
					newSounds = append(newSounds, id)
					numNewSounds++
					break
				}
			}
		} else {
			newSounds = []uint16{id}
		}
	}
	playSound(newSounds)
	prev2Sounds = prevSounds
	prevSounds = newSounds

	stage = "inventory"
	rest, ok := parseInventory(stateData)
	if !ok || len(rest) > 0 {
		return ack, resend, errors.New(stage)
	}

	return ack, resend, nil
}

var prevSounds []uint16
var prev2Sounds []uint16
