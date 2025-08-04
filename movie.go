package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
)

const (
	flagStale        = 0x01
	flagMobileData   = 0x02
	flagGameState    = 0x04
	flagPictureTable = 0x08
)

const movieSignature = 0xdeadbeef
const oldestMovieVersion = 193

var movieRevision int32

func parseMovie(path string, clientVersion int) ([][]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 8 {
		return nil, fmt.Errorf("short file")
	}
	if binary.BigEndian.Uint32(data[:4]) != movieSignature {
		return nil, fmt.Errorf("bad signature")
	}
	version := binary.BigEndian.Uint16(data[4:6])
	revision := binary.BigEndian.Uint16(data[16:18])
	// Arindal movies store version numbers 100x larger.
	if version > 50000 {
		version /= 100
	}
	if version < oldestMovieVersion {
		return nil, fmt.Errorf("movie version too old: %d", version)
	}
	headerLen := int(binary.BigEndian.Uint16(data[6:8]))
	if headerLen <= 0 || headerLen > len(data) {
		headerLen = 24
	}
	logDebug("movie version %d.%d headerLen %d", version, revision, headerLen)

	pos := headerLen
	sign := []byte{0xde, 0xad, 0xbe, 0xef}
	frames := [][]byte{}
	frameNum := 0
	for pos+12 <= len(data) {
		if binary.BigEndian.Uint32(data[pos:pos+4]) != movieSignature {
			idx := bytes.Index(data[pos:], sign)
			if idx < 0 {
				break
			}
			pos += idx
			continue
		}
		frame := binary.BigEndian.Uint32(data[pos+4 : pos+8])
		size := int(binary.BigEndian.Uint16(data[pos+8 : pos+10]))
		flags := binary.BigEndian.Uint16(data[pos+10 : pos+12])
		logDebug("frame %d index=%d size=%d flags=0x%x", frameNum, frame, size, flags)
		pos += 12
		if flags&flagGameState != 0 {
			logDebug("GameState block at %d", pos)
			if pos+24 > len(data) {
				break
			}
			maxSize := int(binary.BigEndian.Uint32(data[pos+12 : pos+16]))
			start := pos + 24
			end := start + maxSize
			if end > len(data) {
				break
			}
			parseGameState(data[start:end], version, revision)
			pos = end
		}
		if flags&flagMobileData != 0 {
			logDebug("MobileData table at %d", pos)
			pos = parseMobileTable(data, pos, version, revision)
		}
		if flags&flagPictureTable != 0 {
			logDebug("PictureTable at %d", pos)
			if pos+2 > len(data) {
				break
			}
			count := int(binary.BigEndian.Uint16(data[pos : pos+2]))
			pos += 2
			pics := make([]framePicture, 0, count)
			for i := 0; i < count && pos+6 <= len(data); i++ {
				id := binary.BigEndian.Uint16(data[pos : pos+2])
				h := int16(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
				v := int16(binary.BigEndian.Uint16(data[pos+4 : pos+6]))
				pos += 6
				pics = append(pics, framePicture{PictID: id, H: h, V: v})
			}
			if pos+4 <= len(data) {
				pos += 4
			}
			stateMu.Lock()
			state.pictures = pics
			stateMu.Unlock()
		}
		if size > 0 {
			if pos+size > len(data) {
				break
			}
			frames = append(frames, append([]byte(nil), data[pos:pos+size]...))
			pos += size
		} else {
			idx := bytes.Index(data[pos:], sign)
			if idx < 0 {
				break
			}
			pos += idx
		}
		frameNum++
	}
	return frames, nil
}

// parseGameState decodes an initial game state block found in movies. The
// payload mirrors the data sent by the server after login and may embed
// descriptor and picture tables. The decoding here is intentionally
// lightweight; only the pieces needed to prime state.mobiles and
// state.descriptors are extracted.
func parseGameState(gs []byte, version, revision uint16) {
	if len(gs) == 0 {
		return
	}
	if i := bytes.IndexByte(gs, 0); i >= 0 {
		dispatchMessage(gs[:i], false)
		gs = gs[i+1:]
	}

	// Attempt to extract a picture table if present. The table format
	// matches the PictureTable blocks used by regular frames.
	if len(gs) >= 2 {
		count := int(binary.BigEndian.Uint16(gs[:2]))
		size := 2 + 6*count + 4
		if count > 0 && size <= len(gs) {
			pos := 2
			pics := make([]framePicture, 0, count)
			for i := 0; i < count && pos+6 <= len(gs); i++ {
				id := binary.BigEndian.Uint16(gs[pos : pos+2])
				h := int16(binary.BigEndian.Uint16(gs[pos+2 : pos+4]))
				v := int16(binary.BigEndian.Uint16(gs[pos+4 : pos+6]))
				pos += 6
				pics = append(pics, framePicture{PictID: id, H: h, V: v})
			}
			if pos+4 <= len(gs) {
				pos += 4
			}
			stateMu.Lock()
			state.pictures = pics
			stateMu.Unlock()
			gs = gs[pos:]
		}
	}

	// Mobile tables end with a -1 index sentinel. If that marker exists,
	// feed the data through the regular parser.
	if bytes.Contains(gs, []byte{0xff, 0xff, 0xff, 0xff}) {
		parseMobileTable(gs, 0, version, revision)
	}
}

// parseMobileTable decodes the descriptor table for a frame.  Descriptor
// layouts have changed many times over Clan Lord's long history; the version
// checks below mirror the Mac client's ReadMobileTable/Read1Descriptor logic.
// Version breakpoints correspond to kOldestMovieVersion and friends in the
// original source.
func parseMobileTable(data []byte, pos int, version, revision uint16) int {
	const descTableSize = 266 // kDescTableSize

	type layout struct {
		descSize            int
		colorsOffset        int
		nameOffset          int
		numColorsOffset     int
		bubbleCounterOffset int
	}

	var l layout
	switch {
	case version > 141: // v142+ (current format)
		l = layout{descSize: 156, colorsOffset: 56, nameOffset: 86, numColorsOffset: 48, bubbleCounterOffset: 28}
	case version > 113: // v114-141
		l = layout{descSize: 150, colorsOffset: 52, nameOffset: 82, numColorsOffset: 44, bubbleCounterOffset: 24}
	case version > 105: // v106-113
		l = layout{descSize: 142, colorsOffset: 52, nameOffset: 82, numColorsOffset: 44, bubbleCounterOffset: 24}
	case version > 97: // v98-105
		l = layout{descSize: 130, colorsOffset: 40, nameOffset: 70, numColorsOffset: 32, bubbleCounterOffset: 24}
	default: // v80-97
		if version < 80 {
			logDebug("unsupported mobile table version %d", version)
			return pos
		}
		l = layout{descSize: 126, colorsOffset: 36, nameOffset: 66, numColorsOffset: 28, bubbleCounterOffset: 20}
	}

	for pos+4 <= len(data) {
		idx := int32(binary.BigEndian.Uint32(data[pos : pos+4]))
		pos += 4
		if idx == -1 {
			break
		}
		hasMobile := idx < descTableSize
		if !hasMobile {
			idx -= descTableSize
		}

		var mob frameMobile
		if hasMobile {
			if pos+16 > len(data) {
				return len(data)
			}
			mob.Index = uint8(idx)
			mob.State = uint8(binary.BigEndian.Uint32(data[pos : pos+4]))
			mob.H = int16(binary.BigEndian.Uint32(data[pos+4 : pos+8]))
			mob.V = int16(binary.BigEndian.Uint32(data[pos+8 : pos+12]))
			mob.Colors = uint8(binary.BigEndian.Uint32(data[pos+12 : pos+16]))
			pos += 16
		}

		if pos+l.descSize > len(data) {
			return len(data)
		}
		buf := data[pos : pos+l.descSize]
		pos += l.descSize

		d := frameDescriptor{Index: uint8(idx)}
		d.Type = uint8(binary.BigEndian.Uint32(buf[16:20]))
		pict := binary.BigEndian.Uint32(buf[0:4])
		d.PictID = uint16(pict & 0xffff)

		numColors := int(binary.BigEndian.Uint32(buf[l.numColorsOffset : l.numColorsOffset+4]))
		if numColors < 0 || numColors > 30 {
			numColors = 30
		}
		end := l.colorsOffset + numColors
		if end > len(buf) {
			end = len(buf)
		}
		d.Colors = append([]byte(nil), buf[l.colorsOffset:end]...)

		nameBytes := buf[l.nameOffset : l.nameOffset+48]
		if i := bytes.IndexByte(nameBytes, 0); i >= 0 {
			d.Name = string(nameBytes[:i])
		} else {
			d.Name = string(nameBytes)
		}

		bubbleCounter := int32(binary.BigEndian.Uint32(buf[l.bubbleCounterOffset : l.bubbleCounterOffset+4]))
		if bubbleCounter != 0 {
			if pos+2 > len(data) {
				return len(data)
			}
			lgt := int(binary.BigEndian.Uint16(data[pos : pos+2]))
			pos += 2
			if pos+lgt > len(data) {
				return len(data)
			}
			_ = string(data[pos : pos+lgt]) // bubble text, ignored
			pos += lgt
		}

		stateMu.Lock()
		if hasMobile {
			if state.mobiles == nil {
				state.mobiles = make(map[uint8]frameMobile)
			}
			state.mobiles[mob.Index] = mob
		}
		if state.descriptors == nil {
			state.descriptors = make(map[uint8]frameDescriptor)
		}
		state.descriptors[d.Index] = d
		stateMu.Unlock()
	}
	return pos
}
