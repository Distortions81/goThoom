package main

import (
	"image"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
)

const defaultSpriteCacheMiB = 512

// Recency is measured in accepted game updates, not interpolated Draw calls.
// A separate lock keeps packet processing independent of texture uploads.
type spriteIDUsage struct {
	lastFrame  uint64
	framesSeen uint64
}

var spriteUsage struct {
	sync.Mutex
	frame uint64
	ids   [1 << 16]spriteIDUsage
}

// Called with stateMu held, after the new picture and mobile lists are installed.
func recordSpriteGameFrameLocked() {
	spriteUsage.Lock()
	defer spriteUsage.Unlock()
	spriteUsage.frame++
	record := func(id uint16) {
		if id == 0xffff {
			return
		}
		u := &spriteUsage.ids[id]
		if u.lastFrame != spriteUsage.frame {
			u.lastFrame = spriteUsage.frame
			u.framesSeen++
		}
	}
	for _, p := range state.pictures {
		record(p.PictID)
	}
	for index := range state.mobiles {
		if d, ok := state.descriptors[index]; ok {
			record(d.PictID)
		}
	}
}

type spriteSlotKey struct {
	picture scaledImageKey
	mobile  scaledMobileKey
	kind    uint8 // 0: picture, 1: mobile, 2: recolor mask
}

func (k spriteSlotKey) id() uint16 {
	if k.kind == 0 {
		return k.picture.id
	}
	return k.mobile.id
}

func (k spriteSlotKey) invalidate() {
	if k.kind == 0 {
		delete(scaledImageCache, k.picture)
		delete(scaledPictureBatches, scaledPictureBatchKey{id: k.picture.id, scale: k.picture.scale, mode: k.picture.mode})
		return
	}
	base := k.mobile.mobileKey
	base.state = 0
	batch := scaledMobileBatchKey{mobileKey: base, scale: k.mobile.scale, mode: k.mobile.mode}
	if k.kind == 1 {
		delete(scaledMobileCache, k.mobile)
		delete(scaledMobileBatches, batch)
	} else {
		delete(mobileRecolorMaskCache, k.mobile)
		delete(mobileRecolorMaskBatches, batch)
	}
}

type spriteSlot struct {
	parent       *ebiten.Image
	size         image.Point
	key          spriteSlotKey
	contentBytes int64
	written      bool
}

type spriteSlotPool struct {
	free       map[image.Point][]*spriteSlot
	owners     map[uint16][]*spriteSlot
	pinned     map[uint16]bool
	preparing  map[uint16]int
	bytes      int64
	usedBytes  int64 // occupied slot area; idle reserve does not create eviction pressure
	count      int
	reuses     uint64
	evictions  uint64 // complete sprite IDs, including all their poses and masks
	loads      uint64 // first residency of a sprite ID
	reloads    uint64 // new residency after that ID was evicted
	loadCounts map[uint16]uint64
}

// All pool access is protected by imageMu. Parents stay allocated on eviction;
// only their cache views and ownership change. Ebitengine still packs the atlas.
var spriteSlots spriteSlotPool

func spriteCacheMiB(value int) int {
	if value <= 0 {
		return defaultSpriteCacheMiB
	}
	return max(128, min(8192, value))
}

// The saved reserve is a 2x reference size. Scale by texture area to retain
// comparable artwork at 3x and 4x, using the actual uploaded texture factor.
func scaledSpriteCacheMiB(value, factor int) int {
	factor = max(1, min(4, factor))
	return min(8192, spriteCacheMiB(value)*factor*factor/4)
}

func spriteSlotSize(bounds image.Rectangle) image.Point {
	// Keep a transparent texel beyond the view for linear sampling. Quantize
	// each dimension independently so narrow animations need not be square.
	return image.Pt((bounds.Dx()+1+31)/32*32, (bounds.Dy()+1+31)/32*32)
}

func spriteSlotBytes(size image.Point) int64 { return int64(size.X) * int64(size.Y) * 4 }

func spriteSlotFits(available, requested image.Point) bool {
	return available.X >= requested.X && available.Y >= requested.Y && spriteSlotBytes(available) <= 2*spriteSlotBytes(requested)
}

func (p *spriteSlotPool) freeSize(size image.Point) (image.Point, bool) {
	// Prefer exact fits, then borrow a bounded amount of already-idle space.
	// Eviction still requires the requested size: reclaiming a larger animation
	// batch for one small sprite can turn its next use into a large reload burst.
	if len(p.free[size]) != 0 {
		return size, true
	}
	var best image.Point
	found := false
	for available, slots := range p.free {
		if len(slots) == 0 || !spriteSlotFits(available, size) {
			continue
		}
		bytes, bestBytes := spriteSlotBytes(available), spriteSlotBytes(best)
		if !found || bytes < bestBytes || bytes == bestBytes && available.X < best.X {
			best, found = available, true
		}
	}
	return best, found
}

