package main

import (
	"runtime"
	"sync/atomic"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/remeh/sizedwaitgroup"
)

func clearCaches() {
	clearCharacterShadowCache()
	clearThoughtBubbleMask()

	imageCacheLifecycleMu.Lock()
	imageMu.Lock()
	imageCache = make(map[imageKey]*ebiten.Image)
	sheetCache = make(map[sheetKey]*ebiten.Image)
	mobileCache = make(map[mobileKey]*ebiten.Image)
	mobileBlendCache = make(map[mobileBlendKey]*ebiten.Image)
	pictBlendCache = make(map[pictBlendKey]*ebiten.Image)
	scaledImageCache = make(map[scaledImageKey]*ebiten.Image)
	scaledMobileCache = make(map[scaledMobileKey]*ebiten.Image)
	imageMu.Unlock()

	pixelCountMu.Lock()
	pixelCountCache = make(map[uint16]int)
	pixelCountMu.Unlock()

	soundMu.Lock()
	pcmCache = make(map[uint16][]byte)
	soundMu.Unlock()

	if clImages != nil {
		clImages.ClearCache()
	}
	imageCacheLifecycleMu.Unlock()
	if clSounds != nil {
		clSounds.ClearCache()
	}

	// Image-backed UI rows reference textures owned by the caches above. Rebuild
	// them after a cache clear instead of leaving stale images in EUI until the
	// corresponding window is resized.
	inventoryDirty = true
	playersDirty = true
}

var assetsPrecached = false
var precacheProgress func(done, total int)

func precacheAssets() {

	for {
		if (gs.PrecacheImages && clImages == nil) || (gs.PrecacheSounds && clSounds == nil) {
			time.Sleep(time.Millisecond * 100)
		} else {
			break
		}
	}

	var preloadMsg string
	switch {
	case gs.PrecacheImages && gs.PrecacheSounds:
		preloadMsg = "Precaching game sounds and images..."
	case gs.PrecacheImages:
		preloadMsg = "Precaching game images..."
	case gs.PrecacheSounds:
		preloadMsg = "Precaching game sounds..."
	}
	if preloadMsg != "" {
		consoleMessage(preloadMsg)
	}

	var total int
	if gs.PrecacheImages && clImages != nil {
		total += len(clImages.IDs())
	}
	if gs.PrecacheSounds && clSounds != nil {
		total += len(clSounds.IDs())
	}
	if precacheProgress != nil {
		precacheProgress(0, total)
	}

	var done int32
	wg := sizedwaitgroup.New(runtime.NumCPU())
	if gs.PrecacheImages && clImages != nil {
		for _, id := range clImages.IDs() {
			wg.Add()
			go func(id uint32) {
				loadSheet(uint16(id), nil, false)
				if precacheProgress != nil {
					n := int(atomic.AddInt32(&done, 1))
					precacheProgress(n, total)
				}
				wg.Done()
			}(id)
		}
	}

	if gs.PrecacheSounds && clSounds != nil {
		for _, id := range clSounds.IDs() {
			wg.Add()
			go func(id uint32) {
				loadSound(uint16(id))
				if precacheProgress != nil {
					n := int(atomic.AddInt32(&done, 1))
					precacheProgress(n, total)
				}
				wg.Done()
			}(id)
		}
	}
	wg.Wait()
	if precacheProgress != nil {
		precacheProgress(total, total)
	}
	assetsPrecached = true

	var doneMsg string
	switch {
	case gs.PrecacheImages && gs.PrecacheSounds:
		doneMsg = "All images and sounds have been loaded."
	case gs.PrecacheImages:
		doneMsg = "All images have been loaded."
	case gs.PrecacheSounds:
		doneMsg = "All sounds have been loaded."
	}
	if doneMsg != "" {
		consoleMessage(doneMsg)
	}
}
