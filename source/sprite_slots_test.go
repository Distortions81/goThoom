package main

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func isolateSpriteSlots(t *testing.T) {
	t.Helper()
	originalSettings := gs
	originalPool := spriteSlots
	originalPictures, originalMobiles, originalMasks := scaledImageCache, scaledMobileCache, mobileRecolorMaskCache
	originalPictureBatches, originalMobileBatches, originalMaskBatches := scaledPictureBatches, scaledMobileBatches, mobileRecolorMaskBatches
	originalFactor := scaledCacheFactor
	originalFrame, originalUsage := spriteUsage.frame, spriteUsage.ids
	originalGeneration := artworkCacheGeneration.Load()
	spriteSlots = spriteSlotPool{}
	scaledImageCache = make(map[scaledImageKey]*ebiten.Image)
	scaledMobileCache = make(map[scaledMobileKey]*ebiten.Image)
	mobileRecolorMaskCache = make(map[scaledMobileKey]*ebiten.Image)
	scaledPictureBatches = make(map[scaledPictureBatchKey]struct{})
	scaledMobileBatches = make(map[scaledMobileBatchKey]struct{})
	mobileRecolorMaskBatches = make(map[scaledMobileBatchKey]struct{})
	spriteUsage.frame = 0
	clear(spriteUsage.ids[:])
	gs.PotatoGPU = false
	t.Cleanup(func() {
		clearScaledArtworkCachesLocked()
		spriteSlots = originalPool
		scaledImageCache, scaledMobileCache, mobileRecolorMaskCache = originalPictures, originalMobiles, originalMasks
		scaledPictureBatches, scaledMobileBatches, mobileRecolorMaskBatches = originalPictureBatches, originalMobileBatches, originalMaskBatches
		scaledCacheFactor = originalFactor
		spriteUsage.frame, spriteUsage.ids = originalFrame, originalUsage
		artworkCacheGeneration.Store(originalGeneration)
		gs = originalSettings
	})
}

func testSpriteSlotPicture(id uint16, frame uint16) spriteSlotKey {
	return spriteSlotKey{picture: scaledImageKey{imageKey: imageKey{id: id, frame: frame}, scale: 2, mode: artworkUpscaleBalanced}}
}

func insertTestSpriteSlot(key spriteSlotKey, pixels *image.RGBA, budget int64) *ebiten.Image {
	img := spriteSlots.upload(key, pixels, budget)
	switch key.kind {
	case 0:
		scaledImageCache[key.picture] = img
	case 1:
		scaledMobileCache[key.mobile] = img
	case 2:
		mobileRecolorMaskCache[key.mobile] = img
	}
	return img
}

func TestSpriteUsageCountsGameFramesOncePerID(t *testing.T) {
	isolateSpriteSlots(t)
	originalState := state
	t.Cleanup(func() { state = originalState })
	state.pictures = []framePicture{{PictID: 10}, {PictID: 10}, {PictID: 0xffff}}
	state.descriptors = map[uint8]frameDescriptor{1: {PictID: 10}, 2: {PictID: 20}, 3: {PictID: 30}}
	state.mobiles = map[uint8]frameMobile{1: {Index: 1}, 2: {Index: 2}}
	recordSpriteGameFrameLocked()
	snap := drawSnapshot{picsZero: state.pictures, descriptors: state.descriptors, mobiles: []frameMobile{{Index: 1}, {Index: 2}}}
	for range 60 {
		pinSceneSpriteSlots(snap)
	}
	if got := spriteUsage.ids[10]; got != (spriteIDUsage{lastFrame: 1, framesSeen: 1}) {
		t.Fatalf("duplicate references or render frames inflated usage: %+v", got)
	}
	if spriteUsage.ids[20].framesSeen != 1 || spriteUsage.ids[30].framesSeen != 0 || spriteUsage.ids[0xffff].framesSeen != 0 {
		t.Fatal("usage must count visible IDs, not stale descriptors or missing IDs")
	}
	state.logicalFrame = -100 // movie seeks must not move LRU time backwards
	state.pictures = nil
	delete(state.mobiles, 1)
	recordSpriteGameFrameLocked()
	if spriteUsage.ids[10].lastFrame != 1 || spriteUsage.ids[20] != (spriteIDUsage{lastFrame: 2, framesSeen: 2}) {
		t.Fatal("game frame recency did not advance independently of movie position")
	}
}

