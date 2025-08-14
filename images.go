package main

import (
	"image"
	"log"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"

	"gothoom/climg"
)

// imageCache lazily loads images from the CL_Images archive. If an image is not
// present, nil is cached to avoid repeated lookups.
const maxColors = 30

type imageKey struct {
	id    uint16
	frame uint16
}

type sheetKey struct {
	id               uint16
	forceTransparent bool
	colorsLen        uint8
	colors           [maxColors]byte
}

type mobileKey struct {
	id        uint16
	state     uint8
	colorsLen uint8
	colors    [maxColors]byte
}

type mobileBlendKey struct {
	from  mobileKey
	to    mobileKey
	step  uint8
	total uint8
}

type pictBlendKey struct {
	id    uint16
	from  uint16
	to    uint16
	step  uint8
	total uint8
}

var (
	// imageCache holds cropped animation frames keyed by picture ID and
	// frame index.
	imageCache = make(map[imageKey]*ebiten.Image)
	// sheetCache holds the full sprite sheet for a picture ID and optional
	// custom color palette. The key combines the picture ID with the custom
	// color bytes so tinted versions are cached separately.
	sheetCache = make(map[sheetKey]*ebiten.Image)
	// mobileCache caches individual mobile frames keyed by picture ID,
	// state, and color overrides.
	mobileCache = make(map[mobileKey]*ebiten.Image)
	// mobileBlendCache stores pre-rendered blended mobile frames.
	mobileBlendCache = make(map[mobileBlendKey]*ebiten.Image)
	// pictBlendCache stores pre-rendered blended picture frames.
	pictBlendCache = make(map[pictBlendKey]*ebiten.Image)

	imageMu  sync.Mutex
	clImages *climg.CLImages
)

// excludedSpriteIDs holds sprite/picture IDs that should never be drawn.
// Populate this map with IDs to exclude at runtime as needed.
var excludedSpriteIDs = map[uint16]struct{}{}

func isSpriteExcluded(id uint16) bool {
	_, ok := excludedSpriteIDs[id]
	return ok
}

func makeSheetKey(id uint16, colors []byte, forceTransparent bool) sheetKey {
	var k sheetKey
	k.id = id
	k.forceTransparent = forceTransparent
	if len(colors) > 0 {
		l := len(colors)
		if l > maxColors {
			l = maxColors
		}
		k.colorsLen = uint8(l)
		copy(k.colors[:], colors[:l])
	}
	return k
}

func makeImageKey(id uint16, frame int) imageKey {
	return imageKey{id: id, frame: uint16(frame)}
}

func makeMobileKey(id uint16, state uint8, colors []byte) mobileKey {
	var k mobileKey
	k.id = id
	k.state = state
	if len(colors) > 0 {
		l := len(colors)
		if l > maxColors {
			l = maxColors
		}
		k.colorsLen = uint8(l)
		copy(k.colors[:], colors[:l])
	}
	return k
}

// loadSheet retrieves the full sprite sheet for the specified picture ID.
// The forceTransparent flag forces palette index 0 to be fully transparent
// regardless of the pictDef flags. Mobile sprites require this behavior
// since the original client always treats index 0 as transparent for them.
func loadSheet(id uint16, colors []byte, forceTransparent bool) *ebiten.Image {
	// Respect explicit exclusion list
	if isSpriteExcluded(id) {
		return nil
	}
	key := makeSheetKey(id, colors, forceTransparent)
	if !gs.NoCaching {
		imageMu.Lock()
		if img, ok := sheetCache[key]; ok {
			imageMu.Unlock()
			return img
		}
		imageMu.Unlock()
	}

	if clImages != nil {
		if img := clImages.Get(uint32(id), colors, forceTransparent); img != nil {
			statImageLoaded(id)
			if gs.NoCaching {
				return img
			}
			imageMu.Lock()
			sheetCache[key] = img
			imageMu.Unlock()
			return img
		}
		log.Printf("missing image %d", id)
	} else {
		log.Printf("CL_Images not loaded when requesting image %d", id)
	}

	return nil
}

// loadImage retrieves the first frame for the specified picture ID. Images are
// cached after the first load to avoid reopening files each frame.
func loadImage(id uint16) *ebiten.Image {
	return loadImageFrame(id, 0)
}

