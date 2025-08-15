package climg

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"log"
	"os"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
)

type dataLocation struct {
	offset       uint32
	size         uint32
	entryType    uint32
	id           uint32
	colorBytes   []uint16
	version      uint32
	imageID      uint32
	colorID      uint32
	checksum     uint32
	flags        uint32
	unusedFlags  uint32
	unusedFlags2 uint32
	lightingID   int32
	plane        int16
	numFrames    uint16

	numAnims       int16
	animFrameTable [16]int16
}

type CLImages struct {
	data             []byte
	idrefs           map[uint32]*dataLocation
	colors           map[uint32]*dataLocation
	images           map[uint32]*dataLocation
	lights           map[uint32]*dataLocation
	items            map[uint32]*ClientItem
	cache            map[string]*ebiten.Image
	mu               sync.Mutex
	Denoise          bool
	DenoiseSharpness float64
	DenoisePercent   float64
}

const (
	TYPE_IDREF = 0x50446635
	TYPE_IMAGE = 0x42697432
	TYPE_COLOR = 0x436c7273
	TYPE_LIGHT = 0x4C697431
	// kTypeClientItemOld4 'CIm4' from DatabaseTypes_cl.h
	TYPE_CLIENT_ITEM = 0x43496d34

	pictDefFlagTransparent = 0x8000
	pictDefBlendMask       = 0x0003
	pictDefCustomColors    = 0x2000
	pictDefFlagNoChecksum  = 0x0400
)