func TestSpriteSlotsEvictAllPosesAndMasksTogether(t *testing.T) {
	isolateSpriteSlots(t)
	pixels := image.NewRGBA(image.Rect(0, 0, 8, 8))
	budget := spriteSlotBytes(spriteSlotSize(pixels.Bounds())) * 4
	base := makeMobileKey(10, 0, []byte{3, 4})
	keys := []spriteSlotKey{
		testSpriteSlotPicture(10, 0), testSpriteSlotPicture(10, 1),
		{kind: 1, mobile: scaledMobileKey{mobileKey: base, scale: 2, mode: artworkUpscaleBalanced}},
		{kind: 2, mobile: scaledMobileKey{mobileKey: makeMobileKey(10, 0, nil), scale: 2, mode: artworkUpscaleBalanced}},
	}
	for _, key := range keys {
		insertTestSpriteSlot(key, pixels, budget)
	}
	markArtworkSheetBatchCompleteLocked(makeSheetKey(10, nil, false), 2, artworkUpscaleBalanced)
	markArtworkSheetBatchCompleteLocked(makeSheetKey(10, []byte{3, 4}, true), 2, artworkUpscaleBalanced)
	mobileRecolorMaskBatches[mobileRecolorBatchKey(10, 2, artworkUpscaleBalanced)] = struct{}{}
	parents := make(map[*ebiten.Image]bool)
	for _, slot := range spriteSlots.owners[10] {
		parents[slot.parent] = true
	}
	generation := artworkCacheGeneration.Load()
	insertTestSpriteSlot(testSpriteSlotPicture(20, 0), pixels, budget)
	if spriteSlots.count != 4 || spriteSlots.bytes != budget || spriteSlots.reuses != 1 || spriteSlots.evictions != 1 {
		t.Fatalf("compatible eviction changed allocations: %+v", spriteSlots)
	}
	if !parents[spriteSlots.owners[20][0].parent] {
		t.Fatal("reuse replaced the managed parent")
	}
	if len(scaledImageCache) != 1 || len(scaledMobileCache) != 0 || len(mobileRecolorMaskCache) != 0 ||
		len(scaledPictureBatches) != 0 || len(scaledMobileBatches) != 0 || len(mobileRecolorMaskBatches) != 0 {
		t.Fatal("eviction left a stale pose, palette variant, mask, or completed batch")
	}
	if artworkCacheGeneration.Load() <= generation {
		t.Fatal("scene request cache was not invalidated")
	}
}

func TestSpriteSlotReloadCountsIDResidenciesNotPoses(t *testing.T) {
	isolateSpriteSlots(t)
	pixels := image.NewRGBA(image.Rect(0, 0, 8, 8))
	const budget = 1 << 20
	insertTestSpriteSlot(testSpriteSlotPicture(10, 0), pixels, budget)
	insertTestSpriteSlot(testSpriteSlotPicture(10, 1), pixels, budget)
	if spriteSlots.loads != 1 || spriteSlots.reloads != 0 {
		t.Fatal("additional poses counted as sprite reloads")
	}
	spriteSlots.evict(10)
	insertTestSpriteSlot(testSpriteSlotPicture(20, 0), pixels, budget)
	if spriteSlots.loads != 2 || spriteSlots.reloads != 0 || spriteSlots.reuses != 1 {
		t.Fatal("reusing another sprite's allocation counted as a reload")
	}
	insertTestSpriteSlot(testSpriteSlotPicture(10, 0), pixels, budget)
	insertTestSpriteSlot(testSpriteSlotPicture(10, 1), pixels, budget)
	stats := imageCacheStats()
	if stats.slotLoads != 2 || stats.slotReloads != 1 {
		t.Fatalf("evicted sprite should count once when it returns: first=%d reloads=%d", stats.slotLoads, stats.slotReloads)
	}
}

