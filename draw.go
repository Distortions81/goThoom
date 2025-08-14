package main

import (
	"bytes"
	"container/list"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// frameDescriptor describes an on-screen descriptor.
type frameDescriptor struct {
	Index  uint8
	Type   uint8
	PictID uint16
	Name   string
	Colors []byte
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
    Ghost        bool
}

type frameMobile struct {
	Index  uint8
	State  uint8
	H, V   int16
	Colors uint8
}

const poseDead = 32
const maxInterpPixels = 64
const maxMobileInterpPixels = 64

// sanity limits for parsed counts to avoid excessive allocations or
// obviously corrupt packets.
const (
	maxDescriptors = 512
	maxPictures    = 512
	maxMobiles     = 512
	maxBubbles     = 128
)

var skipPictShift = map[uint16]struct{}{
    3037: {},
}

// bgKey identifies a background sprite at a stable relative location
// (anchored by cumulative world shift).
type bgKey struct {
    id uint16
    h  int16
    v  int16
}

type bgStableInfo struct {
    count     int
    lastFrame int
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

func sortDescriptors(descs []frameDescriptor) {
	sort.Slice(descs, func(i, j int) bool {
		return descs[i].Index < descs[j].Index
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

var pixelCountMu sync.RWMutex
var pixelCountCache = make(map[uint16]int)

// pixelDataCache caches raw pixel data for images so that subsequent
// nonTransparentPixels calls do not need to read back from the GPU.
// Reading pixels triggers a GPU stall, so we do it once per image and reuse
// the cached slice thereafter. To prevent unbounded memory growth the cache
// is capped at a fixed number of entries and evicts least-recently-used
// items when full. 128 entries correspond to roughly 2MB of RGBA pixel data
// for 64x64 images, which comfortably covers the working set while keeping
// memory usage modest.
const pixelDataCacheLimit = 128

type pixelDataEntry struct {
	id   uint16
	data []byte
}

var (
	pixelDataMu    sync.Mutex
	pixelDataCache = make(map[uint16]*list.Element)
	pixelDataList  = list.New()
)

// nonTransparentPixels returns the number of non-transparent pixels for the
// given picture ID. The result is cached after the first computation. When
// possible, it uses raw pixel slices for faster counting and falls back to the
// generic img.At path otherwise.
func nonTransparentPixels(id uint16) int {
	pixelCountMu.RLock()
	if !gs.NoCaching {
		if c, ok := pixelCountCache[id]; ok {
			pixelCountMu.RUnlock()
			return c
		}
	}
	pixelCountMu.RUnlock()

	var img image.Image = loadImage(id)
	bounds := img.Bounds()
	count := 0

	switch src := img.(type) {
	case *ebiten.Image:
		// Fast path: read raw pixels once and optionally cache them.
		w, h := bounds.Dx(), bounds.Dy()
		var buf []byte
		if gs.NoCaching {
			buf = make([]byte, 4*w*h)
			src.ReadPixels(buf)
		} else {
			pixelDataMu.Lock()
			if elem, ok := pixelDataCache[id]; ok {
				entry := elem.Value.(*pixelDataEntry)
				if len(entry.data) < 4*w*h {
					entry.data = make([]byte, 4*w*h)
					src.ReadPixels(entry.data)
				}
				buf = entry.data
				pixelDataList.MoveToFront(elem)
			} else {
				buf = make([]byte, 4*w*h)
				src.ReadPixels(buf)
				elem := pixelDataList.PushFront(&pixelDataEntry{id: id, data: buf})
				pixelDataCache[id] = elem
				if pixelDataList.Len() > pixelDataCacheLimit {
					if back := pixelDataList.Back(); back != nil {
						pixelDataList.Remove(back)
						e := back.Value.(*pixelDataEntry)
						delete(pixelDataCache, e.id)
					}
				}
			}
			pixelDataMu.Unlock()
		}
		for i := 3; i < len(buf); i += 4 {
			if buf[i] != 0 {
				count++
			}
		}
	case *image.RGBA:
		// Fast path for RGBA images: directly access the Pix slice.
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			i := (y-bounds.Min.Y)*src.Stride + (bounds.Min.X * 4)
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				if src.Pix[i+3] != 0 {
					count++
				}
				i += 4
			}
		}
	default:
		// Fallback: use the image's At method.
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

// pictureShift returns the (dx, dy) movement that most on-screen pictures agree on
// between two consecutive frames. Pictures are matched by PictID (duplicates
// included) and weighted by their non-transparent pixel counts. The returned
// slice contains the indexes within the current frame that contributed to the
// winning movement. The boolean result is false when no majority offset is
// found.
func pictureShift(prev, cur []framePicture) (int, int, []int, bool) {
	if len(prev) == 0 || len(cur) == 0 {
		logDebug("pictureShift: no data prev=%d cur=%d", len(prev), len(cur))
		return 0, 0, nil, false
	}

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
	bestCount := 0
	for k, c := range counts {
		if c > bestCount {
			best = k
			bestCount = c
		}
	}
	logDebug("pictureShift: counts=%v best=%v count=%d total=%d", counts, best, bestCount, total)
	if bestCount*2 <= total {
		logDebug("pictureShift: no majority best=%d total=%d", bestCount, total)
		return 0, 0, nil, false
	}
	if best[0]*best[0]+best[1]*best[1] > maxInterpPixels*maxInterpPixels {
		logDebug("pictureShift: motion too large (%d,%d)", best[0], best[1])
		return 0, 0, nil, false
	}

	idxs := make([]int, 0, len(idxMap[best]))
	for idx := range idxMap[best] {
		idxs = append(idxs, idx)
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
func handleDrawState(m []byte) {
	frameCounter++

	if len(m) < 11 { // 2 byte tag + 9 bytes minimum
		return
	}

	data := append([]byte(nil), m[2:]...)
	if drawStateEncrypted {
		simpleEncrypt(data)
	}
	if err := parseDrawState(data); err != nil {
		logDebugPacket(fmt.Sprintf("parseDrawState error: %v", err), data)
	}
}

// parseInventory walks the inventory command stream and returns the remaining
// slice and success flag. The layout mirrors the old Mac client's
// HandleInventory function.
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
			return nil, false
		}
		cmdCount = int(data[0])
		cmd = int(data[1])
		data = data[2:]
	}

	for i := 0; i < cmdCount; i++ {
		if cmd&kInvCmdIndex != 0 && (cmd&^kInvCmdIndex == kInvCmdFull || cmd&^kInvCmdIndex == kInvCmdNone) {
			if len(data) < 1 {
				logError("inventory: missing index for cmd %x", cmd)
				return nil, false
			}
			// consume the index and drop the flag
			data = data[1:]
			cmd &^= kInvCmdIndex
			if cmd == kInvCmdNone {
				// nothing else to do
				if len(data) > 0 {
					cmd = int(data[0])
					data = data[1:]
				}
				continue
			}
		}

		base := cmd &^ kInvCmdIndex
		switch base {
		case kInvCmdFull:
			if cmd&kInvCmdIndex != 0 {
				if len(data) < 1 {
					return nil, false
				}
				// consume index byte but ignore value
				data = data[1:]
			}
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
			for j := 0; j < itemCount; j++ {
				ids[j] = binary.BigEndian.Uint16(data[equipBytes+j*2:])
			}
			eq := make([]bool, itemCount)
			for j := 0; j < itemCount; j++ {
				if equips[j/8]&(1<<uint(j%8)) != 0 {
					eq[j] = true
				}
			}
			setFullInventory(ids, eq)
			data = data[bytesNeeded:]
		case kInvCmdNone:
			if cmd&kInvCmdIndex != 0 {
				if len(data) < 1 {
					return nil, false
				}
				// consume index byte but ignore value
				data = data[1:]
			}
			// nothing else to do for kInvCmdNone
		case kInvCmdAdd, kInvCmdAddEquip, kInvCmdDelete, kInvCmdEquip,
			kInvCmdUnequip, kInvCmdName:
			if len(data) < 2 {
				logError("inventory: cmd %x missing id", cmd)
				return nil, false
			}
			id := binary.BigEndian.Uint16(data[:2])
			data = data[2:]
			idx := 0
			if cmd&kInvCmdIndex != 0 {
				if len(data) < 1 {
					logError("inventory: cmd %x missing index", cmd)
					return nil, false
				}
				idx = int(data[0])
				data = data[1:]
			}
			var name string
			if base == kInvCmdAdd || base == kInvCmdAddEquip || base == kInvCmdName {
				nidx := bytes.IndexByte(data, 0)
				if nidx < 0 {
					logError("inventory: cmd %x missing name", cmd)
					return nil, false
				}
				name = string(data[:nidx])
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
			}
		default:
			logError("inventory: unknown command %x (%d bytes left)", cmd, len(data))
			return data, true
		}
		if len(data) > 0 {
			cmd = int(data[0])
			data = data[1:]
		} else {
			cmd = kInvCmdNone
		}
	}
	if cmd == kInvCmdNone|kInvCmdIndex {
		if len(data) < 1 {
			return nil, false
		}
		// consume trailing index byte
		data = data[1:]
	} else if cmd != kInvCmdNone {
		logError("inventory: unexpected trailing cmd %d", cmd)
	}
	for len(data) > 0 && data[0] == 0 {
		data = data[1:]
	}
	inventoryDirty = true
	return data, true
}

// parseDrawState decodes the draw state data. It returns an error when the
// packet appears malformed, indicating the parsing stage that failed.
func parseDrawState(data []byte) error {
	stage := "header"
	if len(data) < 9 {
		return errors.New(stage)
	}

	ackCmd := data[0]
	ackFrame = int32(binary.BigEndian.Uint32(data[1:5]))
	resendFrame = int32(binary.BigEndian.Uint32(data[5:9]))
	p := 9

	stage = "descriptor count"
	if len(data) <= p {
		return errors.New(stage)
	}
	descCount := int(data[p])
	p++
	if descCount > maxDescriptors {
		return errors.New(stage)
	}
	stage = "descriptor"
	descs := make([]frameDescriptor, 0, descCount)
	for i := 0; i < descCount && p < len(data); i++ {
		if p+4 > len(data) {
			return errors.New(stage)
		}
		d := frameDescriptor{}
		d.Index = data[p]
		d.Type = data[p+1]
		d.PictID = binary.BigEndian.Uint16(data[p+2:])
		p += 4
		if idx := bytes.IndexByte(data[p:], 0); idx >= 0 {
			d.Name = string(data[p : p+idx])
			p += idx + 1
			if d.Name == playerName {
				playerIndex = d.Index
			}
		} else {
			return errors.New(stage)
		}
		if p >= len(data) {
			return errors.New(stage)
		}
		cnt := int(data[p])
		p++
		if p+cnt > len(data) {
			return errors.New(stage)
		}
		d.Colors = append([]byte(nil), data[p:p+cnt]...)
		p += cnt
		updatePlayerAppearance(d.Name, d.PictID, d.Colors, d.Type == kDescNPC)
		descs = append(descs, d)
	}

	stage = "stats"
	if len(data) < p+7 {
		return errors.New(stage)
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
		return errors.New(stage)
	}
	pictCount := int(data[p])
	p++
	pictAgain := 0
	stage = "picture header"
	if pictCount == 255 {
		if len(data) < p+2 {
			return errors.New(stage)
		}
		pictAgain = int(data[p])
		pictCount = int(data[p+1])
		p += 2
	}
	stage = "picture count"
	if pictAgain+pictCount > maxPictures {
		return errors.New(stage)
	}

	stage = "pictures"
	pics := make([]framePicture, 0, pictAgain+pictCount)
	br := bitReader{data: data[p:]}
	for i := 0; i < pictCount; i++ {
		idBits, ok := br.readBits(14)
		if !ok {
			return errors.New("truncated picture bit stream")
		}
		hBits, ok := br.readBits(11)
		if !ok {
			return errors.New("truncated picture bit stream")
		}
		vBits, ok := br.readBits(11)
		if !ok {
			return errors.New("truncated picture bit stream")
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
		return errors.New(stage)
	}
	mobileCount := int(data[p])
	p++
	if mobileCount > maxMobiles {
		return errors.New(stage)
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
		return errors.New(stage)
	}

	stage = "state size"
	if len(data) < p+2 {
		return errors.New(stage)
	}
	stateLen := int(binary.BigEndian.Uint16(data[p:]))
	p += 2
	if len(data) < p+stateLen {
		return errors.New(stage)
	}
	stateData := data[p : p+stateLen]

	stateMu.Lock()
	state.ackCmd = ackCmd
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
	if gs.BlendMobiles {
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
	dx, dy, bgIdxs, ok := pictureShift(prevPics, newPics)
    if gs.MotionSmoothing {
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
    // Maintain cumulative world shift when motion is reliable.
    if ok {
        state.worldShiftX += state.picShiftX
        state.worldShiftY += state.picShiftY
    } else {
        state.worldShiftX = 0
        state.worldShiftY = 0
        if state.bgStable != nil {
            for k := range state.bgStable {
                delete(state.bgStable, k)
            }
        }
    }
	} else {
		state.picShiftX = 0
		state.picShiftY = 0
	}
    // Retain background pictures that disappeared, but only if we've seen the
    // same sprite at the same relative location for more than 1 consecutive
    // frame. This keeps true backgrounds when the server stops sending them.
    if ok {
        type picKey struct {
            id   uint16
            h, v int16
        }
        lookup := make(map[picKey]struct{}, len(newPics))
        for _, p := range newPics {
            lookup[picKey{p.PictID, p.H, p.V}] = struct{}{}
        }
        if state.bgStable == nil {
            state.bgStable = make(map[bgKey]bgStableInfo)
        }
        for _, p := range prevPics {
            if !p.Background {
                continue
            }
            shiftedH := int16(int(p.H) + state.picShiftX)
            shiftedV := int16(int(p.V) + state.picShiftY)
            key := picKey{p.PictID, shiftedH, shiftedV}
            if _, found := lookup[key]; found {
                continue // still present after shift
            }
            // Anchored position for stability check uses previous-frame coords
            // normalized by world shift prior to this frame's delta.
            anchorH := int16(int(p.H) - (state.worldShiftX - state.picShiftX))
            anchorV := int16(int(p.V) - (state.worldShiftY - state.picShiftY))
            bk := bgKey{id: p.PictID, h: anchorH, v: anchorV}
            if info, ok := state.bgStable[bk]; ok && info.lastFrame == frameCounter-1 && info.count >= 5 {
                p.H = shiftedH
                p.V = shiftedV
                p.Again = true
                p.Ghost = true
                newPics = append(newPics, p)
                bgIdxs = append(bgIdxs, len(newPics)-1)
                lookup[key] = struct{}{}
            }
        }
    }
	if !ok {
		prevPics = nil
		again = 0
		newPics = append([]framePicture(nil), pics...)
		state.prevDescs = nil
		state.prevMobiles = nil
		state.prevTime = time.Time{}
		state.curTime = time.Time{}
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
		if moving && gs.smoothMoving {
			bestDist := maxInterpPixels*maxInterpPixels + 1
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
			if best != nil && bestDist <= maxInterpPixels*maxInterpPixels {
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

    // Update background stability counters using anchored positions.
    if ok {
        if state.bgStable == nil {
            state.bgStable = make(map[bgKey]bgStableInfo)
        }
        seen := make(map[bgKey]struct{})
        for _, idx := range bgIdxs {
            if idx < 0 || idx >= len(newPics) {
                continue
            }
            p := newPics[idx]
            // Count only sprites actually sent by the server this frame.
            // Skip retained ghosts so stability reflects continuous sightings.
            if p.Ghost {
                continue
            }
            anchorH := int16(int(p.H) - state.worldShiftX)
            anchorV := int16(int(p.V) - state.worldShiftY)
            bk := bgKey{id: p.PictID, h: anchorH, v: anchorV}
            if _, dup := seen[bk]; dup {
                continue
            }
            seen[bk] = struct{}{}
            info := state.bgStable[bk]
            if info.lastFrame == frameCounter-1 {
                info.count++
            } else {
                info.count = 1
            }
            info.lastFrame = frameCounter
            state.bgStable[bk] = info
        }
        // prune stale entries
        for k, info := range state.bgStable {
            if info.lastFrame < frameCounter-180 {
                delete(state.bgStable, k)
            }
        }
    }

	state.pictures = newPics

	needPrev := (gs.MotionSmoothing || gs.BlendMobiles) && ok
	if needPrev {
		if state.prevMobiles == nil {
			state.prevMobiles = make(map[uint8]frameMobile)
		}
		state.prevMobiles = make(map[uint8]frameMobile, len(state.mobiles))
		for idx, m := range state.mobiles {
			state.prevMobiles[idx] = m
		}
	}
	needAnimUpdate := (gs.MotionSmoothing || (gs.BlendMobiles && changed)) && ok
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
		logDebug("interp mobiles interval=%v", interval)
		state.prevTime = time.Now()
		state.curTime = state.prevTime.Add(interval)
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
		state.mobiles[m.Index] = m
	}
	ack := state.ackCmd
	light := state.lightingFlags
	stateMu.Unlock()

	logDebug("draw state cmd=%d ack=%d resend=%d light=%#x desc=%d pict=%d again=%d mobile=%d state=%d",
		ack, ackFrame, resendFrame, light, len(descs), len(pics), pictAgain, len(mobiles), len(stateData))

	stage = "info strings"
	for {
		if len(stateData) == 0 {
			return errors.New(stage)
		}
		idx := bytes.IndexByte(stateData, 0)
		if idx < 0 {
			return errors.New(stage)
		}
		if idx == 0 {
			stateData = stateData[1:]
			break
		}
		handleInfoText(stateData[:idx])
		stateData = stateData[idx+1:]
	}

	stage = "bubble count"
	if len(stateData) == 0 {
		return errors.New(stage)
	}
	bubbleCount := int(stateData[0])
	stateData = stateData[1:]
	if bubbleCount > maxBubbles {
		return errors.New(stage)
	}
	stage = "bubble"
	for i := 0; i < bubbleCount && len(stateData) > 0; i++ {
		off := len(data) - len(stateData)
		if len(stateData) < 2 {
			return fmt.Errorf("bubble=%d off=%d len=%d", i, off, len(stateData))
		}
		idx := stateData[0]
		typ := int(stateData[1])
		p := 2
		if typ&kBubbleNotCommon != 0 {
			if len(stateData) < p+1 {
				return fmt.Errorf("bubble=%d off=%d len=%d", i, off, len(stateData))
			}
			p++
		}
		var h, v int16
		if typ&kBubbleFar != 0 {
			if len(stateData) < p+4 {
				return fmt.Errorf("bubble=%d off=%d len=%d", i, off, len(stateData))
			}
			h = int16(binary.BigEndian.Uint16(stateData[p:]))
			v = int16(binary.BigEndian.Uint16(stateData[p+2:]))
			p += 4
		}
		if len(stateData) <= p {
			return fmt.Errorf("bubble=%d off=%d len=%d", i, off, len(stateData))
		}
		end := bytes.IndexByte(stateData[p:], 0)
		if end < 0 {
			return fmt.Errorf("bubble=%d off=%d len=%d", i, off, len(stateData))
		}
		bubbleData := stateData[:p+end+1]
		if verb, txt, bubbleName, lang, code, target := decodeBubble(bubbleData); txt != "" || code != kBubbleCodeKnown {
			name := bubbleName
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
			if gs.SpeechBubbles && txt != "" && !blockBubbles {
				frameMu.Lock()
				interval := frameInterval
				frameMu.Unlock()
				frames := int((4*time.Second + interval - 1) / interval)
				b := bubble{Index: idx, Text: txt, Type: typ, ExpireFrame: frameCounter + frames}
				switch typ & kBubbleTypeMask {
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
							msg = fmt.Sprintf("%v thinks to you, %v", name, txt)
						case thinkToClan:
							msg = fmt.Sprintf("%v thinks to your clan, %v", name, txt)
						case thinkToGroup:
							msg = fmt.Sprintf("%v thinks to a group, %v", name, txt)
						default:
							msg = fmt.Sprintf("%v thinks, %v", name, txt)
						}
					} else if typ&kBubbleNotCommon != 0 {
						langWord := lang
						lw := strings.ToLower(langWord)
						if langWord == "" || strings.HasPrefix(lw, "unknown") {
							langWord = "an unknown language"
						}
						if code == kBubbleCodeKnown {
							msg = fmt.Sprintf("%v %v in %v, %v", name, verb, langWord, txt)
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
			chatMessage(msg)
			if gs.MessagesToConsole {
				consoleMessage(msg)
			}
		}
		stateData = stateData[p+end+1:]
	}

	stage = "sound count"
	if len(stateData) < 1 {
		return errors.New(stage)
	}
	soundCount := int(stateData[0])
	stateData = stateData[1:]
	stage = "sounds"
	if len(stateData) < soundCount*2 {
		return errors.New(stage)
	}
	for i := 0; i < soundCount; i++ {
		id := binary.BigEndian.Uint16(stateData[:2])
		stateData = stateData[2:]
		playSound(id)
	}
	stage = "inventory"
	var invOK bool
	stateData, invOK = parseInventory(stateData)
	if !invOK {
		return errors.New(stage)
	}
	return nil
}