func Load(path string) (*CLImages, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	r := bytes.NewReader(data)
	var header uint16
	var entryCount uint32
	if err := binary.Read(r, binary.BigEndian, &header); err != nil {
		return nil, err
	}
	if header != 0xffff {
		return nil, fmt.Errorf("bad header")
	}
	if err := binary.Read(r, binary.BigEndian, &entryCount); err != nil {
		return nil, err
	}
	var pad1 uint32
	var pad2 uint16
	if err := binary.Read(r, binary.BigEndian, &pad1); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.BigEndian, &pad2); err != nil {
		return nil, err
	}

	imgs := &CLImages{
		data:   data,
		idrefs: make(map[uint32]*dataLocation, entryCount),
		colors: make(map[uint32]*dataLocation, entryCount),
		images: make(map[uint32]*dataLocation, entryCount),
		lights: make(map[uint32]*dataLocation, entryCount),
		items:  make(map[uint32]*ClientItem),
		cache:  make(map[string]*ebiten.Image),
	}

	for i := uint32(0); i < entryCount; i++ {
		dl := &dataLocation{}
		if err := binary.Read(r, binary.BigEndian, &dl.offset); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.BigEndian, &dl.size); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.BigEndian, &dl.entryType); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.BigEndian, &dl.id); err != nil {
			return nil, err
		}
		switch dl.entryType {
		case TYPE_IDREF:
			imgs.idrefs[dl.id] = dl
		case TYPE_COLOR:
			imgs.colors[dl.id] = dl
		case TYPE_IMAGE:
			imgs.images[dl.id] = dl
		case TYPE_LIGHT:
			imgs.lights[dl.id] = dl
		case TYPE_CLIENT_ITEM:
			// store location to parse later
			imgs.items[dl.id] = &ClientItem{_loc: dl}
		}
	}

	// populate IDREF data
	var loadErr error
	for _, ref := range imgs.idrefs {
		start := int64(ref.offset)
		end := start + int64(ref.size)
		if end > int64(len(imgs.data)) {
			end = int64(len(imgs.data))
		}
		sr := io.NewSectionReader(bytes.NewReader(imgs.data), start, end-start)
		remaining := end - start

		// mandatory fields
		if remaining < 4 {
			loadErr = io.ErrUnexpectedEOF
			log.Printf("climg: truncated idref %d", ref.id)
			continue
		}
		if err := binary.Read(sr, binary.BigEndian, &ref.version); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				loadErr = err
				log.Printf("climg: truncated idref %d: %v", ref.id, err)
				continue
			}
			return nil, err
		}
		remaining -= 4

		if remaining < 4 {
			loadErr = io.ErrUnexpectedEOF
			log.Printf("climg: truncated idref %d", ref.id)
			continue
		}
		if err := binary.Read(sr, binary.BigEndian, &ref.imageID); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				loadErr = err
				log.Printf("climg: truncated idref %d: %v", ref.id, err)
				continue
			}
			return nil, err
		}
		remaining -= 4

		if remaining < 4 {
			loadErr = io.ErrUnexpectedEOF
			log.Printf("climg: truncated idref %d", ref.id)
			continue
		}
		if err := binary.Read(sr, binary.BigEndian, &ref.colorID); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				loadErr = err
				log.Printf("climg: truncated idref %d: %v", ref.id, err)
				continue
			}
			return nil, err
		}
		remaining -= 4

		// optional fields
		if remaining >= 4 {
			if err := binary.Read(sr, binary.BigEndian, &ref.checksum); err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					loadErr = err
					log.Printf("climg: truncated idref %d: %v", ref.id, err)
					continue
				}
				return nil, err
			}
			remaining -= 4
		}
		if remaining >= 4 {
			if err := binary.Read(sr, binary.BigEndian, &ref.flags); err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					loadErr = err
					log.Printf("climg: truncated idref %d: %v", ref.id, err)
					continue
				}
				return nil, err
			}
			remaining -= 4
		}
		if remaining >= 4 {
			if err := binary.Read(sr, binary.BigEndian, &ref.unusedFlags); err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					loadErr = err
					log.Printf("climg: truncated idref %d: %v", ref.id, err)
					continue
				}
				return nil, err
			}
			remaining -= 4
		}
		if remaining >= 4 {
			if err := binary.Read(sr, binary.BigEndian, &ref.unusedFlags2); err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					loadErr = err
					log.Printf("climg: truncated idref %d: %v", ref.id, err)
					continue
				}
				return nil, err
			}
			remaining -= 4
		}
		if remaining >= 4 {
			if err := binary.Read(sr, binary.BigEndian, &ref.lightingID); err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					loadErr = err
					log.Printf("climg: truncated idref %d: %v", ref.id, err)
					continue
				}
				return nil, err
			}
			remaining -= 4
		}
		if remaining >= 2 {
			if err := binary.Read(sr, binary.BigEndian, &ref.plane); err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					loadErr = err
					log.Printf("climg: truncated idref %d: %v", ref.id, err)
					continue
				}
				return nil, err
			}
			remaining -= 2
		}
		if remaining >= 2 {
			if err := binary.Read(sr, binary.BigEndian, &ref.numFrames); err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					loadErr = err
					log.Printf("climg: truncated idref %d: %v", ref.id, err)
					continue
				}
				return nil, err
			}
			remaining -= 2
		}
		if remaining >= 2 {
			if err := binary.Read(sr, binary.BigEndian, &ref.numAnims); err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					loadErr = err
					log.Printf("climg: truncated idref %d: %v", ref.id, err)
					continue
				}
				return nil, err
			}
			remaining -= 2
		}

		for i := 0; i < 16 && remaining >= 2; i++ {
			var v int16
			if err := binary.Read(sr, binary.BigEndian, &v); err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					loadErr = err
					log.Printf("climg: truncated idref %d: %v", ref.id, err)
				} else {
					return nil, err
				}
				break
			}
			if int16(i) < ref.numAnims {
				ref.animFrameTable[i] = v
			}
			remaining -= 2
		}

		// verify checksum unless disabled
		/*
			bitsLoc := imgs.images[ref.imageID]
			colLoc := imgs.colors[ref.colorID]
			if bitsLoc != nil && colLoc != nil {
				endBits := int(bitsLoc.offset + bitsLoc.size)
				endCols := int(colLoc.offset + colLoc.size)
				if endBits <= len(imgs.data) && endCols <= len(imgs.data) {
					bits := imgs.data[bitsLoc.offset:endBits]
					colors := imgs.data[colLoc.offset:endCols]
					var light []byte
					if ref.lightingID != 0 {
						if l := imgs.lights[uint32(ref.lightingID)]; l != nil {
							endLight := int(l.offset + l.size)
							if endLight <= len(imgs.data) {
								light = imgs.data[l.offset:endLight]
							}
						}
					}
						sum := calculateChecksum(bits, colors, light, ref)
						if ref.checksum != 0 && (ref.flags&pictDefFlagNoChecksum) == 0 && sum != ref.checksum {
							log.Printf("climg: checksum mismatch for idref %d: have %08x want %08x", ref.id, sum, ref.checksum)
							loadErr = fmt.Errorf("climg: checksum mismatch for idref %d", ref.id)
							panic(loadErr)
						}
				}
			} */
	}

	// parse client items (names, slots, pictIDs)
	for id, it := range imgs.items {
		if it == nil || it._loc == nil {
			continue
		}
		start := int64(it._loc.offset)
		end := start + int64(it._loc.size)
		if end > int64(len(imgs.data)) {
			end = int64(len(imgs.data))
		}
		r := io.NewSectionReader(bytes.NewReader(imgs.data), start, end-start)
		var flags uint32
		var slot int32
		var right, left, worn int32
		if err := binary.Read(r, binary.BigEndian, &flags); err != nil {
			continue
		}
		if err := binary.Read(r, binary.BigEndian, &slot); err != nil {
			continue
		}
		if err := binary.Read(r, binary.BigEndian, &right); err != nil {
			continue
		}
		if err := binary.Read(r, binary.BigEndian, &left); err != nil {
			continue
		}
		if err := binary.Read(r, binary.BigEndian, &worn); err != nil {
			continue
		}
		// Read up to kMaxItemNameLen (256) bytes for name, but tolerate
		// shorter records by reading whatever remains.
		nameBytes := make([]byte, 256)
		n, _ := r.Read(nameBytes)
		if n < 0 {
			n = 0
		}
		if n > 256 {
			n = 256
		}
		nameBytes = nameBytes[:n]
		if i := bytes.IndexByte(nameBytes, 0); i >= 0 {
			nameBytes = nameBytes[:i]
		}
		name := string(nameBytes)
		imgs.items[id] = &ClientItem{
			Flags:           flags,
			Slot:            int(slot),
			RightHandPictID: uint32(right),
			LeftHandPictID:  uint32(left),
			WornPictID:      uint32(worn),
			Name:            name,
		}
	}

	// preload colors
	for _, c := range imgs.colors {
		if _, err := r.Seek(int64(c.offset), io.SeekStart); err != nil {
			return nil, err
		}
		c.colorBytes = make([]uint16, c.size)
		for i := 0; i < int(c.size); i++ {
			b, err := r.ReadByte()
			if err != nil {
				return nil, err
			}
			c.colorBytes[i] = uint16(b)
		}
	}
	return imgs, loadErr
}

