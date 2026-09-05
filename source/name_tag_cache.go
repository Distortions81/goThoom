package main

import (
	"gothoom/internal/renderpool"
	"image/color"
	"math"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
)

type cachedNameTagImage struct {
	image         *ebiten.Image
	width, height int
	bytes         int64
	lastUsed      uint64
	borrowers     int
	retired       bool
}

const maxSharedNameTags = 4096
const maxSharedNameTagBytes = 64 << 20
const nameTagRasterScaleUnits = 64

var (
	sharedNameTagMu    sync.RWMutex
	sharedNameTagBytes int64
	sharedNameTagClock uint64
	nameTagTargets     = renderpool.Pool{MaxFreeBytes: 8 << 20, NewImage: newBubbleRenderTarget, Deallocate: deallocateImage}
	sharedNameTagCache = make(map[nameTagKey]*cachedNameTagImage)
)

func nameTagFrameColor(name string, opacity uint8) color.RGBA {
	if !gs.NameTagLabelColors {
		return color.RGBA{}
	}
	playersMu.RLock()
	defer playersMu.RUnlock()
	p, ok := players[name]
	if !ok || p.FriendLabel <= 0 || p.FriendLabel > len(labelColors) {
		return color.RGBA{}
	}
	lc := labelColors[p.FriendLabel-1]
	return color.RGBA{R: lc.R, G: lc.G, B: lc.B, A: opacity}
}

func quantizedNameTagRasterScale(scale float64) (uint16, float64) {
	if math.IsNaN(scale) || math.IsInf(scale, 0) || scale <= 0 {
		scale = 1
	}
	units := int(math.Round(scale * nameTagRasterScaleUnits))
	units = min(max(units, 1), math.MaxUint16)
	return uint16(units), float64(units) / nameTagRasterScaleUnits
}

func nameTagRasterScaleFromKey(key uint16) float64 {
	if key == 0 {
		return 1
	}
	return float64(key) / nameTagRasterScaleUnits
}

func makeNameTagKey(name string, colors, descriptorType, opacity, style uint8, dead bool, rasterScale float64) nameTagKey {
	// Modern tags do not derive their text surface from the health color or
	// descriptor type; the health bar is composed separately at draw time.
	if gs.NameHealthBarModern {
		colors = 0
		descriptorType = 0
	}
	rasterScaleKey, _ := quantizedNameTagRasterScale(rasterScale)
	return nameTagKey{
		Text:          name,
		Colors:        colors,
		Type:          descriptorType,
		HealthOptions: nameHealthOptionsKey(),
		Opacity:       opacity,
		FontGen:       fontGen,
		RasterScale:   rasterScaleKey,
		Style:         style,
		Dead:          dead,
		FrameColor:    nameTagFrameColor(name, opacity),
	}
}

// Mobile snapshots can retain old image pointers after eviction. Those pointers
// are hints for parsing only; rendering borrows the current cache entry by key.
func reuseSharedNameTag(m *frameMobile, key nameTagKey) bool {
	sharedNameTagMu.Lock()
	defer sharedNameTagMu.Unlock()
	cached := sharedNameTagCache[key]
	if cached == nil || cached.image == nil {
		return false
	}
	touchSharedNameTag(cached)
	m.nameTag, m.nameTagW, m.nameTagH, m.nameTagKey = cached.image, cached.width, cached.height, key
	return true
}

func touchSharedNameTag(entry *cachedNameTagImage) {
	sharedNameTagClock++
	entry.lastUsed = sharedNameTagClock
}

func sharedNameTagImage(key nameTagKey) (*ebiten.Image, int, int) {
	sharedNameTagMu.Lock()
	defer sharedNameTagMu.Unlock()
	entry := sharedNameTagImageLocked(key)
	if entry == nil {
		return nil, 0, 0
	}
	return entry.image, entry.width, entry.height
}

