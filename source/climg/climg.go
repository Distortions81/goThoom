package climg

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"log"
	"math"
	"os"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
)

type dataLocation struct {
	offset       uint32
	size         uint32
	entryType    uint32
	id           uint32
	colorBytes   []byte
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
	sizeOnce       sync.Once
	width          uint16
	height         uint16
	visibleSizeMu  sync.Mutex
	visibleSizeSet bool
	visibleWidth   uint16
	visibleHeight  uint16
}

type CLImages struct {
	data             []byte
	locations        []dataLocation
	itemValues       []ClientItem
	idrefs           map[uint32]*dataLocation
	colors           map[uint32]*dataLocation
	images           map[uint32]*dataLocation
	lights           map[uint32]*dataLocation
	items            map[uint32]*ClientItem
	cache            map[string]*ebiten.Image
	lightInfos       map[uint32]LightInfo
	masks            map[alphaMaskKey]*AlphaMask
	mu               sync.Mutex
	Denoise          bool
	DenoiseSharpness float64
	DenoiseAmount    float64
	processingMu     sync.RWMutex
	gammaMu          sync.RWMutex
	gammaEnabled     bool
	spriteGamma      float64
	monitorGamma     float64
	gammaLUT         []uint8
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

	// PictDefFlagRandomAnimation selects a random animation-table entry for
	// each picture instance instead of advancing every instance in lockstep.
	PictDefFlagRandomAnimation = 0x0004
	// PictDefIsShadow marks explicit ground-shadow artwork controlled by the
	// server's shadow and night levels.
	PictDefIsShadow = 0x1000
)

func Load(path string) (*CLImages, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseCLImages(data)
}

// LoadBytes parses the CL_Images keyfile from an in-memory byte slice.
func LoadBytes(data []byte) (*CLImages, error) { return parseCLImages(data) }

