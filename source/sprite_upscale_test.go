package main

import (
	"image"
	"image/color"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func isolateScaledArtworkCaches(t *testing.T) {
	t.Helper()
	imageMu.Lock()
	originalImageCache := scaledImageCache
	originalMobileCache := scaledMobileCache
	originalMobileBlendCache := mobileBlendCache
	originalPictBlendCache := pictBlendCache
	originalPictureBatches := scaledPictureBatches
	originalMobileBatches := scaledMobileBatches
	originalFactor := scaledCacheFactor
	scaledImageCache = make(map[scaledImageKey]*ebiten.Image)
	scaledMobileCache = make(map[scaledMobileKey]*ebiten.Image)
	mobileBlendCache = make(map[mobileBlendKey]*ebiten.Image)
	pictBlendCache = make(map[pictBlendKey]*ebiten.Image)
	scaledPictureBatches = make(map[scaledPictureBatchKey]struct{})
	scaledMobileBatches = make(map[scaledMobileBatchKey]struct{})
	scaledCacheFactor = 0
	imageMu.Unlock()
	t.Cleanup(func() {
		imageMu.Lock()
		clearScaledArtworkCachesLocked()
		scaledImageCache = originalImageCache
		scaledMobileCache = originalMobileCache
		mobileBlendCache = originalMobileBlendCache
		pictBlendCache = originalPictBlendCache
		scaledPictureBatches = originalPictureBatches
		scaledMobileBatches = originalMobileBatches
		scaledCacheFactor = originalFactor
		imageMu.Unlock()
	})
}

func isolateSourceFrameCaches(t *testing.T) {
	t.Helper()
	imageMu.Lock()
	originalImageCache := imageCache
	originalMobileCache := mobileCache
	imageCache = make(map[imageKey]*ebiten.Image)
	mobileCache = make(map[mobileKey]*ebiten.Image)
	imageMu.Unlock()
	t.Cleanup(func() {
		imageMu.Lock()
		imageCache = originalImageCache
		mobileCache = originalMobileCache
		imageMu.Unlock()
	})
}

func transparentSpritePixels(img *ebiten.Image) *image.RGBA {
	return image.NewRGBA(image.Rect(0, 0, img.Bounds().Dx(), img.Bounds().Dy()))
}

func TestArtworkUpscaleIsIndependentOfPotatoMode(t *testing.T) {
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })

	gs.SpriteUpscaleFilter = true
	gs.SpriteUpscaleMode = artworkUpscaleBalanced
	gs.GameScale = 4
	gs.SpriteUpscale = 4
	gs.PotatoGPU = false
	if !artworkUpscaleEnabled() || artworkUpscaleFactor() != 4 {
		t.Fatal("artwork upscale should use the full render scale normally")
	}
	gs.PotatoGPU = true
	if !artworkUpscaleEnabled() || artworkUpscaleFactor() != 4 {
		t.Fatal("Potato GPU should not change artwork upscaling")
	}
	gs.SpriteUpscaleFilter = false
	if artworkUpscaleEnabled() {
		t.Fatal("disabled artwork upscale should remain disabled")
	}
}

func TestArtworkUpscaleFactorIsCappedToTwiceScreenScale(t *testing.T) {
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })

	gs.SpriteUpscale = 4
	tests := []struct {
		scale float64
		want  int
	}{
		{scale: 0.5, want: 2},
		{scale: 1, want: 2},
		{scale: 1.49, want: 2},
		{scale: 1.5, want: 3},
		{scale: 1.99, want: 3},
		{scale: 2, want: 4},
		{scale: 4, want: 4},
	}
	for _, test := range tests {
		gs.GameScale = test.scale
		if got := screenCappedArtworkUpscaleFactor(); got != test.want {
			t.Errorf("screen scale %.2f capped factor = %d, want %d", test.scale, got, test.want)
		}
	}

	gs.SpriteUpscale = 2
	gs.GameScale = 4
	if got := screenCappedArtworkUpscaleFactor(); got != 2 {
		t.Fatalf("screen cap raised configured 2x upscale to %dx", got)
	}

	gs.SpriteUpscale = 1
	gs.GameScale = 0.5
	if got := screenCappedArtworkUpscaleFactor(); got != 2 {
		t.Fatalf("enabled artwork upscale fell below 2x: got %dx", got)
	}
	if got := spriteUpscaleFactorFromScale(1); got != 2 {
		t.Fatalf("1x render scale derived a %dx artwork upscale, want 2x", got)
	}
}