func (p *spriteSlotPool) init() {
	if p.free == nil {
		p.free = make(map[image.Point][]*spriteSlot)
	}
	if p.owners == nil {
		p.owners = make(map[uint16][]*spriteSlot)
	}
	if p.loadCounts == nil {
		p.loadCounts = make(map[uint16]uint64)
	}
}

func (p *spriteSlotPool) allocate(size image.Point) *spriteSlot {
	slot := &spriteSlot{parent: newManagedImage(size.X, size.Y), size: size}
	p.bytes += spriteSlotBytes(size)
	p.count++
	return slot
}

func (p *spriteSlotPool) evict(id uint16) {
	for _, slot := range p.owners[id] {
		slot.key.invalidate()
		p.usedBytes -= spriteSlotBytes(slot.size)
		p.free[slot.size] = append(p.free[slot.size], slot)
	}
	delete(p.owners, id)
	p.evictions++
	artworkCacheGeneration.Add(1)
}

// Prefer an old ID with compatible slots. Evict its whole pose batch so that
// the next first-use load can prepare that batch once, rather than thrashing
// individual poses. Frequency is recorded, but the policy is LRU, not LFU.
func (p *spriteSlotPool) oldest(size image.Point) (uint16, bool) {
	spriteUsage.Lock()
	defer spriteUsage.Unlock()
	var oldest uint16
	var age uint64
	found := false
	for id, slots := range p.owners {
		if p.pinned[id] || p.preparing[id] != 0 {
			continue
		}
		fits := false
		for _, slot := range slots {
			if slot.size == size {
				fits = true
				break
			}
		}
		if !fits {
			continue
		}

		last := spriteUsage.ids[id].lastFrame
		if !found || last < age || (last == age && id < oldest) {
			oldest, age, found = id, last, true
		}
	}
	return oldest, found
}

// The reserve is a soft target, not a VRAM limit. Never release a parent
// during normal play: if the size mix changes or every compatible slot is
// pinned, grow and retain the new allocation for subsequent scenes.
// When no free slot fits, only occupied slot area creates eviction pressure.
// Counting idle preallocation here would evict sprites from a nearly empty cache.
func (p *spriteSlotPool) take(size image.Point, budget int64) *spriteSlot {
	p.init()
	available, haveFree := p.freeSize(size)
	if !haveFree && p.usedBytes+spriteSlotBytes(size) > budget {
		if id, ok := p.oldest(size); ok {
			p.evict(id)
			available, haveFree = p.freeSize(size)
		}
	}
	if haveFree {
		slots := p.free[available]
		slot := slots[len(slots)-1]
		p.free[available] = slots[:len(slots)-1]
		return slot
	}
	return p.allocate(size)
}

func (p *spriteSlotPool) upload(key spriteSlotKey, src *image.RGBA, budget int64) *ebiten.Image {
	slot := p.take(spriteSlotSize(src.Bounds()), budget)
	// Write the entire allocation, including transparent padding. This also
	// removes pixels left by a larger previous occupant without a GPU clear.
	pixels := acquireArtworkRGBA(image.Rectangle{Max: slot.size})
	clear(pixels.Pix)
	for y := 0; y < src.Bounds().Dy(); y++ {
		from := src.PixOffset(src.Bounds().Min.X, src.Bounds().Min.Y+y)
		copy(pixels.Pix[y*pixels.Stride:y*pixels.Stride+src.Bounds().Dx()*4], src.Pix[from:from+src.Bounds().Dx()*4])
	}
	slot.parent.WritePixels(pixels.Pix)
	releaseArtworkRGBA(pixels)
	p.noteUpload(slot, key, int64(src.Bounds().Dx())*int64(src.Bounds().Dy())*4)
	return slot.parent.SubImage(image.Rect(0, 0, src.Bounds().Dx(), src.Bounds().Dy())).(*ebiten.Image)
}

func (p *spriteSlotPool) noteUpload(slot *spriteSlot, key spriteSlotKey, contentBytes int64) {
	id := key.id()
	if len(p.owners[id]) == 0 {
		if p.loadCounts[id] == 0 {
			p.loads++
		} else {
			p.reloads++
		}
		p.loadCounts[id]++
	}
	if slot.written {
		p.reuses++
	}
	slot.written = true
	slot.key = key
	slot.contentBytes = contentBytes
	p.usedBytes += spriteSlotBytes(slot.size)
	p.owners[id] = append(p.owners[id], slot)
}

