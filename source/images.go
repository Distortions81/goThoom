package main

import (
	_ "embed"
	"encoding/binary"
	"encoding/csv"
	"fmt"
	"image"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
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
	// scaledImageCache stores pixel-art upscaled world picture frames.
	scaledImageCache = make(map[scaledImageKey]*ebiten.Image)
	// scaledMobileCache stores pixel-art upscaled mobile frames.
	scaledMobileCache = make(map[scaledMobileKey]*ebiten.Image)
	// mobileRecolorMaskCache stores palette-independent per-pixel custom-color
	// influences for mobile poses. Keys never contain custom colors.
	mobileRecolorMaskCache = make(map[scaledMobileKey]*ebiten.Image)
	// mobilePaletteDeltaCache stores the small palette payload sent to the GPU.
	mobilePaletteDeltaCache = make(map[mobileKey]*mobilePaletteShaderState)
	// Completed batch keys avoid rescanning every animation frame or mobile pose
	// on every draw after its first-use upscale batch finishes.
	scaledPictureBatches     = make(map[scaledPictureBatchKey]struct{})
	scaledMobileBatches      = make(map[scaledMobileBatchKey]struct{})
	mobileRecolorMaskBatches = make(map[scaledMobileBatchKey]struct{})
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

	spriteUpscaleShader      *ebiten.Shader
	frameBlendShader         *ebiten.Shader
	mobileRecolorShader      *ebiten.Shader
	mobileRecolorBlendShader *ebiten.Shader
	spriteUpscaleScratchMu   sync.Mutex
	spriteUpscaleScratch     reusableUpscaleScratch

	artworkWorkerOnce sync.Once
	artworkWorkerJobs chan func()
	artworkRGBAPoolMu sync.Mutex
	artworkRGBAPool   = make(map[image.Point][]*image.RGBA)
	artworkRGBAPixels int
)

const maxArtworkWorkers = 16
const maxArtworkPooledBytes = 64 << 20

func acquireArtworkRGBA(bounds image.Rectangle) *image.RGBA {
	size := bounds.Size()
	artworkRGBAPoolMu.Lock()
	pooled := artworkRGBAPool[size]
	if len(pooled) != 0 {
		img := pooled[len(pooled)-1]
		artworkRGBAPool[size] = pooled[:len(pooled)-1]
		artworkRGBAPixels -= size.X * size.Y
		artworkRGBAPoolMu.Unlock()
		img.Rect = bounds
		img.Stride = size.X * 4
		return img
	}
	artworkRGBAPoolMu.Unlock()
	return image.NewRGBA(bounds)
}

func releaseArtworkRGBA(img *image.RGBA) {
	if img == nil || img.Bounds().Empty() {
		return
	}
	size := img.Bounds().Size()
	pixels := size.X * size.Y
	artworkRGBAPoolMu.Lock()
	if (artworkRGBAPixels+pixels)*4 <= maxArtworkPooledBytes && len(artworkRGBAPool[size]) < maxArtworkWorkers {
		artworkRGBAPool[size] = append(artworkRGBAPool[size], img)
		artworkRGBAPixels += pixels
	}
	artworkRGBAPoolMu.Unlock()
}

func runArtworkJobs(jobs []func()) {
	if len(jobs) == 0 {
		return
	}
	artworkWorkerOnce.Do(func() {
		// Bound simultaneous 4x output buffers while still using enough cores for
		// room-wide batches of independent scenery and mobile poses.
		workerCount := min(max(1, runtime.GOMAXPROCS(0)), maxArtworkWorkers)
		artworkWorkerJobs = make(chan func(), workerCount*2)
		for range workerCount {
			go func() {
				for job := range artworkWorkerJobs {
					job()
				}
			}()
		}
	})
	var workers sync.WaitGroup
	workers.Add(len(jobs))
	for _, job := range jobs {
		job := job
		artworkWorkerJobs <- func() {
			defer workers.Done()
			job()
		}
	}
	workers.Wait()
}

type preparedArtworkRegion struct {
	index      int
	rect       image.Rectangle
	base       *image.RGBA
	scaled     *image.RGBA
	influence  *image.RGBA
	metrics    mobileSpriteMetrics
	hasMetrics bool
}

type preparedArtworkSheet struct {
	key           sheetKey
	pixels        *image.RGBA
	slots         *image.Gray
	regions       []preparedArtworkRegion
	needBase      bool
	needScale     bool
	needInfluence bool
	factor        int
	mode          int
}

func mobileRecolorSourceEligible(key sheetKey) bool {
	return key.forceTransparent && key.colorsLen == 0 && clImages != nil && clImages.HasCustomColors(uint32(key.id)) &&
		gs.ShadersEnabled && !gs.DenoiseImages && mobileRecolorShader != nil && mobileRecolorBlendShader != nil
}

func mobileRecolorBatchKey(id uint16, factor, mode int) scaledMobileBatchKey {
	return scaledMobileBatchKey{
		mobileKey: makeMobileKey(id, 0, nil),
		scale:     uint8(factor),
		mode:      uint8(mode),
	}
}

func sheetKeyColors(key *sheetKey) []byte {
	if key.colorsLen == 0 {
		return nil
	}
	return key.colors[:int(key.colorsLen)]
}