func TestArtworkUpscaleStyles(t *testing.T) {
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })

	tests := []struct {
		mode     int
		enabled  bool
		reach    float32
		strength float32
	}{
		{mode: artworkUpscaleOff, enabled: false, reach: 0, strength: 0},
		{mode: artworkUpscaleCrisp, enabled: true, reach: 1.35, strength: 0.65},
		{mode: artworkUpscaleBalanced, enabled: true, reach: 1.65, strength: 0.8},
		{mode: artworkUpscaleSmooth, enabled: true, reach: 2.75, strength: 1},
		{mode: artworkUpscaleUltraSmooth, enabled: true, reach: 2.75, strength: 0.82},
	}
	for _, tt := range tests {
		setArtworkUpscaleMode(tt.mode)
		if artworkUpscaleEnabled() != tt.enabled {
			t.Errorf("mode %d enabled = %v, want %v", tt.mode, artworkUpscaleEnabled(), tt.enabled)
		}
		if got := artworkUpscaleCornerReach(); got != tt.reach {
			t.Errorf("mode %d corner reach = %v, want %v", tt.mode, got, tt.reach)
		}
		if got := artworkUpscaleBlendStrength(); got != tt.strength {
			t.Errorf("mode %d blend strength = %v, want %v", tt.mode, got, tt.strength)
		}
	}
}

func TestKageArtworkUpscaleCreatesRequestedResolution(t *testing.T) {
	originalSettings := gs
	gs.PotatoGPU = false
	t.Cleanup(func() { gs = originalSettings })

	src := ebiten.NewImage(3, 5)
	src.Fill(color.RGBA{R: 255, A: 255})
	for factor := 2; factor <= 4; factor++ {
		scaled := upscaleTransientSpriteImageWithMode(src, factor, artworkUpscaleMode())
		if got, want := scaled.Bounds().Size(), image.Pt(3*factor, 5*factor); got != want {
			t.Fatalf("%dx shader upscale size = %v, want %v", factor, got, want)
		}
		scaled.Deallocate()
	}
}

func TestReusableUpscaleScratchGrowsWithoutShrinking(t *testing.T) {
	var scratch reusableUpscaleScratch
	t.Cleanup(scratch.deallocate)

	first := scratch.region(16, 12)
	backing := scratch.image
	if first.Bounds().Size() != image.Pt(16, 12) {
		t.Fatalf("initial scratch region = %v, want 16x12", first.Bounds().Size())
	}
	if smaller := scratch.region(8, 6); scratch.image != backing || smaller.Bounds().Size() != image.Pt(8, 6) {
		t.Fatalf("smaller scratch request replaced or mis-sized the backing: backing changed=%v size=%v", scratch.image != backing, smaller.Bounds().Size())
	}
	if grown := scratch.region(20, 10); scratch.image == backing || grown.Bounds().Size() != image.Pt(20, 10) {
		t.Fatalf("larger scratch request did not grow correctly: backing changed=%v size=%v", scratch.image != backing, grown.Bounds().Size())
	}
	if got := scratch.image.Bounds().Size(); got != image.Pt(20, 12) {
		t.Fatalf("grown scratch backing = %v, want 20x12", got)
	}
}