// ClientItem describes per-item metadata stored in CL_Images (kTypeClientItem).
type ClientItem struct {
	Flags           uint32
	Slot            int
	RightHandPictID uint32
	LeftHandPictID  uint32
	WornPictID      uint32
	Name            string
	_loc            *dataLocation // internal: original location
}

// Item returns the CL_Images metadata for an item id, if present.
func (c *CLImages) Item(id uint32) (ClientItem, bool) {
	if it, ok := c.items[id]; ok && it != nil {
		return *it, true
	}
	return ClientItem{}, false
}

// ItemName returns the public name for an item id, or empty if unknown.
func (c *CLImages) ItemName(id uint32) string {
	if it, ok := c.items[id]; ok && it != nil {
		return it.Name
	}
	return ""
}

// ItemWornPict returns the worn picture ID for an item id, or 0.
func (c *CLImages) ItemWornPict(id uint32) uint32 {
	if it, ok := c.items[id]; ok && it != nil {
		return it.WornPictID
	}
	return 0
}

// ItemRightHandPict returns the right-hand picture ID for an item id, or 0.
func (c *CLImages) ItemRightHandPict(id uint32) uint32 {
	if it, ok := c.items[id]; ok && it != nil {
		return it.RightHandPictID
	}
	return 0
}

// ItemLeftHandPict returns the left-hand picture ID for an item id, or 0.
func (c *CLImages) ItemLeftHandPict(id uint32) uint32 {
	if it, ok := c.items[id]; ok && it != nil {
		return it.LeftHandPictID
	}
	return 0
}

// ItemSlot returns the slot enum for an item id, or 0.
func (c *CLImages) ItemSlot(id uint32) int {
	if it, ok := c.items[id]; ok && it != nil {
		return it.Slot
	}
	return 0
}

