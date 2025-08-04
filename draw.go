package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"
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
	PictID uint16
	H, V   int16
}

type frameMobile struct {
	Index  uint8
	State  uint8
	H, V   int16
	Colors uint8
}

const poseDead = 32
const maxInterpPixels = 128

// sanity limits for parsed counts to avoid excessive allocations or
// obviously corrupt packets.
const (
	maxDescriptors = 512
	maxPictures    = 512
	maxMobiles     = 512
	maxBubbles     = 128
)

// bitReader helps decode the packed picture fields.
type bitReader struct {
	data   []byte
	bitPos int
}

func (br *bitReader) readBits(n int) uint32 {
	var v uint32
	for n > 0 {
		if br.bitPos/8 >= len(br.data) {
			return v
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
	return v
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

// onScreen reports whether the picture lies within the visible playfield.
func onScreen(p framePicture) bool {
	x := int(p.H) + fieldCenterX
	y := int(p.V) + fieldCenterY
	return x >= 0 && x < gameAreaSizeX && y >= 0 && y < gameAreaSizeY
}

// pictureShift returns the (dx, dy) movement that most on-screen pictures agree on
// between two consecutive frames. Pictures are matched by PictID (duplicates
// included). The boolean result is false when no majority offset is found.
func pictureShift(prev, cur []framePicture) (int, int, bool) {
	if len(prev) == 0 || len(cur) == 0 {
		logDebug("pictureShift: no data prev=%d cur=%d", len(prev), len(cur))
		return 0, 0, false
	}

	counts := make(map[[2]int]int)
	total := 0
	maxInt := int(^uint(0) >> 1)
	for _, p := range prev {
		if !onScreen(p) {
			continue
		}
		bestDist := maxInt
		var bestDx, bestDy int
		matched := false
		for _, c := range cur {
			if p.PictID != c.PictID || !onScreen(c) {
				continue
			}
			dx := int(c.H) - int(p.H)
			dy := int(c.V) - int(p.V)
			dist := dx*dx + dy*dy
			if dist < bestDist {
				bestDist = dist
				bestDx = dx
				bestDy = dy
				matched = true
			}
		}
		if matched {
			counts[[2]int{bestDx, bestDy}]++
			total++
		}
	}
	if total == 0 {
		logDebug("pictureShift: no matching pairs")
		return 0, 0, false
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
		return 0, 0, false
	}
	if best[0]*best[0]+best[1]*best[1] > maxInterpPixels*maxInterpPixels {
		logDebug("pictureShift: motion too large (%d,%d)", best[0], best[1])
		return 0, 0, false
	}
	return best[0], best[1], true
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
		logDebugPacket(fmt.Sprintf("failed to parse draw state stage=%v", err), data)
	}
}

// parseInventory walks the inventory command stream and returns the remaining
// slice and success flag. The layout mirrors the old Mac client's
// HandleInventory function.
func parseInventory(data []byte) ([]byte, bool) {
	if len(data) == 0 {
		return nil, false
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
		base := cmd &^ kInvCmdIndex
		switch base {
		case kInvCmdFull:
			if cmd&kInvCmdIndex != 0 || len(data) < 1 {
				return nil, false
			}
			itemCount := int(data[0])
			data = data[1:]
			bytesNeeded := (itemCount+7)>>3 + itemCount*2
			if len(data) < bytesNeeded {
				return nil, false
			}
			data = data[bytesNeeded:]
		case kInvCmdAdd, kInvCmdAddEquip, kInvCmdDelete, kInvCmdEquip,
			kInvCmdUnequip, kInvCmdName:
			if len(data) < 2 {
				return nil, false
			}
			data = data[2:]
			if cmd&kInvCmdIndex != 0 {
				if len(data) < 1 {
					return nil, false
				}
				data = data[1:]
			}
			if base == kInvCmdAdd || base == kInvCmdAddEquip || base == kInvCmdName {
				idx := bytes.IndexByte(data, 0)
				if idx < 0 {
					return nil, false
				}
				data = data[idx+1:]
			}
		default:
			return nil, false
		}
		if len(data) == 0 {
			return nil, false
		}
		cmd = int(data[0])
		data = data[1:]
	}
	if cmd != kInvCmdNone {
		return nil, false
	}
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
		updatePlayerAppearance(d.Name, d.PictID, d.Colors)
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
		id := uint16(br.readBits(14))
		h := signExtend(br.readBits(11), 11)
		v := signExtend(br.readBits(11), 11)
		pics = append(pics, framePicture{PictID: id, H: h, V: v})
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

	stateData := data[p:]

	stateMu.Lock()
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
	if onion {
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
	if state.descriptors == nil {
		state.descriptors = make(map[uint8]frameDescriptor)
	}
	for _, d := range descs {
		state.descriptors[d.Index] = d
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
	if interp {
		dx, dy, ok := pictureShift(prevPics, newPics)
		logDebug("interp pictures again=%d prev=%d cur=%d shift=(%d,%d) ok=%t", again, len(prevPics), len(newPics), dx, dy, ok)
		if !ok {
			logDebug("prev pics: %v", picturesSummary(prevPics))
			logDebug("new  pics: %v", picturesSummary(newPics))
		}
		if ok {
			state.picShiftX = dx
			state.picShiftY = dy
		} else {
			state.picShiftX = 0
			state.picShiftY = 0
		}
	}
	state.pictures = newPics

	needAnimUpdate := interp || (onion && changed)
	if needAnimUpdate {
		// save previous mobile positions for interpolation and fading
		if state.prevMobiles == nil {
			state.prevMobiles = make(map[uint8]frameMobile)
		}
		// copy current mobiles to prevMobiles before replacing
		state.prevMobiles = make(map[uint8]frameMobile, len(state.mobiles))
		for idx, m := range state.mobiles {
			state.prevMobiles[idx] = m
		}
		const defaultInterval = time.Second / 5
		interval := defaultInterval
		if !state.prevTime.IsZero() && !state.curTime.IsZero() {
			if d := state.curTime.Sub(state.prevTime); d > 0 {
				interval = d
			}
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
	stateMu.Unlock()

	logDebug("draw state cmd=%d ack=%d resend=%d desc=%d pict=%d again=%d mobile=%d state=%d",
		ackCmd, ackFrame, resendFrame, len(descs), len(pics), pictAgain, len(mobiles), len(stateData))

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
	for i := 0; i < bubbleCount; i++ {
		if len(stateData) < 2 {
			return errors.New(stage)
		}
		idx := stateData[0]
		typ := int(stateData[1])
		p := 2
		if typ&kBubbleNotCommon != 0 {
			if len(stateData) < p+1 {
				return errors.New(stage)
			}
			p++
		}
		var h, v int16
		if typ&kBubbleFar != 0 {
			if len(stateData) < p+4 {
				return errors.New(stage)
			}
			h = int16(binary.BigEndian.Uint16(stateData[p:]))
			v = int16(binary.BigEndian.Uint16(stateData[p+2:]))
			p += 4
		}
		if len(stateData) < p {
			return errors.New(stage)
		}
		end := bytes.IndexByte(stateData[p:], 0)
		if end < 0 {
			return errors.New(stage)
		}
		bubbleData := stateData[:p+end+1]
		if verb, txt, bubbleName, lang, code, target := decodeBubble(bubbleData); txt != "" || code != kBubbleCodeKnown {
			name := bubbleName
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
			if showBubbles && txt != "" {
				bx, by := h, v
				if typ&kBubbleFar == 0 {
					stateMu.Lock()
					if m, ok := state.mobiles[idx]; ok {
						bx, by = m.H, m.V
					}
					stateMu.Unlock()
				}
				stateMu.Lock()
				state.bubbles = append(state.bubbles, debugBubble{H: bx, V: by, Text: txt, Expire: time.Now().Add(4 * time.Second)})
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
			if idx != playerIndex {
				addMessage(msg)
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
	var ok bool
	stateData, ok = parseInventory(stateData)
	if !ok {
		return errors.New(stage)
	}
	return nil
}
