package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image/color"
	"math"
	"sort"
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
	Text    string
	Colors  uint8
	Opacity uint8
	FontGen uint32
	Style   uint8
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
const maxPersistImageSize = 256
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

func sortPictures(pics []framePicture) {
	sort.Slice(pics, func(i, j int) bool {
		if pics[i].Plane != pics[j].Plane {
			return pics[i].Plane < pics[j].Plane
		}
		if pics[i].V == pics[j].V {
			return pics[i].H < pics[j].H
		}
		return pics[i].V < pics[j].V
	})
}

func sortMobiles(mobs []frameMobile) {
	sort.Slice(mobs, func(i, j int) bool {
		if mobs[i].V == mobs[j].V {
			return mobs[i].H < mobs[j].H
		}
		return mobs[i].V < mobs[j].V
	})
}

func sortMobilesNameTags(mobs []frameMobile) {
	sort.Slice(mobs, func(i, j int) bool {
		if mobs[i].H == mobs[j].H {
			return mobs[i].V < mobs[j].V
		}
		return mobs[i].H > mobs[j].H
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

var (
	pixelCountMu    sync.RWMutex
	pixelCountCache = make(map[uint16]int)
)

// nonTransparentPixels returns the number of non-transparent pixels for the
// given picture ID. When CL_Images data is available it avoids GPU readbacks
// by using the decoded pixel data directly and falls back to a generic path
// otherwise.
func nonTransparentPixels(id uint16) int {
	if !gs.NoCaching {
		pixelCountMu.RLock()
		if c, ok := pixelCountCache[id]; ok {
			pixelCountMu.RUnlock()
			return c
		}
		pixelCountMu.RUnlock()
	}

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

	if !gs.NoCaching {
		pixelCountMu.Lock()
		pixelCountCache[id] = count
		pixelCountMu.Unlock()
	}
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
	if offArea*10 < totalArea*7 {
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
// returns true only when one or two edges are crossed and at least 70% of the
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
	if offArea*10 < totalArea*7 {
		return false
	}
	return true
}

// buildNameTagImage creates a cached image for a mobile name tag using the
// current font and settings. Returns the image and its width/height in pixels.
// The frame color defaults to the name color unless frameClr overrides it.
func buildNameTagImage(name string, colorCode uint8, opacity uint8, style uint8, frameClr color.RGBA) (*ebiten.Image, int, int) {
	if name == "" {
		return nil, 0, 0
	}
	textClr, bgClr, defFrame := mobileNameColors(colorCode)
	if frameClr.A == 0 {
		frameClr = defFrame
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
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(iw+5), float64(ih))
	op.ColorScale.ScaleWithColor(bgClr)
	op.ColorScale.ScaleAlpha(float32(opacity) / 255)
	img.DrawImage(whiteImage, op)
	// Border
	vector.StrokeRect(img, 1, 1, float32(iw+4), float32(ih-1), 1, frameClr, false)
	// Text
	opTxt := &text.DrawOptions{}
	opTxt.GeoM.Translate(2, 2)
	opTxt.ColorScale.ScaleWithColor(textClr)
	text.Draw(img, name, face, opTxt)
	return img, iw + 5, ih
}

// pictureShift returns the (dx, dy) movement that most on-screen pictures agree on
// between two consecutive frames. Pictures are matched by PictID (duplicates
// included) and weighted by their non-transparent pixel counts. The returned
// slice contains the indexes within the current frame that contributed to the
// winning movement. The boolean result is false when no majority offset is
// found. max sets the maximum allowed pixel delta for the returned shift.
func pictureShift(prev, cur []framePicture, max int) (int, int, []int, bool) {
	if len(prev) == 0 || len(cur) == 0 {
		//logDebug("pictureShift: no data prev=%d cur=%d", len(prev), len(cur))
		return 0, 0, nil, false
	}

	const maxWeight = 100000

	counts := make(map[[2]int]int)
	idxMap := make(map[[2]int]map[int]struct{})
	total := 0
	maxInt := int(^uint(0) >> 1)

	// Build a map from PictID to indexes in the current frame to avoid
	// repeatedly scanning the entire list for matches.
	curIdx := make(map[uint16][]int, len(cur))
	for i, c := range cur {
		if _, skip := skipPictShift[c.PictID]; skip {
			continue
		}
		curIdx[c.PictID] = append(curIdx[c.PictID], i)
	}

	// Cache pixel counts locally so that each PictID is computed at most once
	// per pictureShift invocation.
	pixelCache := make(map[uint16]int)

	for _, p := range prev {
		if _, skip := skipPictShift[p.PictID]; skip {
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
			if idxMap[key] == nil {
				idxMap[key] = make(map[int]struct{})
			}
			idxMap[key][bestIdx] = struct{}{}
			total += pixels
		}
	}
	if total == 0 {
		logDebug("pictureShift: no matching pairs")
		return 0, 0, nil, false
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
		return 0, 0, nil, false
	}
	if best[0]*best[0]+best[1]*best[1] > max*max {
		logDebug("pictureShift: motion too large (%d,%d)", best[0], best[1])
		return 0, 0, nil, false
	}

	// Collect candidate background indices for the winning motion.
	// Filter out tiny sprites so we don't pin small pictures to
	// the screen background when the camera pans. Pixel weights above
	// maxWeight (100k) are clamped so a single large background doesn't
	// dominate motion detection.
	const minBackgroundPixels = 900
	idxs := make([]int, 0, len(idxMap[best]))
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

	data := append([]byte(nil), m[2:]...)
	if drawStateEncrypted {
		simpleEncrypt(data)
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
		name = strings.TrimSpace(decodeMacRoman(raw))
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
	descs := make([]frameDescriptor, 0, descCount)
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
			d.Name = utfFold(decodeMacRoman(data[p : p+idx]))
			p += idx + 1
			if d.Name == playerName {
				playerIndex = d.Index
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
	pics := make([]framePicture, 0, pictAgain+pictCount)
	br := bitReader{data: data[p:]}
	for i := 0; i < pictCount; i++ {
		idBits, ok := br.readBits(14)
		if !ok {
			return ack, resend, errors.New("truncated picture bit stream")
		}
		hBits, ok := br.readBits(11)
		if !ok {
			return ack, resend, errors.New("truncated picture bit stream")
		}
		vBits, ok := br.readBits(11)
		if !ok {
			return ack, resend, errors.New("truncated picture bit stream")
		}
		id := uint16(idBits)
		h := signExtend(hBits, 11)
		v := signExtend(vBits, 11)
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
	mobiles := make([]frameMobile, 0, mobileCount)
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
	if gs.BlendMobiles && !seekingMov {
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
				state.prevDescs = make(map[uint8]frameDescriptor)
			}
			state.prevDescs = make(map[uint8]frameDescriptor, len(state.descriptors))
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
	newPics := make([]framePicture, again+pictCount)
	copy(newPics, prevPics[:again])
	copy(newPics[again:], pics)
	for i := 0; i < again && i < len(newPics); i++ {
		newPics[i].Again = true
	}
	for i := again; i < len(newPics); i++ {
		newPics[i].Again = false
	}
	maxInterp := maxInterpPixels * (extra + 1)
	dx, dy, bgIdxs, ok := pictureShift(prevPics, newPics, maxInterp)
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
		newPics = append([]framePicture(nil), pics...)
		state.prevDescs = nil
		state.prevMobiles = nil
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
	for i := range newPics {
		if _, skip := skipPictShift[newPics[i].PictID]; skip {
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
		} else {
			for j := range prevPics {
				pp := &prevPics[j]
				if pp.Owned {
					continue
				}
				if pp.PictID == newPics[i].PictID &&
					int(pp.H)+state.picShiftX == int(newPics[i].H) &&
					int(pp.V)+state.picShiftY == int(newPics[i].V) {
					moving = false
					owner = pp
					break
				}
			}
		}
		if moving && pictureOnEdge(newPics[i]) {
			moving = false
		}
		if moving && gs.smoothMoving {
			bestDist := maxInterp*maxInterp + 1
			var best *framePicture
			for j := range prevPics {
				pp := &prevPics[j]
				if pp.Owned || pp.PictID != newPics[i].PictID {
					continue
				}
				dh := int(newPics[i].H) - int(pp.H) - state.picShiftX
				dv := int(newPics[i].V) - int(pp.V) - state.picShiftY
				dist := dh*dh + dv*dv
				if dist < bestDist {
					bestDist = dist
					best = pp
				}
			}
			if best != nil && bestDist <= maxInterp*maxInterp {
				newPics[i].PrevH = best.H
				newPics[i].PrevV = best.V
				best.Owned = true
			}
		} else if owner != nil {
			owner.Owned = true
		}
		newPics[i].Moving = moving
		newPics[i].Background = false
	}
	for _, idx := range bgIdxs {
		if idx >= 0 && idx < len(newPics) {
			newPics[idx].Moving = false
			newPics[idx].Background = true
		}
	}

	// Carry over previous-frame ground sprites that are missing this frame.
	// Advance them by the detected picture shift and keep them while visible
	// to prevent flashes of black at the viewport edges during camera motion.
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
			// Only carry if the previous position's bounding box was on-screen.
			oldH, oldV := pp.H, pp.V
			if !pictureOnEdge(pp) {
				continue
			}
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
	state.prevPictures = append([]framePicture(nil), prevPics...)
	state.pictures = newPics

	needPrev := (gs.MotionSmoothing || gs.BlendMobiles) && !seekingMov && ok
	if needPrev {
		if state.prevMobiles == nil {
			state.prevMobiles = make(map[uint8]frameMobile)
		}
		state.prevMobiles = make(map[uint8]frameMobile, len(state.mobiles))
		for idx, m := range state.mobiles {
			state.prevMobiles[idx] = m
		}
	}
	needAnimUpdate := (gs.MotionSmoothing || (gs.BlendMobiles && changed)) && ok && !seekingMov
	if needAnimUpdate {
		frameMu.Lock()
		interval := frameInterval
		frameMu.Unlock()
		if !state.prevTime.IsZero() && !state.curTime.IsZero() {
			if d := state.curTime.Sub(state.prevTime); d > 0 {
				interval = d
			}
		}
		if interval <= 0 {
			interval = time.Second / 5
		}
		interval *= time.Duration(extra + 1)
		//logDebug("interp mobiles interval=%v extra=%d", interval, extra)
		state.prevTime = time.Now()
		state.curTime = state.prevTime.Add(interval)
	}

	// Carry over previous-frame mobiles that disappear at the edge to avoid
	// premature culling from interpolation.
	if len(state.mobiles) > 0 {
		present := make(map[uint8]struct{}, len(mobiles))
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

	if state.mobiles == nil {
		state.mobiles = make(map[uint8]frameMobile)
	} else {
		// clear map while keeping allocation
		for k := range state.mobiles {
			delete(state.mobiles, k)
		}
	}
	for _, m := range mobiles {
		if d, ok := state.descriptors[m.Index]; ok && d.Name != "" {
			style := styleRegular
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
			key := nameTagKey{
				Text:    d.Name,
				Colors:  m.Colors,
				Opacity: uint8(gs.NameBgOpacity*255 + 0.5),
				FontGen: fontGen,
				Style:   style,
			}
			if prev, ok := state.mobiles[m.Index]; ok && prev.nameTag != nil && prev.nameTagKey == key {
				m.nameTag = prev.nameTag
				m.nameTagW = prev.nameTagW
				m.nameTagH = prev.nameTagH
				m.nameTagKey = prev.nameTagKey
			} else {
				frame := color.RGBA{}
				if gs.NameTagLabelColors {
					playersMu.RLock()
					if p, ok := players[d.Name]; ok && p.FriendLabel > 0 && p.FriendLabel <= len(labelColors) {
						lc := labelColors[p.FriendLabel-1]
						frame = color.RGBA{lc.R, lc.G, lc.B, 0xff}
					}
					playersMu.RUnlock()
				}
				img, iw, ih := buildNameTagImage(d.Name, m.Colors, uint8(gs.NameBgOpacity*255+0.5), style, frame)
				m.nameTag = img
				m.nameTagW = iw
				m.nameTagH = ih
				m.nameTagKey = key
			}
		}
		state.mobiles[m.Index] = m
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
			skipRender := false
			if name != "" {
				playersMu.RLock()
				if p, ok := players[name]; ok && (p.Blocked || p.Ignored) {
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
				stateMu.Lock()
				state.bubbles = append(state.bubbles, b)
				stateMu.Unlock()
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
			if gs.MessagesToConsole || !isChatBubble(bubbleType) {
				consoleMessage(msg)
			} else {
				chatMessage(msg)
			}
		}
		stateData = stateData[p+end+1:]
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

		if gs.throttleSounds {
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
