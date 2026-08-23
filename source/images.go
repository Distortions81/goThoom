package main

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

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

type scaledImageKey struct {
	imageKey
	scale uint8
	mode  uint8
}

type scaledMobileKey struct {
	mobileKey
	scale uint8
	mode  uint8
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
	// scaledImageCache stores pixel-art upscaled world picture frames.
	scaledImageCache = make(map[scaledImageKey]*ebiten.Image)
	// scaledMobileCache stores pixel-art upscaled mobile frames.
	scaledMobileCache = make(map[scaledMobileKey]*ebiten.Image)

	imageMu  sync.Mutex
	clImages *climg.CLImages

	dumpImgOnce   sync.Once
	dumpImgMu     sync.Mutex
	dumpedImgIDs  = make(map[uint16]struct{})
	imgMetaWriter *csv.Writer

	spriteUpscaleShader *ebiten.Shader
)

//go:embed data/shaders/sprite_upscale.kage
var spriteUpscaleShaderSource []byte

func init() {
	var err error
	spriteUpscaleShader, err = ebiten.NewShader(spriteUpscaleShaderSource)
	if err != nil {
		panic(err)
	}
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
	if id == 0xffff {
		return nil
	}
	key := makeSheetKey(id, colors, forceTransparent)
	imageMu.Lock()
	if img, ok := sheetCache[key]; ok {
		imageMu.Unlock()
		return img
	}
	imageMu.Unlock()

	if clImages != nil {
		var t0 time.Time
		if measureLoads {
			t0 = time.Now()
		}
		if img := clImages.Get(uint32(id), colors, forceTransparent); img != nil {
			statImageLoaded(id)
			if imgDump && colors == nil && !forceTransparent {
				dumpImageSheet(id, img)
			}
			imageMu.Lock()
			sheetCache[key] = img
			imageMu.Unlock()
			if measureLoads {
				frames := clImages.NumFrames(uint32(id))
				if frames <= 0 {
					frames = 1
				}
				innerW := img.Bounds().Dx() - 2
				innerH := img.Bounds().Dy() - 2
				h := 0
				if frames > 0 {
					h = innerH / frames
				}
				dtms := float64(time.Since(t0).Nanoseconds()) / 1e6
				log.Printf("measure: sprite id=%d frames=%d size=%dx%d load=%.2fms frame=%d", id, frames, innerW, h, dtms, frameCounter)
			}
			return img
		}
		log.Printf("missing image %d", id)
	} else {
		log.Printf("CL_Images not loaded when requesting image %d", id)
	}

	return nil
}

func dumpImageSheet(id uint16, sheet *ebiten.Image) {
	if isWASM {
		return
	}
	dumpImgOnce.Do(func() {
		os.MkdirAll(filepath.Join("dump", "img"), 0755)
		if f, err := os.Create(filepath.Join("dump", "img", "metadata.csv")); err == nil {
			imgMetaWriter = csv.NewWriter(f)
			imgMetaWriter.Write([]string{"id", "width", "height", "frames", "flags", "name"})
		}
	})
	dumpImgMu.Lock()
	if _, ok := dumpedImgIDs[id]; ok {
		dumpImgMu.Unlock()
		return
	}
	dumpedImgIDs[id] = struct{}{}
	dumpImgMu.Unlock()

	frames := 1
	if clImages != nil {
		frames = clImages.NumFrames(uint32(id))
	}
	if frames <= 0 {
		frames = 1
	}
	innerHeight := sheet.Bounds().Dy() - 2
	innerWidth := sheet.Bounds().Dx() - 2
	h := innerHeight / frames

	for f := 0; f < frames; f++ {
		y := 1 + f*h
		frameImg := sheet.SubImage(image.Rect(1, y, 1+innerWidth, y+h)).(*ebiten.Image)
		fn := filepath.Join("dump", "img", fmt.Sprintf("%d_%d.png", id, f))
		if file, err := os.Create(fn); err == nil {
			png.Encode(file, frameImg)
			file.Close()
		}
	}

	width, height := innerWidth, h
	var flags uint32
	var name string
	if clImages != nil {
		if it, ok := clImages.Item(uint32(id)); ok {
			flags = it.Flags
			name = it.Name
		}
	}
	if imgMetaWriter != nil {
		imgMetaWriter.Write([]string{
			strconv.Itoa(int(id)),
			strconv.Itoa(width),
			strconv.Itoa(height),
			strconv.Itoa(frames),
			strconv.FormatUint(uint64(flags), 10),
			name,
		})
		imgMetaWriter.Flush()
	}
}

