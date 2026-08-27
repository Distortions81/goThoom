package main

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"image"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

type scaledPictureBatchKey struct {
	id    uint16
	scale uint8
	mode  uint8
}

type scaledMobileBatchKey struct {
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
	// Completed batch keys avoid rescanning every animation frame or mobile pose
	// on every draw after its first-use upscale batch finishes.
	scaledPictureBatches = make(map[scaledPictureBatchKey]struct{})
	scaledMobileBatches  = make(map[scaledMobileBatchKey]struct{})
	// scaledCacheFactor is the fitted-screen factor represented by all scaled
	// and blended artwork caches. A threshold change replaces these caches.
	scaledCacheFactor uint8

	imageMu sync.Mutex
	// imageCacheLifecycleMu allows image loads to remain concurrent while
	// preventing a cache clear from deallocating a sheet between its decode and
	// insertion into the application caches.
	imageCacheLifecycleMu sync.RWMutex
	clImages              *climg.CLImages

	dumpImgOnce   sync.Once
	dumpImgMu     sync.Mutex
	dumpedImgIDs  = make(map[uint16]struct{})
	imgMetaWriter *csv.Writer

	spriteUpscaleShader    *ebiten.Shader
	spriteUpscaleScratchMu sync.Mutex
	spriteUpscaleScratch   reusableUpscaleScratch
)

// reusableUpscaleScratch owns the standalone nearest-neighbor staging texture
// used by the upscale shader. It only grows, so normal sprite cache misses do
// not allocate and dispose one GPU texture apiece.
type reusableUpscaleScratch struct {
	image *ebiten.Image
}

func (s *reusableUpscaleScratch) region(w, h int) *ebiten.Image {
	if s.image == nil || s.image.Bounds().Dx() < w || s.image.Bounds().Dy() < h {
		newW, newH := w, h
		if s.image != nil {
			newW = max(newW, s.image.Bounds().Dx())
			newH = max(newH, s.image.Bounds().Dy())
			s.image.Deallocate()
		}
		s.image = newUnmanagedImage(newW, newH)
	}
	return s.image.SubImage(image.Rect(0, 0, w, h)).(*ebiten.Image)
}

func (s *reusableUpscaleScratch) deallocate() {
	if s.image != nil {
		s.image.Deallocate()
		s.image = nil
	}
}

//go:embed data/shaders/sprite_upscale.kage
var spriteUpscaleShaderSource []byte