// loadImageFrame retrieves a specific animation frame for the specified picture
// ID. Frames are cached individually after the first load.
func loadImageFrame(id uint16, frame int) *ebiten.Image {
	// Respect explicit exclusion list
	if isSpriteExcluded(id) {
		return nil
	}
	origKey := makeImageKey(id, frame)
	if !gs.NoCaching {
		imageMu.Lock()
		if img, ok := imageCache[origKey]; ok {
			imageMu.Unlock()
			return img
		}
		imageMu.Unlock()
	}

	sheet := loadSheet(id, nil, false)
	if sheet == nil {
		if !gs.NoCaching {
			imageMu.Lock()
			imageCache[origKey] = nil
			imageMu.Unlock()
		}
		return nil
	}

	frames := 1
	if clImages != nil {
		frames = clImages.NumFrames(uint32(id))
	}
	if frames <= 0 {
		frames = 1
	}
	frame = frame % frames
	innerHeight := sheet.Bounds().Dy() - 2
	innerWidth := sheet.Bounds().Dx() - 2
	h := innerHeight / frames

	// Filter out animated sprites smaller than 32x32
	if frames > 1 {
		if innerWidth < 32 || h < 32 {
			if !gs.NoCaching {
				imageMu.Lock()
				imageCache[origKey] = nil
				imageMu.Unlock()
			}
			return nil
		}
	}

	if gs.cacheWholeSheet && !gs.NoCaching {
		imageMu.Lock()
		for f := 0; f < frames; f++ {
			k := makeImageKey(id, f)
			if _, ok := imageCache[k]; !ok {
				y := 1 + f*h
				imageCache[k] = sheet.SubImage(image.Rect(1, y, 1+innerWidth, y+h)).(*ebiten.Image)
			}
		}
		img := imageCache[makeImageKey(id, frame)]
		imageMu.Unlock()
		return img
	}

	y0 := frame * h
	sub := sheet.SubImage(image.Rect(1, 1+y0, 1+innerWidth, 1+y0+h)).(*ebiten.Image)

	if !gs.NoCaching {
		imageMu.Lock()
		imageCache[makeImageKey(id, frame)] = sub
		imageMu.Unlock()
	}
	return sub
}

// loadMobileFrame retrieves a cropped frame from a mobile sprite sheet based on
// the state value provided by the server. The optional colors slice allows
// caller-supplied palette overrides to be cached separately.
func loadMobileFrame(id uint16, state uint8, colors []byte) *ebiten.Image {
	baseKey := makeMobileKey(id, 0, colors)
	key := baseKey
	key.state = state
	if !gs.NoCaching {
		imageMu.Lock()
		if img, ok := mobileCache[key]; ok {
			imageMu.Unlock()
			return img
		}
		imageMu.Unlock()
	}

	sheet := loadSheet(id, colors, true)
	if sheet == nil {
		if !gs.NoCaching {
			imageMu.Lock()
			mobileCache[key] = nil
			imageMu.Unlock()
		}
		return nil
	}

	innerSize := (sheet.Bounds().Dx() - 2) / 16
	x := 1 + int(state&0x0F)*innerSize
	y := 1 + int(state>>4)*innerSize
	if x+innerSize > sheet.Bounds().Dx()-1 || y+innerSize > sheet.Bounds().Dy()-1 {
		if !gs.NoCaching {
			imageMu.Lock()
			mobileCache[key] = nil
			imageMu.Unlock()
		}
		return nil
	}

	if gs.cacheWholeSheet && !gs.NoCaching {
		imageMu.Lock()
		for yy := 0; yy < 16; yy++ {
			for xx := 0; xx < 16; xx++ {
				k := baseKey
				k.state = uint8(yy<<4 | xx)
				if _, ok := mobileCache[k]; !ok {
					sx := 1 + xx*innerSize
					sy := 1 + yy*innerSize
					if sx+innerSize <= sheet.Bounds().Dx()-1 && sy+innerSize <= sheet.Bounds().Dy()-1 {
						mobileCache[k] = sheet.SubImage(image.Rect(sx, sy, sx+innerSize, sy+innerSize)).(*ebiten.Image)
					} else {
						mobileCache[k] = nil
					}
				}
			}
		}
		img := mobileCache[key]
		imageMu.Unlock()
		return img
	}

	frame := sheet.SubImage(image.Rect(x, y, x+innerSize, y+innerSize)).(*ebiten.Image)
	if !gs.NoCaching {
		imageMu.Lock()
		mobileCache[key] = frame
		imageMu.Unlock()
	}
	return frame
}

// mobileSize returns the dimension of a single mobile frame for the given
// image ID. If the image cannot be loaded, 0 is returned.
func mobileSize(id uint16) int {
	sheet := loadSheet(id, nil, true)
	if sheet == nil {
		return 0
	}
	return (sheet.Bounds().Dx() - 2) / 16
}