// loadImage retrieves the first frame for the specified picture ID. Images are
// cached after the first load to avoid reopening files each frame.
func loadImage(id uint16) *ebiten.Image {
	return loadImageFrame(id, 0)
}

// loadImageFrame retrieves a specific animation frame for the specified picture
// ID. Frames are cached individually after the first load.
func loadImageFrame(id uint16, frame int) *ebiten.Image {
	origKey := makeImageKey(id, frame)
	imageMu.Lock()
	if img, ok := imageCache[origKey]; ok {
		imageMu.Unlock()
		return img
	}
	imageMu.Unlock()

	sheet := loadSheet(id, nil, false)
	if sheet == nil {
		imageMu.Lock()
		imageCache[origKey] = nil
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

// loadMobileFrame retrieves a cropped frame from a mobile sprite sheet based on
// the state value provided by the server. The optional colors slice allows
// caller-supplied palette overrides to be cached separately.
func loadMobileFrame(id uint16, state uint8, colors []byte) *ebiten.Image {
	baseKey := makeMobileKey(id, 0, colors)
	key := baseKey
	key.state = state
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

	innerSize := (sheet.Bounds().Dx() - 2) / 16
	x := 1 + int(state&0x0F)*innerSize
	y := 1 + int(state>>4)*innerSize
	if x+innerSize > sheet.Bounds().Dx()-1 || y+innerSize > sheet.Bounds().Dy()-1 {
		imageMu.Lock()
		mobileCache[key] = nil
		imageMu.Unlock()
		return nil
	}

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

func upscaleSpriteImage(img *ebiten.Image, factor int) *ebiten.Image {
	if factor <= 1 || img == nil {
		return img
	}
	if factor > 4 {
		return img
	}
	w, h := img.Bounds().Dx()*factor, img.Bounds().Dy()*factor
	nearest := newImage(w, h)
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
	op.GeoM.Scale(float64(factor), float64(factor))
	nearest.DrawImage(img, op)

	scaled := newImage(w, h)
	shaderOp := &ebiten.DrawRectShaderOptions{Uniforms: map[string]any{
		"Scale":         float32(factor),
		"CornerReach":   artworkUpscaleCornerReach(),
		"BlendStrength": artworkUpscaleBlendStrength(),
	}}
	shaderOp.Images[0] = nearest
	scaled.DrawRectShader(w, h, spriteUpscaleShader, shaderOp)
	nearest.Deallocate()
	return scaled
}

const (
	artworkUpscaleOff = iota
	artworkUpscaleCrisp
	artworkUpscaleBalanced
	artworkUpscaleSmooth
	artworkUpscaleUltraSmooth
)

var artworkUpscaleModeNames = []string{"Off", "Crisp", "Balanced", "Smooth", "Ultra Smooth"}

func artworkUpscaleMode() int {
	if !gs.SpriteUpscaleFilter {
		return artworkUpscaleOff
	}
	if gs.SpriteUpscaleMode < artworkUpscaleCrisp || gs.SpriteUpscaleMode > artworkUpscaleUltraSmooth {
		return artworkUpscaleUltraSmooth
	}
	return gs.SpriteUpscaleMode
}

func setArtworkUpscaleMode(mode int) {
	if mode < artworkUpscaleOff || mode > artworkUpscaleUltraSmooth {
		mode = artworkUpscaleUltraSmooth
	}
	gs.SpriteUpscaleMode = mode
	gs.SpriteUpscaleFilter = mode != artworkUpscaleOff
}

func artworkUpscaleEnabled() bool {
	return artworkUpscaleMode() != artworkUpscaleOff
}

func artworkUpscaleCornerReach() float32 {
	switch artworkUpscaleMode() {
	case artworkUpscaleOff:
		return 0
	case artworkUpscaleCrisp:
		return 1.35
	case artworkUpscaleSmooth, artworkUpscaleUltraSmooth:
		return 2.75
	default:
		return 1.65
	}
}

func artworkUpscaleBlendStrength() float32 {
	switch artworkUpscaleMode() {
	case artworkUpscaleOff:
		return 0
	case artworkUpscaleCrisp:
		return 0.65
	case artworkUpscaleSmooth:
		return 1
	case artworkUpscaleUltraSmooth:
		// Keep reconstructed boundary pixels partially covered for an
		// anti-aliased look without sampling outside the sprite.
		return 0.82
	default:
		return 0.8
	}
}

func artworkUpscaleFactor() int {
	factor := spriteUpscaleFactor()
	if gs.PotatoGPU && factor > 2 {
		return 2
	}
	return factor
}

func getScaledPictureFrame(id uint16, frame int, img *ebiten.Image) *ebiten.Image {
	factor := artworkUpscaleFactor()
	if factor <= 1 || img == nil || !artworkUpscaleEnabled() {
		return img
	}
	key := scaledImageKey{imageKey: makeImageKey(id, frame), scale: uint8(factor), mode: uint8(artworkUpscaleMode())}
	imageMu.Lock()
	if cached, ok := scaledImageCache[key]; ok {
		imageMu.Unlock()
		return cached
	}
	imageMu.Unlock()
	scaled := upscaleSpriteImage(img, factor)
	imageMu.Lock()
	scaledImageCache[key] = scaled
	imageMu.Unlock()
	return scaled
}

func getScaledMobileFrame(key mobileKey, img *ebiten.Image) *ebiten.Image {
	factor := artworkUpscaleFactor()
	if factor <= 1 || img == nil || !artworkUpscaleEnabled() {
		return img
	}
	sKey := scaledMobileKey{mobileKey: key, scale: uint8(factor), mode: uint8(artworkUpscaleMode())}
	imageMu.Lock()
	if cached, ok := scaledMobileCache[sKey]; ok {
		imageMu.Unlock()
		return cached
	}
	imageMu.Unlock()
	scaled := upscaleSpriteImage(img, factor)
	imageMu.Lock()
	scaledMobileCache[sKey] = scaled
	imageMu.Unlock()
	return scaled
}

// mobileSize returns the dimension of a single mobile frame for the given
// image ID. If the image cannot be loaded, 0 is returned.
func mobileSize(id uint16) int {
	if clImages == nil {
		return 0
	}
	w, _ := clImages.Size(uint32(id))
	if w <= 0 {
		return 0
	}
	return w / 16
}

func mobileBlendFrame(from, to mobileKey, prevImg, img *ebiten.Image, step, total int) *ebiten.Image {
	if prevImg == nil || img == nil {
		return nil
	}
	k := mobileBlendKey{from: from, to: to, step: uint8(step), total: uint8(total)}
	imageMu.Lock()
	if b, ok := mobileBlendCache[k]; ok {
		imageMu.Unlock()
		return b
	}
	imageMu.Unlock()

	size := img.Bounds().Dx()
	if s := prevImg.Bounds().Dx(); s > size {
		size = s
	}
	blended := newImage(size, size)
	alpha := float32(step) / float32(total)
	offPrev := (size - prevImg.Bounds().Dx()) / 2
	op1 := acquireDrawOpts()
	op1.ColorScale.Reset()
	op1.ColorScale.ScaleAlpha(1 - alpha)
	op1.Blend = ebiten.BlendCopy
	op1.GeoM.Reset()
	op1.GeoM.Translate(float64(offPrev), float64(offPrev))
	blended.DrawImage(prevImg, op1)
	op1.ColorScale.Reset()
	op1.GeoM.Reset()
	op1.Filter = 0
	op1.DisableMipmaps = false
	op1.Blend = ebiten.BlendSourceOver
	releaseDrawOpts(op1)
	offCur := (size - img.Bounds().Dx()) / 2
	op2 := acquireDrawOpts()
	op2.ColorScale.Reset()
	op2.ColorScale.ScaleAlpha(alpha)
	op2.Blend = ebiten.BlendLighter
	op2.GeoM.Reset()
	op2.GeoM.Translate(float64(offCur), float64(offCur))
	blended.DrawImage(img, op2)
	op2.ColorScale.Reset()
	op2.GeoM.Reset()
	op2.Filter = 0
	op2.DisableMipmaps = false
	op2.Blend = ebiten.BlendSourceOver
	releaseDrawOpts(op2)
	imageMu.Lock()
	mobileBlendCache[k] = blended
	imageMu.Unlock()
	return blended
}

func pictBlendFrame(id uint16, fromFrame, toFrame int, prevImg, img *ebiten.Image, step, total int) *ebiten.Image {
	if prevImg == nil || img == nil {
		return nil
	}
	k := pictBlendKey{id: id, from: uint16(fromFrame), to: uint16(toFrame), step: uint8(step), total: uint8(total)}
	imageMu.Lock()
	if b, ok := pictBlendCache[k]; ok {
		imageMu.Unlock()
		return b
	}
	imageMu.Unlock()

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
	op1 := acquireDrawOpts()
	op1.ColorScale.Reset()
	op1.ColorScale.ScaleAlpha(1 - alpha)
	op1.Blend = ebiten.BlendCopy
	op1.GeoM.Reset()
	op1.GeoM.Translate(float64(offPrevX), float64(offPrevY))
	blended.DrawImage(prevImg, op1)
	op1.ColorScale.Reset()
	op1.GeoM.Reset()
	op1.Filter = 0
	op1.DisableMipmaps = false
	op1.Blend = ebiten.BlendSourceOver
	releaseDrawOpts(op1)
	offCurX := (w - w2) / 2
	offCurY := (h - h2) / 2
	op2 := acquireDrawOpts()
	op2.ColorScale.Reset()
	op2.ColorScale.ScaleAlpha(alpha)
	op2.Blend = ebiten.BlendLighter
	op2.GeoM.Reset()
	op2.GeoM.Translate(float64(offCurX), float64(offCurY))
	blended.DrawImage(img, op2)
	op2.ColorScale.Reset()
	op2.GeoM.Reset()
	op2.Filter = 0
	op2.DisableMipmaps = false
	op2.Blend = ebiten.BlendSourceOver
	releaseDrawOpts(op2)
	imageMu.Lock()
	pictBlendCache[k] = blended
	imageMu.Unlock()
	return blended
}

type imageCacheStatsData struct {
	sheetCount        int
	sheetBytes        int
	frameCount        int
	frameBytes        int
	scaledFrameCount  int
	scaledFrameBytes  int
	mobileCount       int
	mobileBytes       int
	scaledMobileCount int
	scaledMobileBytes int
	mobileBlendCount  int
	mobileBlendBytes  int
	pictBlendCount    int
	pictBlendBytes    int
}

// imageCacheStats returns the counts and approximate memory usage in bytes for
// each of the image caches: sheets, cropped frames, scaled variants, and blends.
func imageCacheStats() imageCacheStatsData {
	imageMu.Lock()
	defer imageMu.Unlock()

	var stats imageCacheStatsData
	for _, img := range sheetCache {
		if img != nil {
			stats.sheetCount++
			b := img.Bounds()
			stats.sheetBytes += b.Dx() * b.Dy() * 4
		}
	}
	for _, img := range imageCache {
		if img != nil {
			stats.frameCount++
			b := img.Bounds()
			stats.frameBytes += b.Dx() * b.Dy() * 4
		}
	}
	for _, img := range scaledImageCache {
		if img != nil {
			stats.scaledFrameCount++
			b := img.Bounds()
			stats.scaledFrameBytes += b.Dx() * b.Dy() * 4
		}
	}
	for _, img := range mobileCache {
		if img != nil {
			stats.mobileCount++
			b := img.Bounds()
			stats.mobileBytes += b.Dx() * b.Dy() * 4
		}
	}
	for _, img := range scaledMobileCache {
		if img != nil {
			stats.scaledMobileCount++
			b := img.Bounds()
			stats.scaledMobileBytes += b.Dx() * b.Dy() * 4
		}
	}
	for _, img := range mobileBlendCache {
		if img != nil {
			stats.mobileBlendCount++
			b := img.Bounds()
			stats.mobileBlendBytes += b.Dx() * b.Dy() * 4
		}
	}
	for _, img := range pictBlendCache {
		if img != nil {
			stats.pictBlendCount++
			b := img.Bounds()
			stats.pictBlendBytes += b.Dx() * b.Dy() * 4
		}
	}
	return stats
}