// ReloadSpriteUpscaleShader recompiles the artwork-upscale shader.
func ReloadSpriteUpscaleShader() error {
	shader, err := ebiten.NewShader(spriteUpscaleShaderSource)
	if err != nil {
		return err
	}
	spriteUpscaleShader = shader
	return nil
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
	if replacementEffectReplacesPict(id) {
		return nil
	}
	imageCacheLifecycleMu.RLock()
	defer imageCacheLifecycleMu.RUnlock()
	key := makeSheetKey(id, colors, forceTransparent)
	imageMu.Lock()
	if img, ok := sheetCache[key]; ok {
		imageMu.Unlock()
		return img
	}
	imageMu.Unlock()

	if clImages != nil {
		if img := clImages.Get(uint32(id), colors, forceTransparent); img != nil {
			statImageLoaded(id)
			if imgDump && colors == nil && !forceTransparent {
				dumpImageSheet(id, img)
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

func dumpImageSheet(id uint16, sheet *ebiten.Image) {
	if isWASM {
		return
	}
	// png.Encode reads the Ebiten image pixels. Initial asset loading happens
	// before RunGame, when Ebiten deliberately rejects ReadPixels, so keep the
	// sheet alive and export it after the first game update has initialized the
	// graphics context.
	if !gameHasStarted() {
		// dump-all mode iterates the complete archive from Game.Update after the
		// graphics context is ready, so there is nothing to defer here.
		if assetDumpMode() {
			return
		}
		go func() {
			<-gameStarted
			dumpImageSheet(id, sheet)
		}()
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
			img := frameImg
			if imgDumpScale > 1 {
				mode, _ := imageDumpUpscaleMode(imgDumpScaleType)
				img = upscaleTransientSpriteImageWithMode(frameImg, imgDumpScale, mode)
			}
			png.Encode(file, img)
			file.Close()
			if img != frameImg {
				img.Deallocate()
			}
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
	if replacementEffectReplacesPict(id) {
		return nil
	}
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
	if replacementEffectReplacesPict(id) {
		return nil
	}
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
	return upscaleSpriteImageWithMode(img, factor, artworkUpscaleMode())
}

func upscaleSpriteImageWithMode(img *ebiten.Image, factor, mode int) *ebiten.Image {
	return upscaleSpriteImageWithModeAndLifetime(img, factor, mode, true)
}

// upscaleTransientSpriteImageWithMode is for exports and diagnostics whose
// result is explicitly discarded instead of entering an application cache.
func upscaleTransientSpriteImageWithMode(img *ebiten.Image, factor, mode int) *ebiten.Image {
	return upscaleSpriteImageWithModeAndLifetime(img, factor, mode, false)
}

func upscaleCachedSpriteImage(img *ebiten.Image, factor int) *ebiten.Image {
	if img == nil {
		return nil
	}
	source := readImageRGBA(img)
	return upscaleCachedSpriteRegionCPU(source, source.Bounds(), factor, artworkUpscaleMode())
}

func upscaleCachedSpriteRegionCPU(source *image.RGBA, sourceRect image.Rectangle, factor, mode int) *ebiten.Image {
	return newManagedImageFromImage(upscaleSpriteRegionCPU(source, sourceRect, factor, mode))
}

type upscaleRGBA struct {
	r float32
	g float32
	b float32
	a float32
}

func readImageRGBA(img *ebiten.Image) *image.RGBA {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	pixels := make([]byte, w*h*4)
	img.ReadPixels(pixels)
	return &image.RGBA{Pix: pixels, Stride: w * 4, Rect: image.Rect(0, 0, w, h)}
}

func rgbaPixelAt(img *image.RGBA, x, y int) upscaleRGBA {
	offset := img.PixOffset(x, y)
	return upscaleRGBA{
		r: float32(img.Pix[offset]),
		g: float32(img.Pix[offset+1]),
		b: float32(img.Pix[offset+2]),
		a: float32(img.Pix[offset+3]),
	}
}

func absFloat32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func upscaleColorDistance(a, b upscaleRGBA) float32 {
	aa := max(a.a, 1)
	ba := max(b.a, 1)
	dr := absFloat32(a.r/aa - b.r/ba)
	dg := absFloat32(a.g/aa - b.g/ba)
	db := absFloat32(a.b/aa - b.b/ba)
	luma := dr*0.299 + dg*0.587 + db*0.114
	chroma := max(dr, max(dg, db)) - min(dr, min(dg, db))
	return absFloat32(a.a-b.a)/255*1.5 + luma*0.75 + chroma*0.25
}

func mixUpscaleColor(a, b upscaleRGBA, amount float32) upscaleRGBA {
	return upscaleRGBA{
		r: a.r + (b.r-a.r)*amount,
		g: a.g + (b.g-a.g)*amount,
		b: a.b + (b.b-a.b)*amount,
		a: a.a + (b.a-a.a)*amount,
	}
}

func averageUpscaleColor(a, b upscaleRGBA) upscaleRGBA {
	return upscaleRGBA{
		r: (a.r + b.r) * 0.5,
		g: (a.g + b.g) * 0.5,
		b: (a.b + b.b) * 0.5,
		a: (a.a + b.a) * 0.5,
	}
}

func upscaleByte(v float32) byte {
	return byte(min(255, max(0, int(v+0.5))))
}

// upscaleSpriteRegionCPU is the CPU equivalent of sprite_upscale.kage. It
// operates on one frame or pose so neighbor sampling remains clamped to that
// frame's boundaries.
func upscaleSpriteRegionCPU(source *image.RGBA, sourceRect image.Rectangle, factor, mode int) *image.RGBA {
	sourceRect = sourceRect.Intersect(source.Bounds())
	if sourceRect.Empty() || factor < 1 {
		return image.NewRGBA(image.Rectangle{})
	}
	destination := image.NewRGBA(image.Rect(0, 0, sourceRect.Dx()*factor, sourceRect.Dy()*factor))
	reach := artworkUpscaleCornerReachForMode(mode)
	strength := artworkUpscaleBlendStrengthForMode(mode)
	for sy := sourceRect.Min.Y; sy < sourceRect.Max.Y; sy++ {
		for sx := sourceRect.Min.X; sx < sourceRect.Max.X; sx++ {
			center := rgbaPixelAt(source, sx, sy)
			top := rgbaPixelAt(source, sx, max(sourceRect.Min.Y, sy-1))
			left := rgbaPixelAt(source, max(sourceRect.Min.X, sx-1), sy)
			right := rgbaPixelAt(source, min(sourceRect.Max.X-1, sx+1), sy)
			bottom := rgbaPixelAt(source, sx, min(sourceRect.Max.Y-1, sy+1))
			edgeCrosses := upscaleColorDistance(top, bottom) > 0.07 && upscaleColorDistance(left, right) > 0.07
			topLeft := edgeCrosses && upscaleColorDistance(left, top) < 0.16
			topRight := edgeCrosses && upscaleColorDistance(top, right) < 0.16
			bottomLeft := edgeCrosses && upscaleColorDistance(left, bottom) < 0.16
			bottomRight := edgeCrosses && upscaleColorDistance(bottom, right) < 0.16
			for oy := 0; oy < factor; oy++ {
				localY := (float32(oy) + 0.5) / float32(factor)
				for ox := 0; ox < factor; ox++ {
					localX := (float32(ox) + 0.5) / float32(factor)
					target := center
					weight := float32(0)
					switch {
					case localX < 0.5 && localY < 0.5 && topLeft:
						target = averageUpscaleColor(left, top)
						weight = max(0, min(1, reach-2*(localX+localY)))
					case localX >= 0.5 && localY < 0.5 && topRight:
						target = averageUpscaleColor(top, right)
						weight = max(0, min(1, reach-2*((1-localX)+localY)))
					case localX < 0.5 && localY >= 0.5 && bottomLeft:
						target = averageUpscaleColor(left, bottom)
						weight = max(0, min(1, reach-2*(localX+(1-localY))))
					case localX >= 0.5 && localY >= 0.5 && bottomRight:
						target = averageUpscaleColor(bottom, right)
						weight = max(0, min(1, reach-2*((1-localX)+(1-localY))))
					}
					result := mixUpscaleColor(center, target, weight*strength)
					dx := (sx-sourceRect.Min.X)*factor + ox
					dy := (sy-sourceRect.Min.Y)*factor + oy
					offset := destination.PixOffset(dx, dy)
					destination.Pix[offset] = upscaleByte(result.r)
					destination.Pix[offset+1] = upscaleByte(result.g)
					destination.Pix[offset+2] = upscaleByte(result.b)
					destination.Pix[offset+3] = upscaleByte(result.a)
				}
			}
		}
	}
	return destination
}

func upscaleSpriteImageWithModeAndLifetime(img *ebiten.Image, factor, mode int, managed bool) *ebiten.Image {
	if factor <= 1 || img == nil {
		return img
	}
	if factor > 4 {
		return img
	}
	w, h := img.Bounds().Dx()*factor, img.Bounds().Dy()*factor
	newOutput := newUnmanagedImage
	if managed {
		newOutput = newManagedImage
	}
	if mode == artworkUpscaleOff || spriteUpscaleShader == nil {
		nearest := newOutput(w, h)
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
		op.GeoM.Scale(float64(factor), float64(factor))
		nearest.DrawImage(img, op)
		return nearest
	}

	spriteUpscaleScratchMu.Lock()
	defer spriteUpscaleScratchMu.Unlock()
	nearest := spriteUpscaleScratch.region(w, h)
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
	op.Blend = ebiten.BlendCopy
	op.GeoM.Scale(float64(factor), float64(factor))
	nearest.DrawImage(img, op)

	scaled := newOutput(w, h)
	shaderOp := &ebiten.DrawRectShaderOptions{Uniforms: map[string]any{
		"Scale":         float32(factor),
		"CornerReach":   artworkUpscaleCornerReachForMode(mode),
		"BlendStrength": artworkUpscaleBlendStrengthForMode(mode),
	}}
	shaderOp.Images[0] = nearest
	scaled.DrawRectShader(w, h, spriteUpscaleShader, shaderOp)
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
	return artworkUpscaleCornerReachForMode(artworkUpscaleMode())
}

func artworkUpscaleCornerReachForMode(mode int) float32 {
	switch mode {
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
	return artworkUpscaleBlendStrengthForMode(artworkUpscaleMode())
}

func artworkUpscaleBlendStrengthForMode(mode int) float32 {
	switch mode {
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

func imageDumpUpscaleMode(name string) (int, bool) {
	switch strings.ToLower(name) {
	case "nearest":
		return artworkUpscaleOff, true
	case "crisp":
		return artworkUpscaleCrisp, true
	case "balanced":
		return artworkUpscaleBalanced, true
	case "smooth":
		return artworkUpscaleSmooth, true
	case "ultra-smooth":
		return artworkUpscaleUltraSmooth, true
	default:
		return artworkUpscaleOff, false
	}
}

func artworkUpscaleFactor() int {
	factor := gs.SpriteUpscale
	if factor < 2 {
		factor = 2
	}
	if factor > 4 {
		factor = 4
	}
	return factor
}

// screenCappedArtworkUpscaleFactor limits cached texture resolution to twice
// the sprite's actual fitted size on screen, with a 2x minimum. During
// direct-resolution drawing, gs.GameScale is the fitted window scale rather
// than the configured scale that originally populated SpriteUpscale.
func screenCappedArtworkUpscaleFactor() int {
	factor := artworkUpscaleFactor()
	maxFactor := int(math.Floor(gs.GameScale*2 + 1e-9))
	if maxFactor < 2 {
		maxFactor = 2
	}
	if factor > maxFactor {
		factor = maxFactor
	}
	return factor
}

func ensureScaledArtworkCacheFactorLocked(factor int) {
	if scaledCacheFactor != 0 && scaledCacheFactor != uint8(factor) {
		clearScaledArtworkCachesLocked()
	}
	scaledCacheFactor = uint8(factor)
}

type pendingScaledPictureFrame struct {
	key   scaledImageKey
	image *ebiten.Image
}

// cacheScaledPictureFrames upscales every frame for an animated picture as
// one first-use batch. The outputs remain separate standalone textures, so
// frame boundaries and rendering behavior are unchanged.
func cacheScaledPictureFrames(id uint16, requestedFrame, frameCount, factor, mode int, requestedImage *ebiten.Image) bool {
	if frameCount < 1 {
		frameCount = 1
	}
	requestedFrame %= frameCount
	if requestedFrame < 0 {
		requestedFrame += frameCount
	}

	batchKey := scaledPictureBatchKey{id: id, scale: uint8(factor), mode: uint8(mode)}
	imageMu.Lock()
	ensureScaledArtworkCacheFactorLocked(factor)
	if _, ok := scaledPictureBatches[batchKey]; ok {
		imageMu.Unlock()
		return true
	}
	missing := make([]int, 0, frameCount)
	for frame := 0; frame < frameCount; frame++ {
		key := scaledImageKey{imageKey: makeImageKey(id, frame), scale: uint8(factor), mode: uint8(mode)}
		if _, ok := scaledImageCache[key]; !ok {
			missing = append(missing, frame)
		}
	}
	imageMu.Unlock()
	if len(missing) == 0 {
		imageMu.Lock()
		if scaledCacheFactor == uint8(factor) {
			scaledPictureBatches[batchKey] = struct{}{}
		}
		imageMu.Unlock()
		return true
	}

	pending := make([]pendingScaledPictureFrame, 0, len(missing))
	for _, frame := range missing {
		source := loadImageFrame(id, frame)
		if source == nil && frame == requestedFrame {
			source = requestedImage
		}
		if source == nil {
			continue
		}
		pending = append(pending, pendingScaledPictureFrame{
			key:   scaledImageKey{imageKey: makeImageKey(id, frame), scale: uint8(factor), mode: uint8(mode)},
			image: upscaleSpriteImageWithModeAndLifetime(source, factor, mode, false),
		})
	}

	imageMu.Lock()
	if scaledCacheFactor != uint8(factor) {
		imageMu.Unlock()
		for _, frame := range pending {
			frame.image.Deallocate()
		}
		return false
	}
	for _, frame := range pending {
		if _, ok := scaledImageCache[frame.key]; ok {
			frame.image.Deallocate()
			continue
		}
		scaledImageCache[frame.key] = frame.image
	}
	complete := true
	for frame := 0; frame < frameCount; frame++ {
		key := scaledImageKey{imageKey: makeImageKey(id, frame), scale: uint8(factor), mode: uint8(mode)}
		if _, ok := scaledImageCache[key]; !ok {
			complete = false
			break
		}
	}
	if complete {
		scaledPictureBatches[batchKey] = struct{}{}
	}
	imageMu.Unlock()
	return true
}

func getScaledPictureFrame(id uint16, frame int, img *ebiten.Image) *ebiten.Image {
	if img == nil || !artworkUpscaleEnabled() {
		return img
	}
	for {
		factor := screenCappedArtworkUpscaleFactor()
		mode := artworkUpscaleMode()
		frameCount := 1
		if clImages != nil {
			frameCount = clImages.NumFrames(uint32(id))
		}
		if frameCount < 1 {
			frameCount = 1
		}
		frame %= frameCount
		if frame < 0 {
			frame += frameCount
		}
		if !cacheScaledPictureFrames(id, frame, frameCount, factor, mode, img) {
			continue
		}
		key := scaledImageKey{imageKey: makeImageKey(id, frame), scale: uint8(factor), mode: uint8(mode)}
		imageMu.Lock()
		scaled := scaledImageCache[key]
		imageMu.Unlock()
		if scaled == nil {
			return img
		}
		return scaled
	}
}

type pendingScaledMobileFrame struct {
	key   scaledMobileKey
	image *ebiten.Image
}

// cacheScaledMobileFrames upscales every valid pose already exposed by the
// source sheet for one exact color palette. It never generates other palette
// variants speculatively.
func cacheScaledMobileFrames(requestedKey mobileKey, factor, mode int, requestedImage *ebiten.Image) bool {
	baseKey := requestedKey
	baseKey.state = 0
	batchKey := scaledMobileBatchKey{mobileKey: baseKey, scale: uint8(factor), mode: uint8(mode)}
	imageMu.Lock()
	ensureScaledArtworkCacheFactorLocked(factor)
	if _, ok := scaledMobileBatches[batchKey]; ok {
		imageMu.Unlock()
		return true
	}
	pendingSources := make([]struct {
		key   mobileKey
		image *ebiten.Image
	}, 0, 256)
	sourceCount := 0
	for state := 0; state < 256; state++ {
		key := baseKey
		key.state = uint8(state)
		source, ok := mobileCache[key]
		if !ok || source == nil {
			continue
		}
		sourceCount++
		scaledKey := scaledMobileKey{mobileKey: key, scale: uint8(factor), mode: uint8(mode)}
		if _, ok := scaledMobileCache[scaledKey]; !ok {
			pendingSources = append(pendingSources, struct {
				key   mobileKey
				image *ebiten.Image
			}{key: key, image: source})
		}
	}
	requestedScaledKey := scaledMobileKey{mobileKey: requestedKey, scale: uint8(factor), mode: uint8(mode)}
	if _, ok := scaledMobileCache[requestedScaledKey]; !ok {
		foundRequested := false
		for _, source := range pendingSources {
			if source.key == requestedKey {
				foundRequested = true
				break
			}
		}
		if !foundRequested && requestedImage != nil {
			pendingSources = append(pendingSources, struct {
				key   mobileKey
				image *ebiten.Image
			}{key: requestedKey, image: requestedImage})
		}
	}
	imageMu.Unlock()
	if len(pendingSources) == 0 {
		if sourceCount > 0 {
			imageMu.Lock()
			if scaledCacheFactor == uint8(factor) {
				scaledMobileBatches[batchKey] = struct{}{}
			}
			imageMu.Unlock()
		}
		return true
	}

	pending := make([]pendingScaledMobileFrame, 0, len(pendingSources))
	for _, source := range pendingSources {
		pending = append(pending, pendingScaledMobileFrame{
			key:   scaledMobileKey{mobileKey: source.key, scale: uint8(factor), mode: uint8(mode)},
			image: upscaleSpriteImageWithModeAndLifetime(source.image, factor, mode, false),
		})
	}

	imageMu.Lock()
	if scaledCacheFactor != uint8(factor) {
		imageMu.Unlock()
		for _, frame := range pending {
			frame.image.Deallocate()
		}
		return false
	}
	for _, frame := range pending {
		if _, ok := scaledMobileCache[frame.key]; ok {
			frame.image.Deallocate()
			continue
		}
		scaledMobileCache[frame.key] = frame.image
	}
	complete := false
	for state := 0; state < 256; state++ {
		key := baseKey
		key.state = uint8(state)
		source, ok := mobileCache[key]
		if !ok || source == nil {
			continue
		}
		complete = true
		scaledKey := scaledMobileKey{mobileKey: key, scale: uint8(factor), mode: uint8(mode)}
		if _, ok := scaledMobileCache[scaledKey]; !ok {
			complete = false
			break
		}
	}
	if complete {
		scaledMobileBatches[batchKey] = struct{}{}
	}
	imageMu.Unlock()
	return true
}

func getScaledMobileFrame(key mobileKey, img *ebiten.Image) *ebiten.Image {
	if img == nil || !artworkUpscaleEnabled() {
		return img
	}
	for {
		factor := screenCappedArtworkUpscaleFactor()
		mode := artworkUpscaleMode()
		if !cacheScaledMobileFrames(key, factor, mode, img) {
			continue
		}
		scaledKey := scaledMobileKey{mobileKey: key, scale: uint8(factor), mode: uint8(mode)}
		imageMu.Lock()
		scaled := scaledMobileCache[scaledKey]
		imageMu.Unlock()
		if scaled == nil {
			return img
		}
		return scaled
	}
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
	blended := newUnmanagedImage(size, size)
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
	blended := newUnmanagedImage(w, h)
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