func mobileBlendFrame(from, to mobileKey, prevImg, img *ebiten.Image, step, total int) *ebiten.Image {
	if prevImg == nil || img == nil {
		return nil
	}
	k := mobileBlendKey{from: from, to: to, step: uint8(step), total: uint8(total)}
	if !gs.NoCaching {
		imageMu.Lock()
		if b, ok := mobileBlendCache[k]; ok {
			imageMu.Unlock()
			return b
		}
		imageMu.Unlock()
	}

	size := img.Bounds().Dx()
	if s := prevImg.Bounds().Dx(); s > size {
		size = s
	}
	blended := newImage(size, size)
	alpha := float32(step) / float32(total)
	offPrev := (size - prevImg.Bounds().Dx()) / 2
	op1 := &ebiten.DrawImageOptions{}
	op1.ColorScale.ScaleAlpha(1 - alpha)
	op1.Blend = ebiten.BlendCopy
	op1.GeoM.Translate(float64(offPrev), float64(offPrev))
	blended.DrawImage(prevImg, op1)
	offCur := (size - img.Bounds().Dx()) / 2
	op2 := &ebiten.DrawImageOptions{}
	op2.ColorScale.ScaleAlpha(alpha)
	op2.Blend = ebiten.BlendLighter
	op2.GeoM.Translate(float64(offCur), float64(offCur))
	blended.DrawImage(img, op2)
	if !gs.NoCaching {
		imageMu.Lock()
		mobileBlendCache[k] = blended
		imageMu.Unlock()
	}
	return blended
}

func pictBlendFrame(id uint16, fromFrame, toFrame int, prevImg, img *ebiten.Image, step, total int) *ebiten.Image {
	if prevImg == nil || img == nil {
		return nil
	}
	k := pictBlendKey{id: id, from: uint16(fromFrame), to: uint16(toFrame), step: uint8(step), total: uint8(total)}
	if !gs.NoCaching {
		imageMu.Lock()
		if b, ok := pictBlendCache[k]; ok {
			imageMu.Unlock()
			return b
		}
		imageMu.Unlock()
	}

	w1, h1 := prevImg.Bounds().Dx(), prevImg.Bounds().Dy()
	w2, h2 := img.Bounds().Dx(), img.Bounds().Dy()
	w := w1
	if w2 > w {
		w = w2
	}
	h := h1
	if h2 > h {
		h = h2
	}
	blended := newImage(w, h)
	alpha := float32(step) / float32(total)
	offPrevX := (w - w1) / 2
	offPrevY := (h - h1) / 2
	op1 := &ebiten.DrawImageOptions{}
	op1.ColorScale.ScaleAlpha(1 - alpha)
	op1.Blend = ebiten.BlendCopy
	op1.GeoM.Translate(float64(offPrevX), float64(offPrevY))
	blended.DrawImage(prevImg, op1)
	offCurX := (w - w2) / 2
	offCurY := (h - h2) / 2
	op2 := &ebiten.DrawImageOptions{}
	op2.ColorScale.ScaleAlpha(alpha)
	op2.Blend = ebiten.BlendLighter
	op2.GeoM.Translate(float64(offCurX), float64(offCurY))
	blended.DrawImage(img, op2)
	if !gs.NoCaching {
		imageMu.Lock()
		pictBlendCache[k] = blended
		imageMu.Unlock()
	}
	return blended
}

// imageCacheStats returns the counts and approximate memory usage in bytes for
// each of the image caches: sheetCache, imageCache, and mobileCache.
func imageCacheStats() (sheetCount, sheetBytes, frameCount, frameBytes, mobileCount, mobileBytes, mobileBlendCount, mobileBlendBytes, pictBlendCount, pictBlendBytes int) {
	imageMu.Lock()
	defer imageMu.Unlock()

	for _, img := range sheetCache {
		if img != nil {
			sheetCount++
			b := img.Bounds()
			sheetBytes += b.Dx() * b.Dy() * 4
		}
	}
	for _, img := range imageCache {
		if img != nil {
			frameCount++
			b := img.Bounds()
			frameBytes += b.Dx() * b.Dy() * 4
		}
	}
	for _, img := range mobileCache {
		if img != nil {
			mobileCount++
			b := img.Bounds()
			mobileBytes += b.Dx() * b.Dy() * 4
		}
	}
	for _, img := range mobileBlendCache {
		if img != nil {
			mobileBlendCount++
			b := img.Bounds()
			mobileBlendBytes += b.Dx() * b.Dy() * 4
		}
	}
	for _, img := range pictBlendCache {
		if img != nil {
			pictBlendCount++
			b := img.Bounds()
			pictBlendBytes += b.Dx() * b.Dy() * 4
		}
	}
	return
}
