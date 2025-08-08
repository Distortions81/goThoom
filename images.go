package main

import (
	"fmt"
	"image"
	"log"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"

	"go_client/climg"
)

// imageCache lazily loads images from the CL_Images archive. If an image is not
// present, nil is cached to avoid repeated lookups.
var (
	// imageCache holds cropped animation frames keyed by picture ID and
	// frame index.
	imageCache = make(map[string]*ebiten.Image)
	// sheetCache holds the full sprite sheet for a picture ID and optional
	// custom color palette. The key combines the picture ID with the custom
	// color bytes so tinted versions are cached separately.
	sheetCache = make(map[string]*ebiten.Image)
	// mobileCache caches individual mobile frames keyed by picture ID,
	// state, and color overrides.
	mobileCache = make(map[string]*ebiten.Image)

	imageMu  sync.Mutex
	clImages *climg.CLImages
)

// loadSheet retrieves the full sprite sheet for the specified picture ID.
// The forceTransparent flag forces palette index 0 to be fully transparent
// regardless of the pictDef flags. Mobile sprites require this behavior
// since the original client always treats index 0 as transparent for them.
func loadSheet(id uint16, colors []byte, forceTransparent bool) *ebiten.Image {
	key := fmt.Sprintf("%d-%x-%t", id, colors, forceTransparent)
	imageMu.Lock()
	if img, ok := sheetCache[key]; ok {
		imageMu.Unlock()
		return img
	}
	imageMu.Unlock()

	if clImages != nil {
		if img := clImages.Get(uint32(id), colors, forceTransparent); img != nil {
			imageMu.Lock()
			sheetCache[key] = img
			imageMu.Unlock()
			return img
		}
		log.Printf("missing image %d", id)
	} else {
		log.Printf("CL_Images not loaded when requesting image %d", id)
	}

	imageMu.Lock()
	sheetCache[key] = nil
	imageMu.Unlock()
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
	imageMu.Lock()
	if img, ok := imageCache[fmt.Sprintf("%d-%d", id, frame)]; ok {
		imageMu.Unlock()
		return img
	}
	imageMu.Unlock()

	sheet := loadSheet(id, nil, false)
	if sheet == nil {
		imageMu.Lock()
		imageCache[fmt.Sprintf("%d-%d", id, frame)] = nil
		imageMu.Unlock()
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

	if gs.cacheWholeSheet {
		imageMu.Lock()
		for f := 0; f < frames; f++ {
			k := fmt.Sprintf("%d-%d", id, f)
			if _, ok := imageCache[k]; !ok {
				y := 1 + f*h
				imageCache[k] = sheet.SubImage(image.Rect(1, y, 1+innerWidth, y+h)).(*ebiten.Image)
			}
		}
		img := imageCache[fmt.Sprintf("%d-%d", id, frame)]
		imageMu.Unlock()
		return img
	}

	y0 := frame * h
	sub := sheet.SubImage(image.Rect(1, 1+y0, 1+innerWidth, 1+y0+h)).(*ebiten.Image)

	imageMu.Lock()
	imageCache[fmt.Sprintf("%d-%d", id, frame)] = sub
	imageMu.Unlock()
	return sub
}

// loadMobileFrame retrieves a cropped frame from a mobile sprite sheet based on
// the state value provided by the server. The optional colors slice allows
// caller-supplied palette overrides to be cached separately.
func loadMobileFrame(id uint16, state uint8, colors []byte) *ebiten.Image {
	imageMu.Lock()
	if img, ok := mobileCache[fmt.Sprintf("%d-%d-%x", id, state, colors)]; ok {
		imageMu.Unlock()
		return img
	}
	imageMu.Unlock()

	sheet := loadSheet(id, colors, true)
	if sheet == nil {
		imageMu.Lock()
		mobileCache[fmt.Sprintf("%d-%d-%x", id, state, colors)] = nil
		imageMu.Unlock()
		return nil
	}

	innerSize := (sheet.Bounds().Dx() - 2) / 16
	x := 1 + int(state&0x0F)*innerSize
	y := 1 + int(state>>4)*innerSize
	if x+innerSize > sheet.Bounds().Dx()-1 || y+innerSize > sheet.Bounds().Dy()-1 {
		imageMu.Lock()
		mobileCache[fmt.Sprintf("%d-%d-%x", id, state, colors)] = nil
		imageMu.Unlock()
		return nil
	}

	if gs.cacheWholeSheet {
		imageMu.Lock()
		for yy := 0; yy < 16; yy++ {
			for xx := 0; xx < 16; xx++ {
				k := fmt.Sprintf("%d-%d-%x", id, uint8(yy<<4|xx), colors)
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
		img := mobileCache[fmt.Sprintf("%d-%d-%x", id, state, colors)]
		imageMu.Unlock()
		return img
	}

	frame := sheet.SubImage(image.Rect(x, y, x+innerSize, y+innerSize)).(*ebiten.Image)
	imageMu.Lock()
	mobileCache[fmt.Sprintf("%d-%d-%x", id, state, colors)] = frame
	imageMu.Unlock()
	return frame
}

// imageCacheStats returns the counts and approximate memory usage in bytes for
// each of the image caches: sheetCache, imageCache, and mobileCache.
func imageCacheStats() (sheetCount, sheetBytes, frameCount, frameBytes, mobileCount, mobileBytes int) {
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
	return
}