func TestCPUMobileUpscaleCacheReusesTexture(t *testing.T) {
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })
	isolateScaledArtworkCaches(t)
	gs.GameScale = 2
	gs.SpriteUpscale = 2
	gs.PotatoGPU = false
	gs.SpriteUpscaleFilter = true
	gs.SpriteUpscaleMode = artworkUpscaleBalanced

	src := ebiten.NewImage(4, 4)
	key := makeMobileKey(447, 0, nil)
	if !cacheScaledMobileFramesWithReader(key, 2, artworkUpscaleBalanced, src, transparentSpritePixels) {
		t.Fatal("balanced mobile batch was invalidated while being built")
	}
	first := getScaledMobileFrame(key, src)
	second := getScaledMobileFrame(key, src)
	if first != second {
		t.Fatal("CPU-upscaled mobile texture was not reused")
	}
	if got := first.Bounds().Size(); got != image.Pt(8, 8) {
		t.Fatalf("cached CPU texture size = %v, want 8x8", got)
	}
	setArtworkUpscaleMode(artworkUpscaleSmooth)
	if !cacheScaledMobileFramesWithReader(key, 2, artworkUpscaleSmooth, src, transparentSpritePixels) {
		t.Fatal("smooth mobile batch was invalidated while being built")
	}
	smooth := getScaledMobileFrame(key, src)
	if smooth == first {
		t.Fatal("smooth mode reused the balanced cached texture")
	}
}

func TestCPUArtworkUpscaleReplicatesPixelsWhenFilteringIsOff(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 4, 3))
	source.SetRGBA(1, 1, color.RGBA{R: 255, A: 255})
	source.SetRGBA(2, 1, color.RGBA{B: 255, A: 255})

	const factor = 3
	scaled := upscaleSpriteRegionCPU(source, image.Rect(1, 1, 3, 2), factor, artworkUpscaleOff)
	if got := scaled.Bounds().Size(); got != image.Pt(6, 3) {
		t.Fatalf("CPU upscale size = %v, want 6x3", got)
	}
	for y := 0; y < 3; y++ {
		for x := 0; x < 6; x++ {
			want := color.RGBA{R: 255, A: 255}
			if x >= 3 {
				want = color.RGBA{B: 255, A: 255}
			}
			if got := scaled.RGBAAt(x, y); got != want {
				t.Fatalf("CPU upscale pixel (%d,%d) = %#v, want %#v", x, y, got, want)
			}
		}
	}
}

func TestCPUArtworkUpscaleBlendsDetectedCorner(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 3, 3))
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	black := color.RGBA{A: 255}
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			source.SetRGBA(x, y, black)
		}
	}
	source.SetRGBA(1, 0, white)
	source.SetRGBA(0, 1, white)

	scaled := upscaleSpriteRegionCPU(source, source.Bounds(), 4, artworkUpscaleBalanced)
	if got, want := scaled.RGBAAt(4, 4), (color.RGBA{R: 204, G: 204, B: 204, A: 255}); got != want {
		t.Fatalf("CPU upscale corner pixel = %#v, want %#v", got, want)
	}
	if got := scaled.RGBAAt(7, 7); got != black {
		t.Fatalf("CPU upscale opposite pixel = %#v, want center color %#v", got, black)
	}
}

func TestAnimatedPictureUpscaleCachesEveryFrameOnFirstUse(t *testing.T) {
	isolateScaledArtworkCaches(t)
	isolateSourceFrameCaches(t)
	const (
		id         = 900
		frameCount = 3
		factor     = 2
	)
	imageMu.Lock()
	for frame := 0; frame < frameCount; frame++ {
		imageCache[makeImageKey(id, frame)] = ebiten.NewImage(3, 5)
	}
	requested := imageCache[makeImageKey(id, 0)]
	imageMu.Unlock()

	if !cacheScaledPictureFramesWithReader(id, 0, frameCount, factor, artworkUpscaleBalanced, requested, transparentSpritePixels) {
		t.Fatal("picture frame batch was invalidated while being built")
	}
	imageMu.Lock()
	defer imageMu.Unlock()
	if len(scaledImageCache) != frameCount {
		t.Fatalf("scaled picture cache has %d frames, want %d", len(scaledImageCache), frameCount)
	}
	if len(scaledPictureBatches) != 1 {
		t.Errorf("completed picture batches = %d, want 1", len(scaledPictureBatches))
	}
	for frame := 0; frame < frameCount; frame++ {
		key := scaledImageKey{imageKey: makeImageKey(id, frame), scale: factor, mode: artworkUpscaleBalanced}
		if got := scaledImageCache[key].Bounds().Size(); got != image.Pt(6, 10) {
			t.Errorf("scaled picture frame %d size = %v, want 6x10", frame, got)
		}
	}
}