func sharedNameTagImageLocked(key nameTagKey) *cachedNameTagImage {
	if entry := sharedNameTagCache[key]; entry != nil {
		touchSharedNameTag(entry)
		return entry
	}
	rasterScale := nameTagRasterScaleFromKey(key.RasterScale)
	// Evict before acquiring storage so the next label can reuse an idle slot.
	for len(sharedNameTagCache) >= maxSharedNameTags || sharedNameTagBytes >= maxSharedNameTagBytes {
		if !evictOldestSharedNameTag() {
			break
		}
	}
	img, width, height := buildNameTagImageWithTarget(key.Text, key.Colors, key.Type, key.Opacity, key.Style, key.Dead, key.FrameColor, rasterScale,
		func(w, h int) *ebiten.Image { return nameTagTargets.Acquire(w, h, gs.PotatoGPU) })
	if img == nil {
		return nil
	}
	entry := &cachedNameTagImage{image: img, width: width, height: height, bytes: nameTagTargets.Bytes(img)}
	for sharedNameTagBytes+entry.bytes > maxSharedNameTagBytes {
		if !evictOldestSharedNameTag() {
			break
		}
	}
	touchSharedNameTag(entry)
	sharedNameTagCache[key] = entry
	sharedNameTagBytes += entry.bytes
	return entry
}

// Borrowing protects the image until DrawImage has queued its use. Repainting
// its slot after release is then safe because Ebitengine orders those commands.
func borrowSharedNameTag(key nameTagKey) *cachedNameTagImage {
	sharedNameTagMu.Lock()
	defer sharedNameTagMu.Unlock()
	entry := sharedNameTagImageLocked(key)
	if entry != nil {
		entry.borrowers++
	}
	return entry
}

func releaseSharedNameTag(entry *cachedNameTagImage) {
	if entry == nil {
		return
	}
	sharedNameTagMu.Lock()
	defer sharedNameTagMu.Unlock()
	entry.borrowers--
	if entry.retired && entry.borrowers == 0 {
		nameTagTargets.Release(entry.image)
	}
}

func retireSharedNameTag(key nameTagKey, entry *cachedNameTagImage) {
	delete(sharedNameTagCache, key)
	sharedNameTagBytes -= entry.bytes
	entry.retired = true
	if entry.borrowers == 0 {
		nameTagTargets.Release(entry.image)
	}
}

// Called with sharedNameTagMu held. Never retire an entry in active use just
// to meet a budget; an oversized label or active borrowers may exceed it briefly.
func evictOldestSharedNameTag() bool {
	var oldest *cachedNameTagImage
	var oldestKey nameTagKey
	for key, entry := range sharedNameTagCache {
		if entry.borrowers == 0 && (oldest == nil || entry.lastUsed < oldest.lastUsed) {
			oldest, oldestKey = entry, key
		}
	}
	if oldest == nil {
		return false
	}
	retireSharedNameTag(oldestKey, oldest)
	return true
}

func clearSharedNameTagCache() {
	sharedNameTagMu.Lock()
	defer sharedNameTagMu.Unlock()
	for key, entry := range sharedNameTagCache {
		retireSharedNameTag(key, entry)
	}
	nameTagTargets.Clear()
}

func clearSharedNameTagCacheFor(name string) {
	sharedNameTagMu.Lock()
	defer sharedNameTagMu.Unlock()
	for key, entry := range sharedNameTagCache {
		if key.Text == name {
			retireSharedNameTag(key, entry)
		}
	}
}

// killNameTagCache clears all cached mobile name tag images.
func killNameTagCache() {
	stateMu.Lock()
	for idx, m := range state.mobiles {
		m.nameTag = nil
		m.nameTagKey = nameTagKey{}
		state.mobiles[idx] = m
	}
	stateMu.Unlock()
	clearSharedNameTagCache()
}

// killNameTagCacheFor clears the cached name tag for the mobile with the given name.
func killNameTagCacheFor(name string) {
	stateMu.Lock()
	for idx, d := range state.descriptors {
		if d.Name == name {
			if m, ok := state.mobiles[idx]; ok {
				m.nameTag = nil
				m.nameTagKey = nameTagKey{}
				state.mobiles[idx] = m
			}
		}
	}
	stateMu.Unlock()
	clearSharedNameTagCacheFor(name)
}
