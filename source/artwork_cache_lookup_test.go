package main

import (
	"encoding/binary"
	"testing"

	"gothoom/climg"

	"github.com/hajimehoshi/ebiten/v2"
)

func isolateArtworkLookupCaches(t testing.TB) {
	t.Helper()
	originalSettings, originalImages := gs, clImages
	originalSheets, originalPool := sheetCache, spriteSlots
	originalPictures, originalMobiles, originalMasks := scaledImageCache, scaledMobileCache, mobileRecolorMaskCache
	originalPictureBatches, originalMobileBatches, originalMaskBatches := scaledPictureBatches, scaledMobileBatches, mobileRecolorMaskBatches
	originalFactor := scaledCacheFactor
	gs.SpriteUpscale, gs.DenoiseImages = 2, false
	setArtworkUpscaleMode(artworkUpscaleBalanced)
	clImages = &climg.CLImages{}
	sheetCache = make(map[sheetKey]*ebiten.Image)
	spriteSlots = spriteSlotPool{}
	scaledImageCache = make(map[scaledImageKey]*ebiten.Image)
	scaledMobileCache = make(map[scaledMobileKey]*ebiten.Image)
	mobileRecolorMaskCache = make(map[scaledMobileKey]*ebiten.Image)
	scaledPictureBatches = make(map[scaledPictureBatchKey]struct{})
	scaledMobileBatches = make(map[scaledMobileBatchKey]struct{})
	mobileRecolorMaskBatches = make(map[scaledMobileBatchKey]struct{})
	scaledCacheFactor = 2
	t.Cleanup(func() {
		gs, clImages = originalSettings, originalImages
		sheetCache, spriteSlots = originalSheets, originalPool
		scaledImageCache, scaledMobileCache, mobileRecolorMaskCache = originalPictures, originalMobiles, originalMasks
		scaledPictureBatches, scaledMobileBatches, mobileRecolorMaskBatches = originalPictureBatches, originalMobileBatches, originalMaskBatches
		scaledCacheFactor = originalFactor
	})
}

func cacheArtworkLookupFixture(t testing.TB, key sheetKey) *ebiten.Image {
	t.Helper()
	img := ebiten.NewImage(1, 1)
	t.Cleanup(img.Deallocate)
	sheetCache[key] = img
	markArtworkSheetBatchCompleteLocked(key, 2, artworkUpscaleBalanced)
	return img
}

func TestScaledArtworkCacheHitsDoNotAllocate(t *testing.T) {
	isolateArtworkLookupCaches(t)
	picture := cacheArtworkLookupFixture(t, makeSheetKey(100, nil, false))
	colors := []byte{1, 2, 3}
	key := makeMobileKey(101, 10, colors)
	mobile := cacheArtworkLookupFixture(t, makeSheetKey(key.id, colors, true))
	if allocs := testing.AllocsPerRun(100, func() {
		if !cacheScaledPictureFrames(100, 1, 2, 2, artworkUpscaleBalanced, picture) {
			t.Fatal("completed picture batch was not cached")
		}
		if !cacheScaledMobileFrames(key, 2, artworkUpscaleBalanced, mobile) {
			t.Fatal("completed mobile batch was not cached")
		}
	}); allocs != 0 {
		t.Fatalf("warm picture and mobile lookups allocated %g times", allocs)
	}
}

func TestScaledArtworkCacheRequiresCompleteMatchingBatch(t *testing.T) {
	for _, mobile := range []bool{false, true} {
		name := "Picture"
		if mobile {
			name = "Mobile"
		}
		t.Run(name, func(t *testing.T) {
			isolateArtworkLookupCaches(t)
			key := makeSheetKey(100, nil, mobile)
			img := cacheArtworkLookupFixture(t, key)
			ready := func() bool { return scaledArtworkSheetReady(key, 2, artworkUpscaleBalanced) }
			if !ready() {
				t.Fatal("complete sheet was not ready")
			}
			if scaledArtworkSheetReady(key, 3, artworkUpscaleBalanced) || scaledArtworkSheetReady(key, 2, artworkUpscaleSmooth) {
				t.Fatal("different scale or style reused a completed batch")
			}
			otherID := key
			otherID.id++
			if scaledArtworkSheetReady(otherID, 2, artworkUpscaleBalanced) {
				t.Fatal("different sprite reused a completed batch")
			}
			if mobile && scaledArtworkSheetReady(makeSheetKey(key.id, []byte{1}, true), 2, artworkUpscaleBalanced) {
				t.Fatal("different mobile palette reused a completed batch")
			}
			delete(sheetCache, key)
			if ready() {
				t.Fatal("missing base sheet skipped preparation")
			}
			sheetCache[key] = img
			scaledCacheFactor = 3
			if ready() {
				t.Fatal("old factor skipped preparation")
			}
			scaledCacheFactor = 2
			// Evicting any frame makes the whole batch incomplete, even if
			// the sheet and other frames remain available.
			slotKey := spriteSlotKey{picture: scaledImageKey{imageKey: makeImageKey(key.id, 1), scale: 2, mode: artworkUpscaleBalanced}}
			if mobile {
				slotKey = spriteSlotKey{kind: 1, mobile: scaledMobileKey{mobileKey: makeMobileKey(key.id, 10, nil), scale: 2, mode: artworkUpscaleBalanced}}
			}
			slotKey.invalidate()
			if ready() {
				t.Fatal("partial batch skipped preparation after eviction")
			}
			markArtworkSheetBatchCompleteLocked(key, 2, artworkUpscaleBalanced)
			if !ready() {
				t.Fatal("rebuilt batch was not ready")
			}
			clearScaledArtworkCachesLocked()
			if ready() {
				t.Fatal("cleared batch skipped preparation")
			}
		})
	}
}

