package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	flagStale        = 0x01
	flagMobileData   = 0x02
	flagGameState    = 0x04
	flagPictureTable = 0x08
)

const movieSignature = 0xdeadbeef
const oldestMovieVersion = 193

var (
	movieVersion  uint16
	movieRevision int32
)

type movieFrame struct {
	data    []byte
	index   int32
	flags   uint16
	preData []byte
}

func parseMovie(path string, clVersion int) ([]movieFrame, error) {
	data, err := loadMovieData(path)
	if err != nil {
		return nil, err
	}
	return parseMovieData(data, clVersion)
}

func parseMovieZipBytes(zipData []byte, clVersion int) ([]movieFrame, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, err
	}
	data, err := extractMovieFromZip(zr)
	if err != nil {
		return nil, err
	}
	return parseMovieData(data, clVersion)
}

func loadMovieData(path string) ([]byte, error) {
	if strings.HasSuffix(strings.ToLower(path), ".zip") {
		zr, err := zip.OpenReader(path)
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		return extractMovieFromZip(&zr.Reader)
	}
	return os.ReadFile(path)
}

func extractMovieFromZip(zr *zip.Reader) ([]byte, error) {
	var target *zip.File
	for _, f := range zr.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".clmov") {
			target = f
			break
		}
	}
	if target == nil {
		if len(zr.File) == 0 {
			return nil, fmt.Errorf("no files in archive")
		}
		target = zr.File[0]
	}
	rc, err := target.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func parseMovieData(data []byte, clVersion int) ([]movieFrame, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("short file")
	}
	if binary.BigEndian.Uint32(data[:4]) != movieSignature {
		return nil, fmt.Errorf("bad signature")
	}
	version := binary.BigEndian.Uint16(data[4:6])
	revision := binary.BigEndian.Uint32(data[16:20])
	movieRevision = int32(revision)
	// Arindal movies store version numbers 100x larger.
	if version > 50000 {
		version /= 100
	}
	movieVersion = version
	if version < oldestMovieVersion {
		return nil, fmt.Errorf("movie version too old: %d", version)
	}
	headerLen := int(binary.BigEndian.Uint16(data[6:8]))
	if headerLen <= 0 || headerLen > len(data) {
		headerLen = 24
	}
	logDebug("movie version %d.%d headerLen %d", version, revision, headerLen)

	resetDrawState()

	pos := headerLen
	sign := []byte{0xde, 0xad, 0xbe, 0xef}
	frames := []movieFrame{}
	var lastFrame int32 = -1
	for pos+12 <= len(data) {
		if binary.BigEndian.Uint32(data[pos:pos+4]) != movieSignature {
			idx := bytes.Index(data[pos:], sign)
			if idx < 0 {
				break
			}
			pos += idx
			continue
		}
		frame := int32(binary.BigEndian.Uint32(data[pos+4 : pos+8]))
		if lastFrame >= 0 && frame != lastFrame+1 {
			logDebug("movie frame gap: %d -> %d", lastFrame, frame)
		}
		lastFrame = frame
		size := int(binary.BigEndian.Uint16(data[pos+8 : pos+10]))
		flags := binary.BigEndian.Uint16(data[pos+10 : pos+12])
		pos += 12
		preStart := pos
		if flags&flagGameState != 0 {
			if pos+24 <= len(data) {
				maxSize := int(binary.BigEndian.Uint32(data[pos+12 : pos+16]))
				start := pos + 24
				end := start + maxSize
				// Sanity check: maxSize must fit and be reasonable.
				if maxSize >= 0 && end <= len(data) {
					parseGameState(data[start:end], version, uint16(revision))
					pos = end
				}
			}
		}
		if flags&flagMobileData != 0 {
			// Only attempt mobile table parse if a sentinel exists ahead.
			if bytes.Contains(data[pos:], []byte{0xff, 0xff, 0xff, 0xff}) {
				pos = parseMobileTable(data, pos, version, uint16(revision))
			}
		}
		if flags&flagPictureTable != 0 {
			if pos+2 <= len(data) {
				count := int(binary.BigEndian.Uint16(data[pos : pos+2]))
				size := 2 + 6*count + 4
				if count >= 0 && size >= 0 && pos+size <= len(data) && count <= 8192 {
					pos += 2
					pics := make([]framePicture, 0, count)
					for i := 0; i < count && pos+6 <= len(data); i++ {
						id := binary.BigEndian.Uint16(data[pos : pos+2])
						h := int16(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
						v := int16(binary.BigEndian.Uint16(data[pos+4 : pos+6]))
						plane := 0
						if clImages != nil {
							plane = clImages.Plane(uint32(id))
						}
						pos += 6
						pics = append(pics, framePicture{PictID: id, H: h, V: v, Plane: plane})
					}
					if pos+4 <= len(data) {
						pos += 4
					}
					stateMu.Lock()
					state.pictures = pics
					stateMu.Unlock()
				}
			}
		}
		preData := append([]byte(nil), data[preStart:pos]...)
		if size > 0 {
			if pos+size > len(data) {
				break
			}
			frames = append(frames, movieFrame{data: append([]byte(nil), data[pos:pos+size]...), index: frame, flags: flags, preData: preData})
			pos += size
		} else {
			frames = append(frames, movieFrame{index: frame, flags: flags, preData: preData})
			idx := bytes.Index(data[pos:], sign)
			if idx < 0 {
				break
			}
			pos += idx
		}
	}
	stateMu.Lock()
	initialState = cloneDrawState(state)
	stateMu.Unlock()
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
		handleInfoText(gs[:i])
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
				plane := 0
				if clImages != nil {
					plane = clImages.Plane(uint32(id))
				}
				pos += 6
				pics = append(pics, framePicture{PictID: id, H: h, V: v, Plane: plane})
			}
			if pos+4 <= len(gs) {
				pos += 4
			}
			stateMu.Lock()
			// Preserve on-disk ordering for pictAgain semantics.
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