func TestSpriteSlotsIdleReserveDoesNotForceEarlyEviction(t *testing.T) {
	isolateSpriteSlots(t)
	const budget = 256 << 10
	spriteSlots.init()
	// Fill the reserve with a shape unsuitable for these small sprites. That
	// spare capacity must not make a nearly empty live cache evict its contents.
	size := image.Pt(256, 256)
	spriteSlots.free[size] = append(spriteSlots.free[size], spriteSlots.allocate(size))
	pixels := image.NewRGBA(image.Rect(0, 0, 8, 8))
	insertTestSpriteSlot(testSpriteSlotPicture(10, 0), pixels, budget)
	insertTestSpriteSlot(testSpriteSlotPicture(20, 0), pixels, budget)
	if spriteSlots.evictions != 0 || len(spriteSlots.owners[10]) != 1 || len(spriteSlots.owners[20]) != 1 {
		t.Fatal("idle preallocation evicted sprites before the live cache reached its budget")
	}
	wantUsed := 2 * spriteSlotBytes(spriteSlotSize(pixels.Bounds()))
	if spriteSlots.usedBytes != wantUsed {
		t.Fatalf("occupied slot bytes = %d, want %d", spriteSlots.usedBytes, wantUsed)
	}
	spriteSlots.evict(10)
	if spriteSlots.usedBytes != wantUsed/2 {
		t.Fatal("eviction did not subtract the released slot area")
	}
	spriteSlots.clear()
	if spriteSlots.usedBytes != 0 {
		t.Fatal("clear retained occupied slot accounting")
	}
}

func TestSpriteSlotsReuseLargerFreeAllocationWithinPaddingLimit(t *testing.T) {
	isolateSpriteSlots(t)
	const budget = 1 << 20
	insertTestSpriteSlot(testSpriteSlotPicture(10, 0), image.NewRGBA(image.Rect(0, 0, 100, 60)), budget)
	parent := spriteSlots.owners[10][0].parent
	spriteSlots.evict(10)
	img := insertTestSpriteSlot(testSpriteSlotPicture(20, 0), image.NewRGBA(image.Rect(0, 0, 80, 48)), budget)
	if spriteSlots.count != 1 || spriteSlots.owners[20][0].parent != parent || img.Bounds() != image.Rect(0, 0, 80, 48) {
		t.Fatal("compatible larger allocation was not reused with the requested view")
	}
	spriteSlots.evict(20)
	insertTestSpriteSlot(testSpriteSlotPicture(30, 0), image.NewRGBA(image.Rect(0, 0, 8, 8)), budget)
	if spriteSlots.count != 2 {
		t.Fatal("tiny sprite consumed an allocation over twice its required area")
	}
}