func TestScaledArtworkCacheRequiresNewlyEligibleRecolorMasks(t *testing.T) {
	isolateArtworkLookupCaches(t)
	const id = 100
	// A minimal archive entry supplies the custom-color flag; no pixel data
	// is needed to exercise eligibility and cache invalidation.
	archive := make([]byte, 48)
	binary.BigEndian.PutUint16(archive[0:2], 0xffff)
	binary.BigEndian.PutUint32(archive[2:6], 1)
	binary.BigEndian.PutUint16(archive[10:12], 2)
	binary.BigEndian.PutUint32(archive[12:16], 28)
	binary.BigEndian.PutUint32(archive[16:20], 20)
	binary.BigEndian.PutUint32(archive[20:24], climg.TYPE_IDREF)
	binary.BigEndian.PutUint32(archive[24:28], id)
	binary.BigEndian.PutUint32(archive[44:48], 0x2000)
	var err error
	clImages, err = climg.LoadBytes(archive)
	if err != nil {
		t.Fatal(err)
	}
	key := makeSheetKey(id, nil, true)
	cacheArtworkLookupFixture(t, key)
	originalShader, originalBlend := mobileRecolorShader, mobileRecolorBlendShader
	t.Cleanup(func() { mobileRecolorShader, mobileRecolorBlendShader = originalShader, originalBlend })
	mobileRecolorShader, mobileRecolorBlendShader = nil, nil
	ready := func() bool { return scaledArtworkSheetReady(key, 2, artworkUpscaleBalanced) }
	if !ready() {
		t.Fatal("unavailable recolor shaders required a mask")
	}
	mobileRecolorShader, mobileRecolorBlendShader = &ebiten.Shader{}, &ebiten.Shader{}
	if !mobileRecolorSourceEligible(key) {
		t.Fatal("fixture did not enable recolor masks")
	}
	if ready() {
		t.Fatal("newly eligible recolor mask skipped preparation")
	}
	batch := mobileRecolorBatchKey(id, 2, artworkUpscaleBalanced)
	mobileRecolorMaskBatches[batch] = struct{}{}
	if !ready() {
		t.Fatal("complete recolor batch was not ready")
	}
	maskKey := scaledMobileKey{mobileKey: makeMobileKey(id, 10, nil), scale: 2, mode: artworkUpscaleBalanced}
	(spriteSlotKey{kind: 2, mobile: maskKey}).invalidate()
	if ready() {
		t.Fatal("evicted recolor mask skipped preparation")
	}
	gs.DenoiseImages = true
	if !ready() {
		t.Fatal("disabled recoloring still required a mask")
	}
}

func BenchmarkScaledArtworkCacheHit(b *testing.B) {
	b.Run("Picture", func(b *testing.B) {
		isolateArtworkLookupCaches(b)
		img := cacheArtworkLookupFixture(b, makeSheetKey(100, nil, false))
		b.ReportAllocs()
		for b.Loop() {
			if !cacheScaledPictureFrames(100, 1, 2, 2, artworkUpscaleBalanced, img) {
				b.Fatal("completed picture batch was not cached")
			}
		}
	})
	b.Run("Mobile", func(b *testing.B) {
		isolateArtworkLookupCaches(b)
		colors := []byte{1, 2, 3}
		key := makeMobileKey(101, 10, colors)
		img := cacheArtworkLookupFixture(b, makeSheetKey(key.id, colors, true))
		b.ReportAllocs()
		for b.Loop() {
			if !cacheScaledMobileFrames(key, 2, artworkUpscaleBalanced, img) {
				b.Fatal("completed mobile batch was not cached")
			}
		}
	})
}