func artworkRegionRects(key sheetKey, pixels *image.RGBA) []image.Rectangle {
	if pixels == nil || pixels.Bounds().Dx() <= 2 || pixels.Bounds().Dy() <= 2 {
		return nil
	}
	if key.forceTransparent {
		size := (pixels.Bounds().Dx() - 2) / 16
		if size < 1 {
			return nil
		}
		regions := make([]image.Rectangle, 0, 256)
		for row := 0; row < 16; row++ {
			for column := 0; column < 16; column++ {
				x := 1 + column*size
				y := 1 + row*size
				r := image.Rect(x, y, x+size, y+size)
				if r.In(pixels.Bounds()) {
					regions = append(regions, r)
				}
			}
		}
		return regions
	}
	frames := 1
	if clImages != nil {
		frames = max(1, clImages.NumFrames(uint32(key.id)))
	}
	height := (pixels.Bounds().Dy() - 2) / frames
	width := pixels.Bounds().Dx() - 2
	if width < 1 || height < 1 {
		return nil
	}
	regions := make([]image.Rectangle, 0, frames)
	for frame := 0; frame < frames; frame++ {
		y := 1 + frame*height
		regions = append(regions, image.Rect(1, y, 1+width, y+height))
	}
	return regions
}

func copyArtworkRegion(source *image.RGBA, sourceRect image.Rectangle) (*image.RGBA, bool) {
	sourceRect = sourceRect.Intersect(source.Bounds())
	if sourceRect.Empty() {
		return nil, false
	}
	visible := false

scan:
	for y := 0; y < sourceRect.Dy(); y++ {
		sourceOffset := source.PixOffset(sourceRect.Min.X, sourceRect.Min.Y+y)
		row := source.Pix[sourceOffset : sourceOffset+sourceRect.Dx()*4]
		for offset := 3; offset < len(row); offset += 4 {
			if row[offset] != 0 {
				visible = true
				break scan
			}
		}
	}
	if !visible {
		return nil, false
	}
	destination := acquireArtworkRGBA(image.Rect(0, 0, sourceRect.Dx(), sourceRect.Dy()))
	for y := 0; y < sourceRect.Dy(); y++ {
		sourceOffset := source.PixOffset(sourceRect.Min.X, sourceRect.Min.Y+y)
		destinationOffset := destination.PixOffset(0, y)
		copy(destination.Pix[destinationOffset:destinationOffset+sourceRect.Dx()*4], source.Pix[sourceOffset:sourceOffset+sourceRect.Dx()*4])
	}
	return destination, visible
}

func pasteArtworkRegion(destination *image.RGBA, destinationRect image.Rectangle, source *image.RGBA) {
	if destination == nil || source == nil {
		return
	}
	width := min(destinationRect.Dx(), source.Bounds().Dx())
	height := min(destinationRect.Dy(), source.Bounds().Dy())
	for y := 0; y < height; y++ {
		destinationOffset := destination.PixOffset(destinationRect.Min.X, destinationRect.Min.Y+y)
		sourceOffset := source.PixOffset(source.Bounds().Min.X, source.Bounds().Min.Y+y)
		copy(destination.Pix[destinationOffset:destinationOffset+width*4], source.Pix[sourceOffset:sourceOffset+width*4])
	}
}

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

//go:embed data/shaders/frame_blend.kage
var frameBlendShaderSource []byte

//go:embed data/shaders/mobile_recolor.kage
var mobileRecolorShaderSource []byte

//go:embed data/shaders/mobile_recolor_blend.kage
var mobileRecolorBlendShaderSource []byte