// alphaTransparentForFlags returns the base alpha value and whether
// color index 0 should be treated as fully transparent for the given
// sprite flags. The mapping mirrors the original client logic in
// GameWin_cl.cp where specific flag combinations select distinct
// alpha maps.
func alphaTransparentForFlags(flags uint32) (uint8, bool) {
	switch flags & (pictDefFlagTransparent | pictDefBlendMask) {
	case pictDefFlagTransparent:
		return 0xFF, true // kPictDefFlagTransparent
	case 1:
		return 0xBF, false // kPictDef25Blend
	case 2:
		return 0x7F, false // kPictDef50Blend
	case 3:
		return 0x3F, false // kPictDef75Blend
	case pictDefFlagTransparent | 1:
		return 0xBF, true // kPictDefFlagTransparent + kPictDef25Blend
	case pictDefFlagTransparent | 2:
		return 0x7F, true // kPictDefFlagTransparent + kPictDef50Blend
	case pictDefFlagTransparent | 3:
		return 0x3F, true // kPictDefFlagTransparent + kPictDef75Blend
	default:
		return 0xFF, false // kPictDefNoBlend or unknown
	}
}

// Get returns an Ebiten image for the given picture ID. The custom slice
// provides optional palette overrides. If forceTransparent is true, palette
// index 0 is treated as fully transparent regardless of the sprite's
// pictDef flags. The Macintosh client always rendered mobile sprites this
// way, even when the transparency flag wasn't set.
func (c *CLImages) Get(id uint32, custom []byte, forceTransparent bool) *ebiten.Image {
	key := fmt.Sprintf("%d-%x-%t", id, custom, forceTransparent)
	c.mu.Lock()
	if img, ok := c.cache[key]; ok {
		c.mu.Unlock()
		return img
	}
	c.mu.Unlock()

	ref := c.idrefs[id]
	if ref == nil {
		return nil
	}
	imgLoc := c.images[ref.imageID]
	colLoc := c.colors[ref.colorID]
	if imgLoc == nil || colLoc == nil {
		return nil
	}

	r := bytes.NewReader(c.data)
	if _, err := r.Seek(int64(imgLoc.offset), io.SeekStart); err != nil {
		log.Printf("seek image %d: %v", id, err)
		return nil
	}

	var h, w uint16
	var pad uint32
	var v, b byte
	if err := binary.Read(r, binary.BigEndian, &h); err != nil {
		log.Printf("read h for %d: %v", id, err)
		return nil
	}
	if err := binary.Read(r, binary.BigEndian, &w); err != nil {
		log.Printf("read w for %d: %v", id, err)
		return nil
	}
	if err := binary.Read(r, binary.BigEndian, &pad); err != nil {
		log.Printf("read pad for %d: %v", id, err)
		return nil
	}
	if err := binary.Read(r, binary.BigEndian, &v); err != nil {
		log.Printf("read v for %d: %v", id, err)
		return nil
	}
	if err := binary.Read(r, binary.BigEndian, &b); err != nil {
		log.Printf("read b for %d: %v", id, err)
		return nil
	}

	width := int(w)
	height := int(h)
	valueW := int(v)
	blockLenW := int(b)
	pixelCount := width * height
	br := New(r)
	data := make([]byte, pixelCount)
	pixPos := 0
	for pixPos < pixelCount {
		t, err := br.ReadBit()
		if err != nil {
			log.Printf("read bit for %d: %v", id, err)
			return nil
		}
		s, err := br.ReadInt(blockLenW)
		if err != nil {
			log.Printf("read int for %d: %v", id, err)
			return nil
		}
		s++
		if t {
			for i := 0; i < s; i++ {
				val, err := br.ReadBits(valueW)
				if err != nil {
					log.Printf("read bits for %d: %v", id, err)
					return nil
				}
				if pixPos < pixelCount {
					data[pixPos] = val
					pixPos++
				} else {
					break
				}
			}
		} else {
			val, err := br.ReadBits(valueW)
			if err != nil {
				log.Printf("read bits for %d: %v", id, err)
				return nil
			}
			for i := 0; i < s; i++ {
				if pixPos < pixelCount {
					data[pixPos] = val
					pixPos++
				} else {
					break
				}
			}
		}
	}

	// prepare color table and handle custom palette row if present
	pal := palette // from palette.go
	col := append([]uint16(nil), colLoc.colorBytes...)

	var mapping []byte
	if ref.flags&pictDefCustomColors != 0 {
		if len(data) >= width {
			mapping = data[:width]
			data = data[width:]
			height--
		}
		if len(custom) > 0 {
			applyCustomPalette(col, mapping, custom)
		}
	}
	pixelCount = len(data)
	// Add a 1 pixel transparent border around the decoded image.
	img := image.NewRGBA(image.Rect(0, 0, width+2, height+2))

	// Determine alpha level and transparency handling based on
	// sprite definition flags. Some assets (like mobiles) rely on
	// index 0 being transparent even without the explicit flag, so
	// allow callers to force this behavior.
	alpha, transparent := alphaTransparentForFlags(ref.flags)
	if forceTransparent {
		transparent = true
	}

	pix := img.Pix
	stride := img.Stride
	for i := 0; i < pixelCount; i++ {
		idx := col[data[i]]
		r := uint8(pal[idx*3])
		g := uint8(pal[idx*3+1])
		b := uint8(pal[idx*3+2])
		a := alpha
		if idx == 0 && transparent {
			a = 0
		}
		// Ebiten expects premultiplied alpha values.
		r = uint8(int(r) * int(a) / 255)
		g = uint8(int(g) * int(a) / 255)
		b = uint8(int(b) * int(a) / 255)
		off := (i/width+1)*stride + (i%width+1)*4
		pix[off+0] = r
		pix[off+1] = g
		pix[off+2] = b
		pix[off+3] = a
	}

	if c.Denoise {
		denoiseImage(img, c.DenoiseSharpness, c.DenoisePercent)
	}

	eimg := newImageFromImage(img)
	c.mu.Lock()
	c.cache[key] = eimg
	c.mu.Unlock()
	return eimg
}