func TestSpriteSlotsUseRecencyAndProtectInterpolationAndPreparation(t *testing.T) {
	isolateSpriteSlots(t)
	pixels := image.NewRGBA(image.Rect(0, 0, 8, 8))
	slotBytes := spriteSlotBytes(spriteSlotSize(pixels.Bounds()))
	for _, id := range []uint16{10, 20, 30} {
		insertTestSpriteSlot(testSpriteSlotPicture(id, 0), pixels, 3*slotBytes)
	}
	spriteUsage.ids[10] = spriteIDUsage{lastFrame: 1, framesSeen: 1000}
	spriteUsage.ids[20] = spriteIDUsage{lastFrame: 2, framesSeen: 1}
	spriteUsage.ids[30] = spriteIDUsage{lastFrame: 3, framesSeen: 1}
	pinSceneSpriteSlots(drawSnapshot{
		picsZero:    []framePicture{{PictID: 10}},
		prevMobiles: map[uint8]frameMobile{1: {Index: 1}},
		prevDescs:   map[uint8]frameDescriptor{1: {PictID: 20}},
	})
	unpin := pinPreparingSpriteSlots([]sheetKey{makeSheetKey(30, nil, false), makeSheetKey(30, nil, false)})
	insertTestSpriteSlot(testSpriteSlotPicture(40, 0), pixels, 3*slotBytes)
	if spriteSlots.count != 4 || spriteSlots.evictions != 0 {
		t.Fatal("a full pinned scene must grow beyond the reserve")
	}
	spriteUsage.ids[40] = spriteIDUsage{lastFrame: 4, framesSeen: 1}
	unpin()
	if len(spriteSlots.preparing) != 0 {
		t.Fatal("duplicate preparation pins leaked")
	}
	pinSceneSpriteSlots(drawSnapshot{})
	insertTestSpriteSlot(testSpriteSlotPicture(50, 0), pixels, 3*slotBytes)
	if _, ok := spriteSlots.owners[10]; ok {
		t.Fatal("frequent but least recently seen ID was not reclaimed")
	}
	if len(spriteSlots.owners[20]) != 1 || len(spriteSlots.owners[30]) != 1 || spriteSlots.count != 4 {
		t.Fatal("LRU reuse evicted recent IDs or released the overflow allocation")
	}
	// A new shape grows the pool without freeing incompatible managed nodes.
	insertTestSpriteSlot(testSpriteSlotPicture(60, 0), image.NewRGBA(image.Rect(0, 0, 100, 8)), 3*slotBytes)
	if spriteSlots.count != 5 || len(spriteSlots.owners[20]) != 1 {
		t.Fatal("new size destroyed incompatible slots")
	}
}

func TestSpriteSlotClearPreservesPinsDuringFactorChange(t *testing.T) {
	isolateSpriteSlots(t)
	pixels := image.NewRGBA(image.Rect(0, 0, 8, 8))
	insertTestSpriteSlot(testSpriteSlotPicture(10, 0), pixels, 1<<20)
	pinSceneSpriteSlots(drawSnapshot{picsZero: []framePicture{{PictID: 10}}})
	unpin := pinPreparingSpriteSlots([]sheetKey{makeSheetKey(20, nil, false)})
	defer unpin()
	scaledCacheFactor = 2
	ensureScaledArtworkCacheFactorLocked(4)
	if spriteSlots.count != 0 || spriteSlots.bytes != 0 || len(scaledImageCache) != 0 {
		t.Fatal("factor change retained stale textures")
	}
	if !spriteSlots.pinned[10] || spriteSlots.preparing[20] != 1 {
		t.Fatal("factor change lost active pins")
	}
}

func TestSpriteSlotStatsIncludeUnusedReserveWithoutCountingViewsTwice(t *testing.T) {
	isolateSpriteSlots(t)
	baseline := imageCacheStats().totalBytes()
	spriteSlots.preallocate(2, 1<<20)
	reserved := spriteSlots.bytes
	if got := imageCacheStats().totalBytes() - baseline; got != reserved {
		t.Fatalf("unused reserve = %d bytes, want %d", got, reserved)
	}
	insertTestSpriteSlot(testSpriteSlotPicture(10, 0), image.NewRGBA(image.Rect(0, 0, 64, 64)), 1<<20)
	if got := imageCacheStats().totalBytes() - baseline; got != reserved {
		t.Fatalf("occupied view counted separately from its slot: %d bytes, want %d", got, reserved)
	}
	spriteSlots.evict(10)
	if got := imageCacheStats().totalBytes() - baseline; got != reserved {
		t.Fatalf("eviction dropped allocated slot bytes: %d, want %d", got, reserved)
	}
}