type mobileTableLayout struct {
	descSize            int
	typeOffset          int
	bubbleTypeOffset    int
	colorsOffset        int
	nameOffset          int
	numColorsOffset     int
	bubbleCounterOffset int
	bubbleLocOffset     int
}

func layoutForMobileTable(version uint16) (mobileTableLayout, bool) {
	switch {
	case version > 141: // v142+ (current format)
		return mobileTableLayout{descSize: 156, typeOffset: 16, bubbleTypeOffset: 20, colorsOffset: 56, nameOffset: 86, numColorsOffset: 48, bubbleCounterOffset: 28, bubbleLocOffset: 52}, true
	case version > 113: // v114-141
		return mobileTableLayout{descSize: 150, typeOffset: 16, bubbleTypeOffset: 20, colorsOffset: 52, nameOffset: 82, numColorsOffset: 44, bubbleCounterOffset: 24, bubbleLocOffset: 48}, true
	case version > 105: // v106-113
		return mobileTableLayout{descSize: 142, typeOffset: 16, bubbleTypeOffset: 20, colorsOffset: 52, nameOffset: 82, numColorsOffset: 44, bubbleCounterOffset: 24, bubbleLocOffset: 48}, true
	case version > 97: // v98-105
		return mobileTableLayout{descSize: 130, typeOffset: 16, bubbleTypeOffset: 20, colorsOffset: 40, nameOffset: 70, numColorsOffset: 32, bubbleCounterOffset: 24, bubbleLocOffset: 36}, true
	case version >= 80: // v80-97
		return mobileTableLayout{descSize: 126, typeOffset: 12, bubbleTypeOffset: 16, colorsOffset: 36, nameOffset: 66, numColorsOffset: 28, bubbleCounterOffset: 20, bubbleLocOffset: 32}, true
	default:
		return mobileTableLayout{}, false
	}
}

// parseMobileTable decodes the descriptor table for a frame.  Descriptor
// layouts have changed many times over Clan Lord's long history; the version
// checks below mirror the Mac client's ReadMobileTable/Read1Descriptor logic.
// Version breakpoints correspond to kOldestMovieVersion and friends in the
// original source.
func parseMobileTable(data []byte, pos int, version, revision uint16) int {
	const descTableSize = 266 // kDescTableSize

	l, ok := layoutForMobileTable(version)
	if !ok {
		logDebug("unsupported mobile table version %d", version)
		return pos
	}
	stateMu.Lock()
	state.bubbles = state.bubbles[:0]
	stateMu.Unlock()

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
		d.Type = uint8(binary.BigEndian.Uint32(buf[l.typeOffset : l.typeOffset+4]))
		pict := binary.BigEndian.Uint32(buf[0:4])
		if pict == 0xffffffff || uint16(pict) == 0xffff {
			d.PictID = picturelessDescriptorFallbackPictID
		} else {
			d.PictID = uint16(pict)
		}
		if clImages != nil {
			d.Plane = clImages.Plane(uint32(d.PictID))
		}

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
			d.Name = decodeServerText(nameBytes[:i])
		} else {
			d.Name = decodeServerText(nameBytes)
		}

		bubbleCounter := int32(binary.BigEndian.Uint32(buf[l.bubbleCounterOffset : l.bubbleCounterOffset+4]))
		var restoredBubble *bubble
		if bubbleCounter != 0 {
			if pos+2 > len(data) {
				return len(data)
			}
			lgt := int(binary.BigEndian.Uint16(data[pos : pos+2]))
			pos += 2
			if pos+lgt > len(data) {
				return len(data)
			}
			text := decodeServerText(data[pos : pos+lgt])
			pos += lgt
			if text != "" && bubbleCounter > 0 {
				typ := int(int32(binary.BigEndian.Uint32(buf[l.bubbleTypeOffset : l.bubbleTypeOffset+4])))
				b := bubble{Index: uint8(idx), Text: text, Type: typ, CreatedFrame: frameCounter, LifeFrames: int(bubbleCounter)}
				switch typ & kBubbleTypeMask {
				case kBubbleRealAction, kBubblePlayerAction, kBubbleNarrate:
					b.NoArrow = true
				}
				if !hasMobile {
					b.Far = true
					b.NoArrow = true
					b.DedupeID = uint16(idx) + 1
					if idx >= 256 {
						b.Index = 255
					}
					b.H = int16(binary.BigEndian.Uint16(buf[l.bubbleLocOffset : l.bubbleLocOffset+2]))
					b.V = int16(binary.BigEndian.Uint16(buf[l.bubbleLocOffset+2 : l.bubbleLocOffset+4]))
				}
				restoredBubble = &b
			}
		}

		stateMu.Lock()
		if hasMobile {
			if state.mobiles == nil {
				state.mobiles = make(map[uint8]frameMobile)
			}
			state.mobiles[mob.Index] = mob
		}
		if idx < 256 {
			if state.descriptors == nil {
				state.descriptors = make(map[uint8]frameDescriptor)
			}
			state.descriptors[d.Index] = d
		}
		if restoredBubble != nil {
			state.bubbles = append(state.bubbles, *restoredBubble)
		}
		stateMu.Unlock()

		// Update the Players list appearance immediately from descriptor data,
		// mirroring live behavior so movies show avatars right away.
		if idx < 256 {
			updatePlayerAppearance(d.Name, d.PictID, d.Colors, d.Type == kDescNPC)
			queueInfoRequest(d.Name)
		}
	}
	return pos
}
