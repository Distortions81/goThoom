package main

import (
	"fmt"
	"image"
	"image/color"
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

	// bubbleCache caches bubble sprites keyed by picture ID and type.
	bubbleCache = make(map[string]*ebiten.Image)

	imageMu  sync.Mutex
	clImages *climg.CLImages
)

// addBorder returns a new image with a one pixel transparent border around img.
// This helps avoid texture bleeding when sprites are scaled or filtered.
func addBorder(img *ebiten.Image) *ebiten.Image {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	bordered := ebiten.NewImage(w+2, h+2)
	bordered.Fill(color.RGBA{0, 0, 0, 0})
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(1, 1)
	bordered.DrawImage(img, op)
	return bordered
}

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

// loadBubbleImage retrieves a specific bubble sprite for the given picture ID
// and bubble type. Bubble sprites are stored in picture IDs 1–3 as sprite
// sheets with one column per bubble type. Each cropped sprite is cached after
// the first load.
func loadBubbleImage(id uint16, typ int) *ebiten.Image {
	key := fmt.Sprintf("bubble-%d-%d", id, typ)
	imageMu.Lock()
	if img, ok := bubbleCache[key]; ok {
		imageMu.Unlock()
		return img
	}
	imageMu.Unlock()

	sheet := loadImage(id)
	if sheet == nil {
		imageMu.Lock()
		bubbleCache[key] = nil
		imageMu.Unlock()
		return nil
	}

	var w, h int
	switch id {
	case 1:
		w, h = bubbleSmallWidth, bubbleSmallHeight
	case 2:
		w, h = bubbleMediumWidth, bubbleMediumHeight
	case 3:
		w, h = bubbleLargeWidth, bubbleLargeHeight
	default:
		return nil
	}

	x := 1 + typ*w
	y := 1
	if x+w+1 > sheet.Bounds().Dx() || y+h+1 > sheet.Bounds().Dy() {
		imageMu.Lock()
		bubbleCache[key] = nil
		imageMu.Unlock()
		return nil
	}
	sub := sheet.SubImage(image.Rect(x, y, x+w, y+h)).(*ebiten.Image)
	sub = addBorder(sub)

	imageMu.Lock()
	bubbleCache[key] = sub
	imageMu.Unlock()
	return sub
}

// loadImageFrame retrieves a specific animation frame for the specified picture
// ID. Frames are cached individually after the first load.
func loadImageFrame(id uint16, frame int) *ebiten.Image {
	key := fmt.Sprintf("%d-%d", id, frame)
	imageMu.Lock()
	if img, ok := imageCache[key]; ok {
		imageMu.Unlock()
		return img
	}
	imageMu.Unlock()

	sheet := loadSheet(id, nil, false)
	if sheet == nil {
		imageMu.Lock()
		imageCache[key] = nil
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
	h := sheet.Bounds().Dy() / frames
	y0 := frame * h
	sub := sheet.SubImage(image.Rect(0, y0, sheet.Bounds().Dx(), y0+h)).(*ebiten.Image)
	sub = addBorder(sub)

	imageMu.Lock()
	imageCache[key] = sub
	imageMu.Unlock()
	return sub
}

// loadMobileFrame retrieves a cropped frame from a mobile sprite sheet based on
// the state value provided by the server. The optional colors slice allows
// caller-supplied palette overrides to be cached separately.
func loadMobileFrame(id uint16, state uint8, colors []byte) *ebiten.Image {
	key := fmt.Sprintf("%d-%d-%x", id, state, colors)
	imageMu.Lock()
	if img, ok := mobileCache[key]; ok {
		imageMu.Unlock()
		return img
	}
	imageMu.Unlock()

	sheet := loadSheet(id, colors, true)
	if sheet == nil {
		imageMu.Lock()
		mobileCache[key] = nil
		imageMu.Unlock()
		return nil
	}

	size := sheet.Bounds().Dx() / 16
	x := int(state&0x0F) * size
	y := int(state>>4) * size
	if x+size > sheet.Bounds().Dx() || y+size > sheet.Bounds().Dy() {
		imageMu.Lock()
		mobileCache[key] = nil
		imageMu.Unlock()
		return nil
	}
	frame := sheet.SubImage(image.Rect(x, y, x+size, y+size)).(*ebiten.Image)
	frame = addBorder(frame)
	imageMu.Lock()
	mobileCache[key] = frame
	imageMu.Unlock()
	return frame
}
