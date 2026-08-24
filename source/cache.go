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
	mobileSpriteMetricsCache = make(map[mobileKey]mobileSpriteMetrics)
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
	// Image-backed UI rows reference textures owned by the caches above. Rebuild
	// them after a cache clear instead of leaving stale images in EUI until the
	// corresponding window is resized.
	inventoryDirty = true
	playersDirty = true
}

var soundsPrecached = false
var soundPrecacheProgress func(done, total int)

func precacheSounds() {

	for {
		if clSounds == nil {
			time.Sleep(time.Millisecond * 100)
		} else {
			break
		}
	}

	consoleMessage("Precaching game sounds...")

	total := len(clSounds.IDs())
	if soundPrecacheProgress != nil {
		soundPrecacheProgress(0, total)
	}

	var done int32
	wg := sizedwaitgroup.New(runtime.NumCPU())
	for _, id := range clSounds.IDs() {
		wg.Add()
		go func(id uint32) {
			loadSound(uint16(id))
			if soundPrecacheProgress != nil {
				n := int(atomic.AddInt32(&done, 1))
				soundPrecacheProgress(n, total)
			}
			wg.Done()
		}(id)
	}
	wg.Wait()
	if soundPrecacheProgress != nil {
		soundPrecacheProgress(total, total)
	}
	soundsPrecached = true

	consoleMessage("All sounds have been loaded.")
}