// NumFrames returns the number of animation frames for the given image ID.
// If unknown, it returns 1.
func (c *CLImages) NumFrames(id uint32) int {
	if ref := c.idrefs[id]; ref != nil && ref.numFrames > 0 {
		return int(ref.numFrames)
	}
	return 1
}

// ClearCache removes all cached images so they will be reloaded on demand.
func (c *CLImages) ClearCache() {
	c.mu.Lock()
	for _, img := range c.cache {
		img.Deallocate()
	}
	c.cache = make(map[string]*ebiten.Image)
	c.mu.Unlock()
}

// FrameIndex returns the picture frame for the given global animation counter.
// If no animation is defined for the image, it returns 0.
func (c *CLImages) FrameIndex(id uint32, counter int) int {
	if counter < 0 {
		return 0
	}
	ref := c.idrefs[id]
	if ref == nil || ref.numFrames <= 1 {
		return 0
	}
	if ref.numAnims > 0 {
		af := counter % int(ref.numAnims)
		pf := int(ref.animFrameTable[af])
		if pf >= 0 && pf < int(ref.numFrames) {
			return pf
		}
		return 0
	}
	return counter % int(ref.numFrames)
}

// Size returns the width and height of the image with the given ID.
// If the image is missing, zeros are returned.
func (c *CLImages) Size(id uint32) (int, int) {
	ref := c.idrefs[id]
	if ref == nil {
		return 0, 0
	}
	imgLoc := c.images[ref.imageID]
	if imgLoc == nil {
		return 0, 0
	}
	r := bytes.NewReader(c.data)
	if _, err := r.Seek(int64(imgLoc.offset), io.SeekStart); err != nil {
		return 0, 0
	}
	var h, w uint16
	if err := binary.Read(r, binary.BigEndian, &h); err != nil {
		return 0, 0
	}
	if err := binary.Read(r, binary.BigEndian, &w); err != nil {
		return 0, 0
	}
	return int(w), int(h)
}

// applyCustomPalette replaces entries in col according to mapping and custom.
// mapping holds color table indices for each customizable slot while custom
// provides the new palette indices supplied by the server for those slots.
func applyCustomPalette(col []uint16, mapping []byte, custom []byte) {
	for i := 0; i < len(custom) && i < len(mapping); i++ {
		idx := int(mapping[i])
		if idx >= 0 && idx < len(col) {
			col[idx] = uint16(custom[i])
		}
	}
}

// Plane returns the drawing plane for the given image ID. If unknown, it
// returns 0.
func (c *CLImages) Plane(id uint32) int {
	if ref := c.idrefs[id]; ref != nil {
		return int(ref.plane)
	}
	return 0
}

// IDs returns all image identifiers present in the archive.
func (c *CLImages) IDs() []uint32 {
	ids := make([]uint32, 0, len(c.idrefs))
	for id := range c.idrefs {
		ids = append(ids, id)
	}
	return ids
}