func parseCLImages(data []byte) (*CLImages, error) {
	if len(data) < 12 {
		return nil, io.ErrUnexpectedEOF
	}
	if binary.BigEndian.Uint16(data[0:2]) != 0xffff {
		return nil, fmt.Errorf("bad header")
	}
	entryCount64 := uint64(binary.BigEndian.Uint32(data[2:6]))
	if entryCount64 > uint64((len(data)-12)/16) {
		return nil, io.ErrUnexpectedEOF
	}
	entryCount := int(entryCount64)
	table := data[12 : 12+entryCount*16]

	var idrefCount, colorCount, imageCount, lightCount, itemCount int
	for offset := 0; offset < len(table); offset += 16 {
		switch binary.BigEndian.Uint32(table[offset+8 : offset+12]) {
		case TYPE_IDREF:
			idrefCount++
		case TYPE_COLOR:
			colorCount++
		case TYPE_IMAGE:
			imageCount++
		case TYPE_LIGHT:
			lightCount++
		case TYPE_CLIENT_ITEM:
			itemCount++
		}
	}

	imgs := &CLImages{
		data:       data,
		locations:  make([]dataLocation, entryCount),
		itemValues: make([]ClientItem, itemCount),
		idrefs:     make(map[uint32]*dataLocation, idrefCount),
		colors:     make(map[uint32]*dataLocation, colorCount),
		images:     make(map[uint32]*dataLocation, imageCount),
		lights:     make(map[uint32]*dataLocation, lightCount),
		items:      make(map[uint32]*ClientItem, itemCount),
		cache:      make(map[string]*ebiten.Image),
		lightInfos: make(map[uint32]LightInfo),
	}

	nextItem := 0
	for i := range imgs.locations {
		offset := i * 16
		dl := &imgs.locations[i]
		dl.offset = binary.BigEndian.Uint32(table[offset : offset+4])
		dl.size = binary.BigEndian.Uint32(table[offset+4 : offset+8])
		dl.entryType = binary.BigEndian.Uint32(table[offset+8 : offset+12])
		dl.id = binary.BigEndian.Uint32(table[offset+12 : offset+16])
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
			item := &imgs.itemValues[nextItem]
			nextItem++
			item._loc = dl
			imgs.items[dl.id] = item
		}
	}

	// populate IDREF data
	var loadErr error
	for _, ref := range imgs.idrefs {
		resource := clampedResource(data, ref)
		if len(resource) < 4 {
			loadErr = io.ErrUnexpectedEOF
			log.Printf("climg: truncated idref %d", ref.id)
			continue
		}
		ref.version = binary.BigEndian.Uint32(resource[0:4])
		if len(resource) < 8 {
			loadErr = io.ErrUnexpectedEOF
			log.Printf("climg: truncated idref %d", ref.id)
			continue
		}
		ref.imageID = binary.BigEndian.Uint32(resource[4:8])
		if len(resource) < 12 {
			loadErr = io.ErrUnexpectedEOF
			log.Printf("climg: truncated idref %d", ref.id)
			continue
		}
		ref.colorID = binary.BigEndian.Uint32(resource[8:12])
		position := 12
		readUint32 := func(target *uint32) {
			if position+4 <= len(resource) {
				*target = binary.BigEndian.Uint32(resource[position : position+4])
				position += 4
			}
		}
		readUint32(&ref.checksum)
		readUint32(&ref.flags)
		readUint32(&ref.unusedFlags)
		readUint32(&ref.unusedFlags2)
		if position+4 <= len(resource) {
			ref.lightingID = int32(binary.BigEndian.Uint32(resource[position : position+4]))
			position += 4
		}
		if position+2 <= len(resource) {
			ref.plane = int16(binary.BigEndian.Uint16(resource[position : position+2]))
			position += 2
		}
		if position+2 <= len(resource) {
			ref.numFrames = binary.BigEndian.Uint16(resource[position : position+2])
			position += 2
		}
		if position+2 <= len(resource) {
			ref.numAnims = int16(binary.BigEndian.Uint16(resource[position : position+2]))
			position += 2
		}
		for i := 0; i < len(ref.animFrameTable) && position+2 <= len(resource); i++ {
			value := int16(binary.BigEndian.Uint16(resource[position : position+2]))
			if int16(i) < ref.numAnims {
				ref.animFrameTable[i] = value
			}
			position += 2
		}

		if ref.lightingID != 0 {
			if l := imgs.lights[uint32(ref.lightingID)]; l != nil && l.size >= 8 {
				resource := clampedResource(data, l)
				if len(resource) >= 8 {
					li := LightInfo{Radius: binary.BigEndian.Uint16(resource[4:6]), Plane: int16(binary.BigEndian.Uint16(resource[6:8]))}
					copy(li.Color[:], resource[:4])
					imgs.lightInfos[ref.id] = li
				}
			}
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
	for _, it := range imgs.items {
		if it == nil || it._loc == nil {
			continue
		}
		resource := clampedResource(data, it._loc)
		if len(resource) < 20 {
			continue
		}
		// Read up to kMaxItemNameLen (256) bytes for name, but tolerate
		// shorter records by reading whatever remains.
		nameBytes := resource[20:min(len(resource), 20+256)]
		if i := bytes.IndexByte(nameBytes, 0); i >= 0 {
			nameBytes = nameBytes[:i]
		}
		*it = ClientItem{
			Flags:           binary.BigEndian.Uint32(resource[0:4]),
			Slot:            int(int32(binary.BigEndian.Uint32(resource[4:8]))),
			RightHandPictID: binary.BigEndian.Uint32(resource[8:12]),
			LeftHandPictID:  binary.BigEndian.Uint32(resource[12:16]),
			WornPictID:      binary.BigEndian.Uint32(resource[16:20]),
			Name:            string(nameBytes),
		}
	}

	// preload colors
	for _, c := range imgs.colors {
		start := uint64(c.offset)
		end := start + uint64(c.size)
		if end > uint64(len(data)) {
			return nil, io.ErrUnexpectedEOF
		}
		c.colorBytes = data[int(start):int(end)]
	}
	return imgs, loadErr
}

func clampedResource(data []byte, location *dataLocation) []byte {
	start := uint64(location.offset)
	if start >= uint64(len(data)) {
		return nil
	}
	end := start + uint64(location.size)
	if end > uint64(len(data)) {
		end = uint64(len(data))
	}
	return data[int(start):int(end)]
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

	img := c.DecodeRGBA(id, custom, forceTransparent)
	if img == nil {
		return nil
	}
	c.processingMu.RLock()
	denoise, denoiseSharpness, denoiseAmount := c.Denoise, c.DenoiseSharpness, c.DenoiseAmount
	c.processingMu.RUnlock()
	if denoise {
		denoiseImage(img, denoiseSharpness, denoiseAmount)
	}

	eimg := newManagedImageFromImage(img)
	c.mu.Lock()
	c.cache[key] = eimg
	c.mu.Unlock()
	return eimg
}

// DecodeRGBA expands one indexed CL_Images sheet into premultiplied high-color
// pixels without applying denoise or creating a GPU image. Callers that process
// individual frames or mobile poses can therefore do all CPU work before the
// first Ebitengine upload.
func (c *CLImages) DecodeRGBA(id uint32, custom []byte, forceTransparent bool) *image.RGBA {

	ref := c.idrefs[id]
	if ref == nil {
		return nil
	}
	imgLoc := c.images[ref.imageID]
	colLoc := c.colors[ref.colorID]
	if imgLoc == nil || colLoc == nil {
		return nil
	}

	data, width, height, err := decodeIndexedImage(c.data, imgLoc)
	if err != nil {
		log.Printf("decode image %d: %v", id, err)
		return nil
	}
	indexedPixels := data
	defer releaseIndexedPixels(indexedPixels)

	// prepare color table and handle custom palette row if present
	pal := palette // from palette.go
	col := colLoc.colorBytes
	var customColorTable [256]byte

	var mapping []byte
	if ref.flags&pictDefCustomColors != 0 {
		if len(data) >= width {
			mapping = data[:width]
			data = data[width:]
			height--
		}
		if len(custom) > 0 {
			// The archive palette is shared by every decode. Copy only when a
			// customized mobile is actually going to modify it.
			if len(col) <= len(customColorTable) {
				copy(customColorTable[:], col)
				col = customColorTable[:len(col)]
			} else {
				col = append([]byte(nil), col...)
			}
			applyCustomPalette(col, mapping, custom)
		}
	}
	// Add a 1 pixel transparent border around the decoded image.
	img := image.NewRGBA(image.Rect(0, 0, width+2, height+2))

	// Determine alpha level and transparency handling based on
	// sprite definition flags. Some assets (like mobiles) rely on
	// index 0 being transparent even without the explicit flag, so
	// allow callers to force this behavior.
	alpha, _ := alphaTransparentForFlags(ref.flags)

	c.gammaMu.RLock()
	gammaEnabled := c.gammaEnabled
	gammaLUT := c.gammaLUT
	c.gammaMu.RUnlock()

	pix := img.Pix
	stride := img.Stride
	var rgbaTable [256]uint32
	for paletteIndex, idx := range col {
		if paletteIndex >= len(rgbaTable) {
			break
		}
		paletteOffset := int(idx) * 3
		r := uint8(pal[paletteOffset])
		g := uint8(pal[paletteOffset+1])
		b := uint8(pal[paletteOffset+2])
		a := alpha
		// Treat palette index 0 as fully transparent universally. The
		// legacy client consistently uses index 0 for transparency,
		// even on assets without the explicit transparent flag.
		if idx == 0 {
			a = 0
		}
		if gammaEnabled && len(gammaLUT) == 256 {
			r = gammaLUT[r]
			g = gammaLUT[g]
			b = gammaLUT[b]
		}
		// Ebiten expects premultiplied alpha values.
		r = uint8(int(r) * int(a) / 255)
		g = uint8(int(g) * int(a) / 255)
		b = uint8(int(b) * int(a) / 255)
		rgbaTable[paletteIndex] = uint32(r) | uint32(g)<<8 | uint32(b)<<16 | uint32(a)<<24
	}
	writePackedRGBA(pix, stride, data, width, height, &rgbaTable)
	// Normal artwork preparation decodes the unmodified palette first. Record
	// the per-frame opaque footprint here so callers deciding whether a sprite
	// is small never have to inspect pixels in the render path.
	if len(custom) == 0 {
		c.cacheVisibleFrameSize(ref, img)
	}

	return img
}

func writePackedRGBA(pix []byte, stride int, data []byte, width, height int, rgbaTable *[256]uint32) {
	position := 0
	for y := 0; y < height; y++ {
		offset := (y+1)*stride + 4
		x := 0
		// Four indexes at a time amortizes loop control while keeping the odd
		// and two-pixel tails compact for narrow artwork.
		for ; x+3 < width; x += 4 {
			rgbaPair0 := uint64(rgbaTable[data[position]]) | uint64(rgbaTable[data[position+1]])<<32
			rgbaPair1 := uint64(rgbaTable[data[position+2]]) | uint64(rgbaTable[data[position+3]])<<32
			binary.LittleEndian.PutUint64(pix[offset:offset+8], rgbaPair0)
			binary.LittleEndian.PutUint64(pix[offset+8:offset+16], rgbaPair1)
			position += 4
			offset += 16
		}
		for ; x+1 < width; x += 2 {
			rgbaPair := uint64(rgbaTable[data[position]]) | uint64(rgbaTable[data[position+1]])<<32
			binary.LittleEndian.PutUint64(pix[offset:offset+8], rgbaPair)
			position += 2
			offset += 8
		}
		if x < width {
			binary.LittleEndian.PutUint32(pix[offset:offset+4], rgbaTable[data[position]])
			position++
		}
	}
}

// VisibleFrameSize returns the largest non-transparent bounding-box width and
// height found in any one animation frame. Transparent sheet padding and the
// combined height of vertically stacked animation frames are not included.
// The result is measured during normal CPU artwork decoding and cached. If the
// artwork has not been decoded yet, this method performs that work once rather
// than requiring a GPU readback.
func (c *CLImages) VisibleFrameSize(id uint32) (int, int) {
	ref := c.idrefs[id]
	if ref == nil {
		return 0, 0
	}
	ref.visibleSizeMu.Lock()
	if ref.visibleSizeSet {
		w, h := ref.visibleWidth, ref.visibleHeight
		ref.visibleSizeMu.Unlock()
		return int(w), int(h)
	}
	ref.visibleSizeMu.Unlock()

	// DecodeRGBA records the measurement before returning. More than one
	// concurrent first request may decode, but only the first result is stored;
	// subsequent calls are a pair of cached integer reads.
	if c.DecodeRGBA(id, nil, false) == nil {
		return 0, 0
	}
	ref.visibleSizeMu.Lock()
	w, h := ref.visibleWidth, ref.visibleHeight
	ref.visibleSizeMu.Unlock()
	return int(w), int(h)
}

func (c *CLImages) cacheVisibleFrameSize(ref *dataLocation, pixels *image.RGBA) {
	if ref == nil || pixels == nil {
		return
	}
	ref.visibleSizeMu.Lock()
	defer ref.visibleSizeMu.Unlock()
	if ref.visibleSizeSet {
		return
	}
	frames := max(1, int(ref.numFrames))
	w, h := visibleFrameSize(pixels, frames)
	ref.visibleWidth = uint16(w)
	ref.visibleHeight = uint16(h)
	ref.visibleSizeSet = true
}

func visibleFrameSize(pixels *image.RGBA, frames int) (int, int) {
	if pixels == nil {
		return 0, 0
	}
	bounds := pixels.Bounds()
	innerWidth := bounds.Dx() - 2
	innerHeight := bounds.Dy() - 2
	frames = max(1, frames)
	if innerWidth <= 0 || innerHeight < frames {
		return 0, 0
	}
	frameHeight := innerHeight / frames
	maxWidth, maxHeight := 0, 0
	for frame := 0; frame < frames; frame++ {
		frameRect := image.Rect(
			bounds.Min.X+1,
			bounds.Min.Y+1+frame*frameHeight,
			bounds.Min.X+1+innerWidth,
			bounds.Min.Y+1+(frame+1)*frameHeight,
		)
		minX, minY := frameRect.Max.X, frameRect.Max.Y
		maxX, maxY := frameRect.Min.X, frameRect.Min.Y
		for y := frameRect.Min.Y; y < frameRect.Max.Y; y++ {
			row := pixels.Pix[pixels.PixOffset(frameRect.Min.X, y):pixels.PixOffset(frameRect.Max.X, y)]
			for x, offset := frameRect.Min.X, 3; offset < len(row); x, offset = x+1, offset+4 {
				if row[offset] == 0 {
					continue
				}
				minX = min(minX, x)
				minY = min(minY, y)
				maxX = max(maxX, x+1)
				maxY = max(maxY, y+1)
			}
		}
		if maxX > minX && maxY > minY {
			maxWidth = max(maxWidth, maxX-minX)
			maxHeight = max(maxHeight, maxY-minY)
		}
	}
	return maxWidth, maxHeight
}

// SetDenoise configures decoded-artwork filtering without racing background
// precache workers that may be decoding another image at the same time.
func (c *CLImages) SetDenoise(enabled bool, sharpness, amount float64) {
	c.processingMu.Lock()
	c.Denoise = enabled
	c.DenoiseSharpness = sharpness
	c.DenoiseAmount = amount
	c.processingMu.Unlock()
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

// SetGammaCorrection configures sprite gamma compensation.
func (c *CLImages) SetGammaCorrection(enabled bool, spriteGamma, monitorGamma float64) {
	appliedSprite := spriteGamma
	if appliedSprite <= 0 {
		appliedSprite = 2.2
	}
	appliedMonitor := monitorGamma
	if appliedMonitor <= 0 {
		appliedMonitor = 2.2
	}
	c.gammaMu.Lock()
	c.gammaEnabled = enabled
	c.spriteGamma = appliedSprite
	c.monitorGamma = appliedMonitor
	if !enabled {
		c.gammaLUT = nil
		c.gammaMu.Unlock()
		return
	}
	lut := make([]uint8, 256)
	invMonitor := 1.0 / appliedMonitor
	for i := 0; i < 256; i++ {
		x := float64(i) / 255.0
		var y float64
		switch {
		case x <= 0:
			y = 0
		case x >= 1:
			y = 1
		default:
			linear := math.Pow(x, appliedSprite)
			y = math.Pow(linear, invMonitor)
		}
		if y < 0 {
			y = 0
		} else if y > 1 {
			y = 1
		}
		lut[i] = uint8(math.Round(y * 255))
	}
	c.gammaLUT = lut
	c.gammaMu.Unlock()
}

// GammaCorrectChannel applies the configured sprite gamma correction to one
// color channel. Non-sprite drawing can use this to match decoded artwork.
func (c *CLImages) GammaCorrectChannel(value uint8) uint8 {
	if c == nil {
		return value
	}
	c.gammaMu.RLock()
	defer c.gammaMu.RUnlock()
	if !c.gammaEnabled || len(c.gammaLUT) != 256 {
		return value
	}
	return c.gammaLUT[value]
}

// FrameIndex returns the picture frame for the given global animation counter.
// If no animation is defined for the image, it returns 0.
func (c *CLImages) FrameIndex(id uint32, counter int) int {
	return c.FrameIndexForInstance(id, counter, 0)
}

// FrameIndexForInstance returns the picture frame for an animation counter and
// picture instance. Pictures marked for random animation use instanceKey to
// keep separate copies from animating in lockstep while remaining stable for
// repeated draws of the same logical frame.
func (c *CLImages) FrameIndexForInstance(id uint32, counter int, instanceKey uint64) int {
	if counter < 0 {
		return 0
	}
	ref := c.idrefs[id]
	if ref == nil || ref.numFrames <= 1 {
		return 0
	}
	if ref.numAnims > 0 {
		af := counter % int(ref.numAnims)
		if ref.flags&PictDefFlagRandomAnimation != 0 {
			seed := uint64(id)<<32 ^ instanceKey ^ uint64(counter)*0x9e3779b97f4a7c15
			af = int(mixAnimationSeed(seed) % uint64(ref.numAnims))
		}
		pf := int(ref.animFrameTable[af])
		if pf >= 0 && pf < int(ref.numFrames) {
			return pf
		}
		return 0
	}
	return counter % int(ref.numFrames)
}

func mixAnimationSeed(value uint64) uint64 {
	value += 0x9e3779b97f4a7c15
	value = (value ^ (value >> 30)) * 0xbf58476d1ce4e5b9
	value = (value ^ (value >> 27)) * 0x94d049bb133111eb
	return value ^ (value >> 31)
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
	imgLoc.sizeOnce.Do(func() {
		off := uint64(imgLoc.offset)
		if off+4 > uint64(len(c.data)) {
			return
		}
		start := int(off)
		imgLoc.height = binary.BigEndian.Uint16(c.data[start : start+2])
		imgLoc.width = binary.BigEndian.Uint16(c.data[start+2 : start+4])
	})
	return int(imgLoc.width), int(imgLoc.height)
}

// IsSemiTransparent reports whether the sprite with the given ID uses a blend
// mode that results in a base alpha below full opacity. Missing IDs or sprites
// without blend flags return false.
func (c *CLImages) IsSemiTransparent(id uint32) bool {
	if ref := c.idrefs[id]; ref != nil {
		alpha, _ := alphaTransparentForFlags(ref.flags)
		return alpha < 0xFF
	}
	return false
}

// NonTransparentPixels returns the number of pixels with non-zero alpha for
// the specified image ID. It decodes the image data directly from the archive
// to avoid GPU readbacks.
func (c *CLImages) NonTransparentPixels(id uint32) int {
	ref := c.idrefs[id]
	if ref == nil {
		return 0
	}
	imgLoc := c.images[ref.imageID]
	colLoc := c.colors[ref.colorID]
	if imgLoc == nil || colLoc == nil {
		return 0
	}
	data, width, _, err := decodeIndexedImage(c.data, imgLoc)
	if err != nil {
		log.Printf("decode image %d: %v", id, err)
		return 0
	}
	indexedPixels := data
	defer releaseIndexedPixels(indexedPixels)

	if ref.flags&pictDefCustomColors != 0 && len(data) >= width {
		data = data[width:]
	}

	col := colLoc.colorBytes
	count := 0
	for _, idx := range data {
		if col[idx] != 0 {
			count++
		}
	}
	return count
}

// HasOpaqueRect reports whether any non-transparent pixels exist within the
// specified rectangle of the image identified by id. The rectangle coordinates
// are relative to the top-left corner of the sprite.
func (c *CLImages) HasOpaqueRect(id uint32, rect image.Rectangle) bool {
	ref := c.idrefs[id]
	if ref == nil {
		return false
	}
	imgLoc := c.images[ref.imageID]
	colLoc := c.colors[ref.colorID]
	if imgLoc == nil || colLoc == nil {
		return false
	}
	data, width, height, err := decodeIndexedImage(c.data, imgLoc)
	if err != nil {
		log.Printf("decode image %d: %v", id, err)
		return false
	}
	indexedPixels := data
	defer releaseIndexedPixels(indexedPixels)
	rect = rect.Intersect(image.Rect(0, 0, width, height))
	if rect.Empty() {
		return false
	}

	if ref.flags&pictDefCustomColors != 0 && len(data) >= width {
		data = data[width:]
	}

	col := colLoc.colorBytes
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		row := y * width
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if col[data[row+x]] != 0 {
				return true
			}
		}
	}
	return false
}

// applyCustomPalette replaces entries in col according to mapping and custom.
// mapping holds color table indices for each customizable slot while custom
// provides the new palette indices supplied by the server for those slots.
func applyCustomPalette(col []byte, mapping []byte, custom []byte) {
	for i := 0; i < len(custom) && i < len(mapping); i++ {
		idx := int(mapping[i])
		if idx >= 0 && idx < len(col) {
			col[idx] = custom[i]
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

// Flags returns the raw PictDef flags for the given image ID. If the ID is
// unknown, it returns 0.
func (c *CLImages) Flags(id uint32) uint32 {
	if ref := c.idrefs[id]; ref != nil {
		return ref.flags
	}
	return 0
}

// Lighting returns lighting metadata for the given image ID. The bool result
// reports whether lighting information was found.
func (c *CLImages) Lighting(id uint32) (LightInfo, bool) {
	li, ok := c.lightInfos[id]
	return li, ok
}

// IDs returns all image identifiers present in the archive.
func (c *CLImages) IDs() []uint32 {
	ids := make([]uint32, 0, len(c.idrefs))
	for id := range c.idrefs {
		ids = append(ids, id)
	}
	return ids
}