func cachedSpriteSlotLocked(key spriteSlotKey, src *image.RGBA) *ebiten.Image {
	factor := int(key.picture.scale)
	if key.kind != 0 {
		factor = int(key.mobile.scale)
	}
	return spriteSlots.upload(key, src, int64(scaledSpriteCacheMiB(gs.SpriteCacheMiB, factor))<<20)
}

func pinSceneSpriteSlots(snap drawSnapshot) {
	imageMu.Lock()
	defer imageMu.Unlock()
	spriteSlots.pinScene(snap)
}

func (p *spriteSlotPool) pinScene(snap drawSnapshot) {
	if p.pinned == nil {
		p.pinned = make(map[uint16]bool)
	}
	clear(p.pinned)
	for _, pictures := range [][]framePicture{snap.picsNeg, snap.picsZero, snap.picsPos} {
		for _, picture := range pictures {
			p.pinned[picture.PictID] = true
		}
	}
	for _, mobile := range snap.mobiles {
		if d, ok := snap.descriptors[mobile.Index]; ok {
			p.pinned[d.PictID] = true
		}
	}
	for index := range snap.prevMobiles {
		d, ok := snap.prevDescs[index]
		if !ok {
			d, ok = snap.descriptors[index]
		}
		if ok {
			p.pinned[d.PictID] = true
		}
	}
}

func pinPreparingSpriteSlots(keys []sheetKey) func() {
	imageMu.Lock()
	if spriteSlots.preparing == nil {
		spriteSlots.preparing = make(map[uint16]int)
	}
	for _, key := range keys {
		spriteSlots.preparing[key.id]++
	}
	imageMu.Unlock()
	return func() {
		imageMu.Lock()
		for _, key := range keys {
			spriteSlots.preparing[key.id]--
			if spriteSlots.preparing[key.id] == 0 {
				delete(spriteSlots.preparing, key.id)
			}
		}
		imageMu.Unlock()
	}
}

func (p *spriteSlotPool) clear() {
	for _, slots := range p.free {
		for _, slot := range slots {
			deallocateImage(slot.parent)
		}
	}
	for _, slots := range p.owners {
		for _, slot := range slots {
			deallocateImage(slot.parent)
		}
	}
	// Preserve scene and preparation pins across upscale-factor changes.
	*p = spriteSlotPool{pinned: p.pinned, preparing: p.preparing}
}

// Reserve common square pose sizes while the startup loading screen is up.
// Larger/narrow artwork adjusts this mix on demand; no custom atlas is needed.
func preallocateSpriteSlots() {
	if !artworkUpscaleEnabled() || gs.PotatoGPU {
		return
	}
	imageCacheLifecycleMu.RLock()
	defer imageCacheLifecycleMu.RUnlock()
	imageMu.Lock()
	defer imageMu.Unlock()
	factor := artworkUpscaleFactor()
	ensureScaledArtworkCacheFactorLocked(factor)
	spriteSlots.preallocate(factor, int64(scaledSpriteCacheMiB(gs.SpriteCacheMiB, factor))<<20)
}

func (p *spriteSlotPool) preallocate(factor int, budget int64) {
	p.init()
	reserve := max(int64(0), budget-p.bytes)
	forEachReservedSpriteSlot(factor, reserve, func(size image.Point) {
		slot := p.allocate(size)
		// Two pixels force allocation without uploading a slot-sized zero
		// buffer. A one-pixel write is deferred by Ebitengine. Every slot
		// receives a full overwrite before it is handed to the renderer.
		slot.parent.SubImage(image.Rect(0, 0, 2, 1)).(*ebiten.Image).WritePixels(make([]byte, 8))
		p.free[size] = append(p.free[size], slot)
	})
}

func forEachReservedSpriteSlot(factor int, reserve int64, visit func(image.Point)) {
	// Allocate large slots first so small slots can fill the remaining atlas gaps.
	for _, class := range []struct{ native, percent int }{{512, 10}, {256, 20}, {128, 20}, {64, 15}, {32, 35}} {
		size := spriteSlotSize(image.Rect(0, 0, class.native*factor, class.native*factor))
		bytes := spriteSlotBytes(size)
		for n := reserve * int64(class.percent) / 100 / bytes; n > 0; n-- {
			visit(size)
		}
	}
}