// ReloadSpriteUpscaleShader recompiles the artwork-upscale shader.
func ReloadSpriteUpscaleShader() error {
	upscale, err := ebiten.NewShader(spriteUpscaleShaderSource)
	if err != nil {
		return err
	}
	blend, err := ebiten.NewShader(frameBlendShaderSource)
	if err != nil {
		upscale.Deallocate()
		return err
	}
	recolor, err := ebiten.NewShader(mobileRecolorShaderSource)
	if err != nil {
		upscale.Deallocate()
		blend.Deallocate()
		return err
	}
	recolorBlend, err := ebiten.NewShader(mobileRecolorBlendShaderSource)
	if err != nil {
		upscale.Deallocate()
		blend.Deallocate()
		recolor.Deallocate()
		return err
	}
	spriteUpscaleShader = upscale
	frameBlendShader = blend
	mobileRecolorShader = recolor
	mobileRecolorBlendShader = recolorBlend
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

func artworkSheetBatchCompleteLocked(key sheetKey, factor, mode int) bool {
	if key.forceTransparent {
		baseKey := makeMobileKey(key.id, 0, key.colors[:int(key.colorsLen)])
		_, ok := scaledMobileBatches[scaledMobileBatchKey{mobileKey: baseKey, scale: uint8(factor), mode: uint8(mode)}]
		return ok
	}
	_, ok := scaledPictureBatches[scaledPictureBatchKey{id: key.id, scale: uint8(factor), mode: uint8(mode)}]
	return ok
}

func markArtworkSheetBatchCompleteLocked(key sheetKey, factor, mode int) {
	if key.forceTransparent {
		baseKey := makeMobileKey(key.id, 0, key.colors[:int(key.colorsLen)])
		scaledMobileBatches[scaledMobileBatchKey{mobileKey: baseKey, scale: uint8(factor), mode: uint8(mode)}] = struct{}{}
		return
	}
	scaledPictureBatches[scaledPictureBatchKey{id: key.id, scale: uint8(factor), mode: uint8(mode)}] = struct{}{}
}

// prepareArtworkSheets decodes all requested sheets in parallel, then sends
// every independent frame or pose through the same global CPU worker queue.
// Ebitengine images are created only after the batch finishes, on the caller's
// render goroutine.
func prepareArtworkSheets(keys []sheetKey) int {
	return prepareArtworkSheetsInternal(keys, false)
}

// prepareBaseArtworkSheets decodes and uploads only the requested sheets. It
// deliberately skips pose processing, upscaling, metrics, and recolor masks so
// broad boot-time mobile preloads remain quick and memory-bounded.
func prepareBaseArtworkSheets(keys []sheetKey) int {
	return prepareArtworkSheetsInternal(keys, true)
}

func prepareArtworkSheetsInternal(keys []sheetKey, baseOnly bool) int {
	if clImages == nil || len(keys) == 0 {
		return 0
	}
	imageCacheLifecycleMu.RLock()
	defer imageCacheLifecycleMu.RUnlock()
	unpin := pinPreparingSpriteSlots(keys)
	defer unpin()
	traceEnabled := currentAssetLoadFrameTrace() != nil
	var prepareStarted time.Time
	if traceEnabled {
		prepareStarted = time.Now()
	}

	factor := screenCappedArtworkUpscaleFactor()
	mode := artworkUpscaleMode()
	needUpscale := !baseOnly && artworkUpscaleEnabled()
	imageMu.Lock()
	if needUpscale {
		ensureScaledArtworkCacheFactorLocked(factor)
	}
	seen := make(map[sheetKey]struct{}, len(keys))
	work := make([]preparedArtworkSheet, 0, len(keys))
	for _, key := range keys {
		if key.id == 0xffff || replacementEffectReplacesPict(key.id) {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		base, haveBase := sheetCache[key]
		knownMissing := haveBase && base == nil
		haveScale := !needUpscale || knownMissing || artworkSheetBatchCompleteLocked(key, factor, mode)
		influenceFactor, influenceMode := factor, mode
		if !needUpscale {
			influenceFactor, influenceMode = 1, artworkUpscaleOff
		}
		needInfluence := !baseOnly && mobileRecolorSourceEligible(key)
		_, haveInfluence := mobileRecolorMaskBatches[mobileRecolorBatchKey(key.id, influenceFactor, influenceMode)]
		if !needInfluence || knownMissing {
			haveInfluence = true
		}
		if haveBase && haveScale && haveInfluence {
			continue
		}
		work = append(work, preparedArtworkSheet{
			key:           key,
			needBase:      !haveBase,
			needScale:     needUpscale && !haveScale,
			needInfluence: needInfluence && !haveInfluence,
			factor:        factor,
			mode:          mode,
		})
	}
	imageMu.Unlock()
	if len(work) == 0 {
		return 0
	}
	noteClientActivity(clientActivityData)

	var decodeStarted time.Time
	if traceEnabled {
		decodeStarted = time.Now()
	}
	decodeJobs := make([]func(), len(work))
	for index := range work {
		index := index
		decodeJobs[index] = func() {
			key := &work[index].key
			if work[index].needInfluence {
				work[index].pixels, work[index].slots = clImages.DecodeRGBAWithCustomColorSlots(uint32(key.id), sheetKeyColors(key), key.forceTransparent, maxColors)
			} else {
				work[index].pixels = clImages.DecodeRGBA(uint32(key.id), sheetKeyColors(key), key.forceTransparent)
			}
		}
	}
	runArtworkJobs(decodeJobs)
	var decodeDuration time.Duration
	if traceEnabled {
		decodeDuration = time.Since(decodeStarted)
	}

	var processStarted time.Time
	if traceEnabled {
		processStarted = time.Now()
	}
	denoise := gs.DenoiseImages
	sharpness := gs.DenoiseSharpness
	amount := gs.DenoiseAmount
	regionJobs := make([]func(), 0)
	regionWorkers := 1
	for sheetIndex := range work {
		if work[sheetIndex].pixels == nil {
			continue
		}
		if baseOnly {
			continue
		}
		rects := artworkRegionRects(work[sheetIndex].key, work[sheetIndex].pixels)
		work[sheetIndex].regions = make([]preparedArtworkRegion, len(rects))
		for regionIndex, rect := range rects {
			work[sheetIndex].regions[regionIndex] = preparedArtworkRegion{index: regionIndex, rect: rect}
			if !denoise && !work[sheetIndex].needScale && !work[sheetIndex].needInfluence && !work[sheetIndex].key.forceTransparent {
				continue
			}
			sheetIndex, regionIndex := sheetIndex, regionIndex
			regionJobs = append(regionJobs, func() {
				region := &work[sheetIndex].regions[regionIndex]
				base, visible := copyArtworkRegion(work[sheetIndex].pixels, region.rect)
				if !visible {
					return
				}
				if work[sheetIndex].key.forceTransparent {
					region.metrics = mobileSpriteMetricsFromRGBA(base)
					region.hasMetrics = true
				}
				if denoise {
					if regionWorkers > 1 && base.Bounds().Dx()*base.Bounds().Dy() >= 64*64 {
						climg.DenoiseRGBAWorkers(base, sharpness, amount, regionWorkers)
					} else {
						climg.DenoiseRGBASerial(base, sharpness, amount)
					}
					region.base = base
				}
				if work[sheetIndex].needScale || work[sheetIndex].needInfluence {
					factor, mode := work[sheetIndex].factor, work[sheetIndex].mode
					if !work[sheetIndex].needScale {
						factor, mode = 1, artworkUpscaleOff
					}
					var slotRegion image.Rectangle
					if work[sheetIndex].slots != nil {
						slotRegion = region.rect
					}
					region.scaled, region.influence = upscaleSpriteRegionCPUWithInfluenceWorkers(base, base.Bounds(), work[sheetIndex].slots, slotRegion, factor, mode, acquireArtworkRGBA, regionWorkers)
					if !work[sheetIndex].needScale {
						releaseArtworkRGBA(region.scaled)
						region.scaled = nil
					}
				}
				if !denoise {
					releaseArtworkRGBA(base)
				}
			})
		}
	}
	// Most sheets contain many small independent frames, which already occupy
	// the shared worker pool well. A sheet with only one or two large frames was
	// previously denoised and upscaled on a single core and accounted for the
	// longest art preparation hitches. Divide the available CPU budget between
	// the active outer jobs and their internal row workers.
	availableWorkers := min(max(1, runtime.GOMAXPROCS(0)), maxArtworkWorkers)
	activeRegionJobs := min(len(regionJobs), availableWorkers)
	if activeRegionJobs > 0 {
		regionWorkers = max(1, availableWorkers/activeRegionJobs)
	}
	runArtworkJobs(regionJobs)
	var processDuration time.Duration
	if traceEnabled {
		processDuration = time.Since(processStarted)
	}

	var uploadStarted time.Time
	if traceEnabled {
		uploadStarted = time.Now()
	}
	for sheetIndex := range work {
		prepared := &work[sheetIndex]
		if prepared.pixels == nil {
			// Cache failed archive lookups too. Movie states are applied repeatedly;
			// without this sentinel, a missing descriptor sheet is decoded again on
			// every frame and can reduce playback to a few FPS.
			firstFailure := false
			imageMu.Lock()
			if _, exists := sheetCache[prepared.key]; !exists {
				sheetCache[prepared.key] = nil
				firstFailure = true
			}
			imageMu.Unlock()
			if firstFailure {
				log.Printf("missing image %d", prepared.key.id)
			}
			continue
		}
		if denoise {
			for index := range prepared.regions {
				pasteArtworkRegion(prepared.pixels, prepared.regions[index].rect, prepared.regions[index].base)
				releaseArtworkRGBA(prepared.regions[index].base)
				prepared.regions[index].base = nil
			}
		}
		var sheet *ebiten.Image
		if prepared.needBase {
			sheet = newManagedImageFromImage(prepared.pixels)
			imageMu.Lock()
			if existing := sheetCache[prepared.key]; existing != nil {
				deallocateImage(sheet)
				sheet = existing
			} else {
				sheetCache[prepared.key] = sheet
			}
			imageMu.Unlock()
			statImageLoaded(prepared.key.id)
			if imgDump && prepared.key.colorsLen == 0 && !prepared.key.forceTransparent {
				dumpImageSheet(prepared.key.id, sheet)
			}
		}
		if prepared.key.forceTransparent {
			imageMu.Lock()
			for index := range prepared.regions {
				region := &prepared.regions[index]
				if !region.hasMetrics {
					continue
				}
				key := makeMobileKey(prepared.key.id, uint8(region.index), prepared.key.colors[:int(prepared.key.colorsLen)])
				mobileSpriteMetricsCache[key] = region.metrics
			}
			imageMu.Unlock()
		}
		if !prepared.needScale {
			// A non-upscaled recolor influence still has to be uploaded below.
			if !prepared.needInfluence {
				prepared.pixels = nil
				continue
			}
		}
		imageMu.Lock()
		if prepared.needScale && scaledCacheFactor != uint8(prepared.factor) {
			imageMu.Unlock()
			for index := range prepared.regions {
				releaseArtworkRGBA(prepared.regions[index].scaled)
				prepared.regions[index].scaled = nil
			}
			prepared.pixels = nil
			continue
		}
		for index := range prepared.regions {
			region := &prepared.regions[index]
			if region.scaled == nil {
				// Influence-only work continues below.
			} else if prepared.key.forceTransparent {
				mobileKey := makeMobileKey(prepared.key.id, uint8(region.index), prepared.key.colors[:int(prepared.key.colorsLen)])
				key := scaledMobileKey{mobileKey: mobileKey, scale: uint8(prepared.factor), mode: uint8(prepared.mode)}
				if _, exists := scaledMobileCache[key]; !exists {
					scaledMobileCache[key] = cachedSpriteSlotLocked(spriteSlotKey{kind: 1, mobile: key}, region.scaled)
				}
			} else {
				key := scaledImageKey{imageKey: makeImageKey(prepared.key.id, region.index), scale: uint8(prepared.factor), mode: uint8(prepared.mode)}
				if _, exists := scaledImageCache[key]; !exists {
					scaledImageCache[key] = cachedSpriteSlotLocked(spriteSlotKey{picture: key}, region.scaled)
				}
			}
			if region.influence != nil {
				influenceFactor, influenceMode := prepared.factor, prepared.mode
				if !prepared.needScale {
					influenceFactor, influenceMode = 1, artworkUpscaleOff
				}
				mobileKey := makeMobileKey(prepared.key.id, uint8(region.index), nil)
				key := scaledMobileKey{mobileKey: mobileKey, scale: uint8(influenceFactor), mode: uint8(influenceMode)}
				if _, exists := mobileRecolorMaskCache[key]; !exists {
					mobileRecolorMaskCache[key] = cachedSpriteSlotLocked(spriteSlotKey{kind: 2, mobile: key}, region.influence)
				}
				releaseArtworkRGBA(region.influence)
				region.influence = nil
			}
			if region.scaled != nil {
				releaseArtworkRGBA(region.scaled)
				region.scaled = nil
			}
		}
		if prepared.needScale {
			markArtworkSheetBatchCompleteLocked(prepared.key, prepared.factor, prepared.mode)
		}
		if prepared.needInfluence {
			influenceFactor, influenceMode := prepared.factor, prepared.mode
			if !prepared.needScale {
				influenceFactor, influenceMode = 1, artworkUpscaleOff
			}
			mobileRecolorMaskBatches[mobileRecolorBatchKey(prepared.key.id, influenceFactor, influenceMode)] = struct{}{}
		}
		imageMu.Unlock()
		prepared.pixels = nil
	}
	if traceEnabled {
		ids := make([]uint16, len(work))
		for index := range work {
			ids[index] = work[index].key.id
		}
		noteFrameArtworkPrepare(len(keys), len(work), factor, ids, time.Since(prepareStarted), decodeDuration, processDuration, time.Since(uploadStarted))
	}
	return len(work)
}

// loadSheet retrieves the processed full sprite sheet for the specified ID.
func loadSheet(id uint16, colors []byte, forceTransparent bool) *ebiten.Image {
	key := makeSheetKey(id, colors, forceTransparent)
	imageMu.Lock()
	img, cached := sheetCache[key]
	imageMu.Unlock()
	if cached {
		return img
	}
	prepareArtworkSheets([]sheetKey{key})
	imageMu.Lock()
	img = sheetCache[key]
	imageMu.Unlock()
	return img
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
		dir := assetDumpImageDir()
		os.MkdirAll(dir, 0755)
		if f, err := os.Create(filepath.Join(dir, "metadata.csv")); err == nil {
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

	framesToDump := imageDumpFrameCount(frames, imgDumpSingleFrame)
	for f := 0; f < framesToDump; f++ {
		y := 1 + f*h
		frameImg := sheet.SubImage(image.Rect(1, y, 1+innerWidth, y+h)).(*ebiten.Image)
		fn := filepath.Join(assetDumpImageDir(), imageDumpFrameFilename(id, f, frames))
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

func imageDumpFrameCount(frames int, singleFrame bool) int {
	if frames <= 0 || singleFrame {
		return 1
	}
	return frames
}

func imageDumpFrameFilename(id uint16, frame, frames int) string {
	if frames <= 1 {
		return fmt.Sprintf("%d.png", id)
	}
	return fmt.Sprintf("%d_%d.png", id, frame)
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

func newCachedSpriteImageFromRGBA(source *image.RGBA) *ebiten.Image {
	// Cached upscales are stable atlas residents. Unlike transient exports,
	// blend intermediates, and reusable scratch, they are managed unless Potato
	// GPU mode requires standalone textures for the 4096x4096 hardware limit.
	return newManagedImageFromImage(source)
}

type spriteUpscaleRegion struct {
	source *image.RGBA
	rect   image.Rectangle
}

// upscaleSpriteRegionsCPU sends independent regions through the shared artwork
// workers. Callers upload the finished RGBA images sequentially.
func upscaleSpriteRegionsCPU(regions []spriteUpscaleRegion, factor, mode int) []*image.RGBA {
	results := make([]*image.RGBA, len(regions))
	jobs := make([]func(), len(regions))
	for index := range regions {
		index := index
		jobs[index] = func() {
			region := regions[index]
			results[index] = upscaleSpriteRegionCPU(region.source, region.rect, factor, mode)
		}
	}
	runArtworkJobs(jobs)
	return results
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

func minFloat32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxFloat32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func clampFloat32(v, low, high float32) float32 {
	return minFloat32(high, maxFloat32(low, v))
}

func upscaleColorDistance(a, b upscaleRGBA) float32 {
	if a.a < 1 && b.a < 1 {
		return 0
	}
	aa := maxFloat32(a.a, 1)
	ba := maxFloat32(b.a, 1)
	dr := absFloat32(a.r/aa - b.r/ba)
	dg := absFloat32(a.g/aa - b.g/ba)
	db := absFloat32(a.b/aa - b.b/ba)
	luma := dr*0.299 + dg*0.587 + db*0.114
	chroma := maxFloat32(dr, maxFloat32(dg, db)) - minFloat32(dr, minFloat32(dg, db))
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
	return upscaleSpriteRegionCPUWithAllocator(source, sourceRect, factor, mode, image.NewRGBA)
}

func upscaleSpriteRegionCPUWithAllocator(source *image.RGBA, sourceRect image.Rectangle, factor, mode int, allocate func(image.Rectangle) *image.RGBA) *image.RGBA {
	destination, _ := upscaleSpriteRegionCPUWithInfluence(source, sourceRect, nil, image.Rectangle{}, factor, mode, allocate)
	return destination
}

func upscaleSpriteRegionCPUWithInfluence(source *image.RGBA, sourceRect image.Rectangle, slots *image.Gray, slotRect image.Rectangle, factor, mode int, allocate func(image.Rectangle) *image.RGBA) (*image.RGBA, *image.RGBA) {
	return upscaleSpriteRegionCPUWithInfluenceWorkers(source, sourceRect, slots, slotRect, factor, mode, allocate, 1)
}

func upscaleSpriteRegionCPUWithInfluenceWorkers(source *image.RGBA, sourceRect image.Rectangle, slots *image.Gray, slotRect image.Rectangle, factor, mode int, allocate func(image.Rectangle) *image.RGBA, workers int) (*image.RGBA, *image.RGBA) {
	sourceRect = sourceRect.Intersect(source.Bounds())
	if sourceRect.Empty() || factor < 1 {
		return image.NewRGBA(image.Rectangle{}), nil
	}
	destination := allocate(image.Rect(0, 0, sourceRect.Dx()*factor, sourceRect.Dy()*factor))
	var influence *image.RGBA
	if slots != nil && slotRect.Dx() == sourceRect.Dx() && slotRect.Dy() == sourceRect.Dy() && slotRect.In(slots.Bounds()) {
		influence = allocate(destination.Bounds())
	}
	slotAt := func(x, y int) byte {
		if influence == nil {
			return 0
		}
		sx := slotRect.Min.X + x - sourceRect.Min.X
		sy := slotRect.Min.Y + y - sourceRect.Min.Y
		return slots.Pix[slots.PixOffset(sx, sy)]
	}
	writeInfluence := func(x, y int, center, first, second byte, weight float32) {
		if influence == nil {
			return
		}
		// Three five-bit slot IDs and an eight-bit blend weight fit in RGB24.
		packed := uint32(center&31) | uint32(first&31)<<5 | uint32(second&31)<<10 | uint32(upscaleByte(weight*255))<<15
		offset := influence.PixOffset(x, y)
		influence.Pix[offset] = byte(packed)
		influence.Pix[offset+1] = byte(packed >> 8)
		influence.Pix[offset+2] = byte(packed >> 16)
		influence.Pix[offset+3] = 255
	}
	reach := artworkUpscaleCornerReachForMode(mode)
	strength := artworkUpscaleBlendStrengthForMode(mode)
	var cornerWeights [4][4][4]float32
	if factor <= 4 {
		for oy := 0; oy < factor; oy++ {
			localY := (float32(oy) + 0.5) / float32(factor)
			for ox := 0; ox < factor; ox++ {
				localX := (float32(ox) + 0.5) / float32(factor)
				cornerWeights[0][oy][ox] = clampFloat32(reach-2*(localX+localY), 0, 1) * strength
				cornerWeights[1][oy][ox] = clampFloat32(reach-2*((1-localX)+localY), 0, 1) * strength
				cornerWeights[2][oy][ox] = clampFloat32(reach-2*(localX+(1-localY)), 0, 1) * strength
				cornerWeights[3][oy][ox] = clampFloat32(reach-2*((1-localX)+(1-localY)), 0, 1) * strength
			}
		}
	}
	processRows := func(startY, endY int) {
		for sy := startY; sy < endY; sy++ {
			for sx := sourceRect.Min.X; sx < sourceRect.Max.X; sx++ {
				center := rgbaPixelAt(source, sx, sy)
				centerSlot := slotAt(sx, sy)
				topSlot := slotAt(sx, max(sourceRect.Min.Y, sy-1))
				leftSlot := slotAt(max(sourceRect.Min.X, sx-1), sy)
				rightSlot := slotAt(min(sourceRect.Max.X-1, sx+1), sy)
				bottomSlot := slotAt(sx, min(sourceRect.Max.Y-1, sy+1))
				top := rgbaPixelAt(source, sx, max(sourceRect.Min.Y, sy-1))
				left := rgbaPixelAt(source, max(sourceRect.Min.X, sx-1), sy)
				right := rgbaPixelAt(source, min(sourceRect.Max.X-1, sx+1), sy)
				bottom := rgbaPixelAt(source, sx, min(sourceRect.Max.Y-1, sy+1))
				edgeCrosses := upscaleColorDistance(top, bottom) > 0.07 && upscaleColorDistance(left, right) > 0.07
				topLeft := edgeCrosses && upscaleColorDistance(left, top) < 0.16
				topRight := edgeCrosses && upscaleColorDistance(top, right) < 0.16
				bottomLeft := edgeCrosses && upscaleColorDistance(left, bottom) < 0.16
				bottomRight := edgeCrosses && upscaleColorDistance(bottom, right) < 0.16
				if !topLeft && !topRight && !bottomLeft && !bottomRight {
					sourceOffset := source.PixOffset(sx, sy)
					rgba := binary.LittleEndian.Uint32(source.Pix[sourceOffset : sourceOffset+4])
					rgbaPair := uint64(rgba) | uint64(rgba)<<32
					for oy := 0; oy < factor; oy++ {
						destinationOffset := destination.PixOffset((sx-sourceRect.Min.X)*factor, (sy-sourceRect.Min.Y)*factor+oy)
						// The client uses factors 1 through 4. Fixed-width stores are
						// faster here than walking every replicated pixel separately.
						switch factor {
						case 1:
							binary.LittleEndian.PutUint32(destination.Pix[destinationOffset:destinationOffset+4], rgba)
						case 2:
							binary.LittleEndian.PutUint64(destination.Pix[destinationOffset:destinationOffset+8], rgbaPair)
						case 3:
							binary.LittleEndian.PutUint64(destination.Pix[destinationOffset:destinationOffset+8], rgbaPair)
							binary.LittleEndian.PutUint32(destination.Pix[destinationOffset+8:destinationOffset+12], rgba)
						case 4:
							binary.LittleEndian.PutUint64(destination.Pix[destinationOffset:destinationOffset+8], rgbaPair)
							binary.LittleEndian.PutUint64(destination.Pix[destinationOffset+8:destinationOffset+16], rgbaPair)
						default:
							for ox := 0; ox < factor; ox++ {
								binary.LittleEndian.PutUint32(destination.Pix[destinationOffset:destinationOffset+4], rgba)
								destinationOffset += 4
							}
						}
						if influence != nil {
							influenceOffset := influence.PixOffset((sx-sourceRect.Min.X)*factor, (sy-sourceRect.Min.Y)*factor+oy)
							encoded := uint32(centerSlot) | 0xff000000
							encodedPair := uint64(encoded) | uint64(encoded)<<32
							switch factor {
							case 1:
								binary.LittleEndian.PutUint32(influence.Pix[influenceOffset:influenceOffset+4], encoded)
							case 2:
								binary.LittleEndian.PutUint64(influence.Pix[influenceOffset:influenceOffset+8], encodedPair)
							case 3:
								binary.LittleEndian.PutUint64(influence.Pix[influenceOffset:influenceOffset+8], encodedPair)
								binary.LittleEndian.PutUint32(influence.Pix[influenceOffset+8:influenceOffset+12], encoded)
							case 4:
								binary.LittleEndian.PutUint64(influence.Pix[influenceOffset:influenceOffset+8], encodedPair)
								binary.LittleEndian.PutUint64(influence.Pix[influenceOffset+8:influenceOffset+16], encodedPair)
							default:
								for ox := 0; ox < factor; ox++ {
									binary.LittleEndian.PutUint32(influence.Pix[influenceOffset:influenceOffset+4], encoded)
									influenceOffset += 4
								}
							}
						}
					}
					continue
				}
				for oy := 0; oy < factor; oy++ {
					for ox := 0; ox < factor; ox++ {
						target := center
						weight := float32(0)
						firstSlot, secondSlot := byte(0), byte(0)
						leftHalf := (2*ox + 1) < factor
						topHalf := (2*oy + 1) < factor
						switch {
						case leftHalf && topHalf && topLeft:
							target = averageUpscaleColor(left, top)
							firstSlot, secondSlot = leftSlot, topSlot
							if factor <= 4 {
								weight = cornerWeights[0][oy][ox]
							} else {
								localX := (float32(ox) + 0.5) / float32(factor)
								localY := (float32(oy) + 0.5) / float32(factor)
								weight = clampFloat32(reach-2*(localX+localY), 0, 1) * strength
							}
						case !leftHalf && topHalf && topRight:
							target = averageUpscaleColor(top, right)
							firstSlot, secondSlot = topSlot, rightSlot
							if factor <= 4 {
								weight = cornerWeights[1][oy][ox]
							} else {
								localX := (float32(ox) + 0.5) / float32(factor)
								localY := (float32(oy) + 0.5) / float32(factor)
								weight = clampFloat32(reach-2*((1-localX)+localY), 0, 1) * strength
							}
						case leftHalf && !topHalf && bottomLeft:
							target = averageUpscaleColor(left, bottom)
							firstSlot, secondSlot = leftSlot, bottomSlot
							if factor <= 4 {
								weight = cornerWeights[2][oy][ox]
							} else {
								localX := (float32(ox) + 0.5) / float32(factor)
								localY := (float32(oy) + 0.5) / float32(factor)
								weight = clampFloat32(reach-2*(localX+(1-localY)), 0, 1) * strength
							}
						case !leftHalf && !topHalf && bottomRight:
							target = averageUpscaleColor(bottom, right)
							firstSlot, secondSlot = bottomSlot, rightSlot
							if factor <= 4 {
								weight = cornerWeights[3][oy][ox]
							} else {
								localX := (float32(ox) + 0.5) / float32(factor)
								localY := (float32(oy) + 0.5) / float32(factor)
								weight = clampFloat32(reach-2*((1-localX)+(1-localY)), 0, 1) * strength
							}
						}
						result := mixUpscaleColor(center, target, weight)
						dx := (sx-sourceRect.Min.X)*factor + ox
						dy := (sy-sourceRect.Min.Y)*factor + oy
						offset := destination.PixOffset(dx, dy)
						destination.Pix[offset] = upscaleByte(result.r)
						destination.Pix[offset+1] = upscaleByte(result.g)
						destination.Pix[offset+2] = upscaleByte(result.b)
						destination.Pix[offset+3] = upscaleByte(result.a)
						writeInfluence(dx, dy, centerSlot, firstSlot, secondSlot, weight)
					}
				}
			}
		}
	}
	if workers < 2 || sourceRect.Dx()*sourceRect.Dy() < 64*64 {
		processRows(sourceRect.Min.Y, sourceRect.Max.Y)
	} else {
		workers = min(workers, sourceRect.Dy())
		var wait sync.WaitGroup
		wait.Add(workers)
		for worker := 0; worker < workers; worker++ {
			startY := sourceRect.Min.Y + worker*sourceRect.Dy()/workers
			endY := sourceRect.Min.Y + (worker+1)*sourceRect.Dy()/workers
			go func() {
				defer wait.Done()
				processRows(startY, endY)
			}()
		}
		wait.Wait()
	}
	return destination, influence
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
	noteFrameAtlasFactorChange(scaledCacheFactor, uint8(factor))
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
// one first-use CPU batch. The outputs remain separate managed images, so
// frame boundaries and rendering behavior are unchanged while the images can
// share Ebitengine's atlas.
func cacheScaledPictureFrames(id uint16, requestedFrame, frameCount, factor, mode int, requestedImage *ebiten.Image) bool {
	if clImages == nil {
		return cacheScaledPictureFramesWithReader(id, requestedFrame, frameCount, factor, mode, requestedImage, readImageRGBA)
	}
	prepareArtworkSheets([]sheetKey{makeSheetKey(id, nil, false)})
	imageMu.Lock()
	complete := artworkSheetBatchCompleteLocked(makeSheetKey(id, nil, false), factor, mode)
	imageMu.Unlock()
	return complete
}

func cacheScaledPictureFramesWithReader(id uint16, requestedFrame, frameCount, factor, mode int, requestedImage *ebiten.Image, readPixels func(*ebiten.Image) *image.RGBA) bool {
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

	// The production source is one vertical sheet. Read it back once, then
	// process all of its missing frames on the CPU without repeatedly touching
	// the graphics driver. Tests and replacement sources can fall back to their
	// already-cropped frame images.
	var sheetPixels *image.RGBA
	frameWidth, frameHeight := 0, 0
	if clImages != nil {
		if sheet := loadSheet(id, nil, false); sheet != nil {
			frameWidth = sheet.Bounds().Dx() - 2
			innerHeight := sheet.Bounds().Dy() - 2
			if frameWidth > 0 && innerHeight >= frameCount && innerHeight%frameCount == 0 {
				frameHeight = innerHeight / frameCount
				sheetPixels = readPixels(sheet)
			}
		}
	}

	pendingKeys := make([]scaledImageKey, 0, len(missing))
	regions := make([]spriteUpscaleRegion, 0, len(missing))
	for _, frame := range missing {
		if sheetPixels != nil {
			y := 1 + frame*frameHeight
			pendingKeys = append(pendingKeys, scaledImageKey{imageKey: makeImageKey(id, frame), scale: uint8(factor), mode: uint8(mode)})
			regions = append(regions, spriteUpscaleRegion{
				source: sheetPixels,
				rect:   image.Rect(1, y, 1+frameWidth, y+frameHeight),
			})
		} else {
			source := loadImageFrame(id, frame)
			if source == nil && frame == requestedFrame {
				source = requestedImage
			}
			if source != nil {
				sourcePixels := readPixels(source)
				pendingKeys = append(pendingKeys, scaledImageKey{imageKey: makeImageKey(id, frame), scale: uint8(factor), mode: uint8(mode)})
				regions = append(regions, spriteUpscaleRegion{source: sourcePixels, rect: sourcePixels.Bounds()})
			}
		}
	}

	upscaled := upscaleSpriteRegionsCPU(regions, factor, mode)
	pending := make([]pendingScaledPictureFrame, 0, len(upscaled))
	for index, pixels := range upscaled {
		pending = append(pending, pendingScaledPictureFrame{
			key:   pendingKeys[index],
			image: newCachedSpriteImageFromRGBA(pixels),
		})
	}

	imageMu.Lock()
	if scaledCacheFactor != uint8(factor) {
		imageMu.Unlock()
		for _, frame := range pending {
			deallocateImage(frame.image)
		}
		return false
	}
	for _, frame := range pending {
		if _, ok := scaledImageCache[frame.key]; ok {
			deallocateImage(frame.image)
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
// source sheet for one exact color palette as one CPU batch. It never
// generates other palette variants speculatively.
func cacheScaledMobileFrames(requestedKey mobileKey, factor, mode int, requestedImage *ebiten.Image) bool {
	if clImages == nil {
		return cacheScaledMobileFramesWithReader(requestedKey, factor, mode, requestedImage, readImageRGBA)
	}
	colors := requestedKey.colors[:int(requestedKey.colorsLen)]
	key := makeSheetKey(requestedKey.id, colors, true)
	prepareArtworkSheets([]sheetKey{key})
	imageMu.Lock()
	complete := artworkSheetBatchCompleteLocked(key, factor, mode)
	imageMu.Unlock()
	return complete
}

func cacheScaledMobileFramesWithReader(requestedKey mobileKey, factor, mode int, requestedImage *ebiten.Image, readPixels func(*ebiten.Image) *image.RGBA) bool {
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

	// Mobile poses occupy a 16-column sheet. Read the exact observed palette
	// once, then isolate pose boundaries in CPU memory so neighboring poses do
	// not influence the upscale filter.
	var sheetPixels *image.RGBA
	innerSize := 0
	if clImages != nil {
		colors := baseKey.colors[:int(baseKey.colorsLen)]
		if sheet := loadSheet(baseKey.id, colors, true); sheet != nil {
			innerSize = (sheet.Bounds().Dx() - 2) / 16
			if innerSize > 0 {
				sheetPixels = readPixels(sheet)
			}
		}
	}

	pendingKeys := make([]scaledMobileKey, 0, len(pendingSources))
	regions := make([]spriteUpscaleRegion, 0, len(pendingSources))
	for _, source := range pendingSources {
		if sheetPixels != nil {
			column := int(source.key.state & 0x0f)
			row := int(source.key.state >> 4)
			x := 1 + column*innerSize
			y := 1 + row*innerSize
			sourceRect := image.Rect(x, y, x+innerSize, y+innerSize)
			if sourceRect.In(sheetPixels.Bounds()) {
				pendingKeys = append(pendingKeys, scaledMobileKey{mobileKey: source.key, scale: uint8(factor), mode: uint8(mode)})
				regions = append(regions, spriteUpscaleRegion{source: sheetPixels, rect: sourceRect})
			}
		} else {
			sourcePixels := readPixels(source.image)
			pendingKeys = append(pendingKeys, scaledMobileKey{mobileKey: source.key, scale: uint8(factor), mode: uint8(mode)})
			regions = append(regions, spriteUpscaleRegion{source: sourcePixels, rect: sourcePixels.Bounds()})
		}
	}

	upscaled := upscaleSpriteRegionsCPU(regions, factor, mode)
	pending := make([]pendingScaledMobileFrame, 0, len(upscaled))
	for index, pixels := range upscaled {
		pending = append(pending, pendingScaledMobileFrame{
			key:   pendingKeys[index],
			image: newCachedSpriteImageFromRGBA(pixels),
		})
	}

	imageMu.Lock()
	if scaledCacheFactor != uint8(factor) {
		imageMu.Unlock()
		for _, frame := range pending {
			deallocateImage(frame.image)
		}
		return false
	}
	for _, frame := range pending {
		if _, ok := scaledMobileCache[frame.key]; ok {
			deallocateImage(frame.image)
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

type imageCacheStatsData struct {
	slotExtraBytes                              int64
	slotCount                                   int
	slotBytes                                   int64
	slotUsedBytes                               int64
	slotReuses, slotEvictions, spriteGameFrames uint64
	slotLoads, slotReloads                      uint64
	sheetCount                                  int
	sheetBytes                                  int
	frameCount                                  int
	frameBytes                                  int
	scaledFrameCount                            int
	scaledFrameBytes                            int
	mobileCount                                 int
	mobileBytes                                 int
	scaledMobileCount                           int
	scaledMobileBytes                           int
}

// Include unused slots and padding once; occupied sprite views are already
// included in the scaled cache content totals below.
func (s imageCacheStatsData) totalBytes() int64 {
	return int64(s.sheetBytes) + int64(s.frameBytes) + int64(s.scaledFrameBytes) + int64(s.mobileBytes) + int64(s.scaledMobileBytes) + s.slotExtraBytes
}

// imageCacheStats returns the counts and approximate memory usage in bytes for
// each of the image caches: sheets, cropped frames, scaled variants, and blends.
func imageCacheStats() imageCacheStatsData {
	imageMu.Lock()
	defer imageMu.Unlock()

	stats := imageCacheStatsData{slotCount: spriteSlots.count, slotBytes: spriteSlots.bytes, slotUsedBytes: spriteSlots.usedBytes, slotReuses: spriteSlots.reuses, slotEvictions: spriteSlots.evictions, slotLoads: spriteSlots.loads, slotReloads: spriteSlots.reloads}
	stats.slotExtraBytes = spriteSlots.bytes
	for _, slots := range spriteSlots.owners {
		for _, slot := range slots {
			stats.slotExtraBytes -= slot.contentBytes
		}
	}
	spriteUsage.Lock()
	stats.spriteGameFrames = spriteUsage.frame
	spriteUsage.Unlock()
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
	for _, img := range mobileRecolorMaskCache {
		if img != nil {
			stats.scaledMobileCount++
			b := img.Bounds()
			stats.scaledMobileBytes += b.Dx() * b.Dy() * 4
		}
	}
	return stats
}