func TestMobileUpscaleCachesAllPosesForOnlyObservedColors(t *testing.T) {
	isolateScaledArtworkCaches(t)
	isolateSourceFrameCaches(t)
	const (
		id     = 901
		factor = 2
	)
	observedColors := []byte{1, 2, 3}
	otherColors := []byte{4, 5, 6}
	imageMu.Lock()
	for state := uint8(0); state < 3; state++ {
		mobileCache[makeMobileKey(id, state, observedColors)] = ebiten.NewImage(4, 4)
	}
	mobileCache[makeMobileKey(id, 0, otherColors)] = ebiten.NewImage(4, 4)
	requestedKey := makeMobileKey(id, 1, observedColors)
	requested := mobileCache[requestedKey]
	imageMu.Unlock()

	if !cacheScaledMobileFramesWithReader(requestedKey, factor, artworkUpscaleBalanced, requested, transparentSpritePixels) {
		t.Fatal("mobile pose batch was invalidated while being built")
	}
	imageMu.Lock()
	defer imageMu.Unlock()
	if len(scaledMobileCache) != 3 {
		t.Fatalf("scaled mobile cache has %d poses, want 3", len(scaledMobileCache))
	}
	if len(scaledMobileBatches) != 1 {
		t.Errorf("completed mobile batches = %d, want 1", len(scaledMobileBatches))
	}
	for key, scaled := range scaledMobileCache {
		if key.colors != requestedKey.colors || key.colorsLen != requestedKey.colorsLen {
			t.Errorf("upscaled unobserved color variant: %#v", key.mobileKey)
		}
		if got := scaled.Bounds().Size(); got != image.Pt(8, 8) {
			t.Errorf("scaled mobile pose %d size = %v, want 8x8", key.state, got)
		}
	}
}

func TestScaledArtworkCacheDropsTexturesAboveNewScreenCap(t *testing.T) {
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })
	isolateScaledArtworkCaches(t)
	gs.SpriteUpscale = 4
	gs.SpriteUpscaleFilter = true
	gs.SpriteUpscaleMode = artworkUpscaleBalanced
	gs.GameScale = 2

	src := ebiten.NewImage(4, 4)
	key := makeMobileKey(447, 0, nil)
	if !cacheScaledMobileFramesWithReader(key, 4, artworkUpscaleBalanced, src, transparentSpritePixels) {
		t.Fatal("4x mobile batch was invalidated while being built")
	}
	large := getScaledMobileFrame(key, src)
	if got := large.Bounds().Size(); got != image.Pt(16, 16) {
		t.Fatalf("initial cached texture size = %v, want 16x16", got)
	}
	imageMu.Lock()
	mobileBlendCache[mobileBlendKey{from: key, to: key, step: 1, total: 2}] = newUnmanagedImage(16, 16)
	imageMu.Unlock()

	gs.GameScale = 1
	if !cacheScaledMobileFramesWithReader(key, 2, artworkUpscaleBalanced, src, transparentSpritePixels) {
		t.Fatal("2x mobile batch was invalidated while being built")
	}
	small := getScaledMobileFrame(key, src)
	if got := small.Bounds().Size(); got != image.Pt(8, 8) {
		t.Fatalf("screen-capped cached texture size = %v, want 8x8", got)
	}

	imageMu.Lock()
	defer imageMu.Unlock()
	if scaledCacheFactor != 2 {
		t.Errorf("cached artwork factor = %d, want 2", scaledCacheFactor)
	}
	if len(scaledMobileCache) != 1 {
		t.Errorf("scaled mobile cache has %d entries after cap change, want 1", len(scaledMobileCache))
	}
	if len(mobileBlendCache) != 0 {
		t.Errorf("mobile blend cache retained %d oversized entries", len(mobileBlendCache))
	}
	for cacheKey := range scaledMobileCache {
		if cacheKey.scale != 2 {
			t.Errorf("scaled mobile cache retained %dx entry after cap change", cacheKey.scale)
		}
	}
}
