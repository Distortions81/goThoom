package main

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

func clearCaches() {
	clearCharacterShadowCache()
	clearThoughtBubbleMask()

	imageCacheLifecycleMu.Lock()
	imageMu.Lock()
	clearScaledArtworkCachesLocked()
	scaledCacheFactor = 0
	imageCache = make(map[imageKey]*ebiten.Image)
	sheetCache = make(map[sheetKey]*ebiten.Image)
	mobileCache = make(map[mobileKey]*ebiten.Image)
	mobileSpriteMetricsCache = make(map[mobileKey]mobileSpriteMetrics)
	imageMu.Unlock()

	pixelCountMu.Lock()
	pixelCountCache = make(map[uint16]int)
	pixelCountMu.Unlock()

	soundMu.Lock()
	pcmCache = make(map[soundPCMKey][]byte)
	soundCacheGeneration++
	restartSoundPrecache := gs.PrecacheSounds && clSounds != nil && audioContext != nil
	soundMu.Unlock()
	soundsPrecached.Store(false)
	if restartSoundPrecache {
		go precacheSounds()
	}

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

func clearScaledArtworkCachesLocked() {
	for _, img := range scaledImageCache {
		if img != nil {
			img.Deallocate()
		}
	}
	for _, img := range scaledMobileCache {
		if img != nil {
			img.Deallocate()
		}
	}
	scaledImageCache = make(map[scaledImageKey]*ebiten.Image)
	scaledMobileCache = make(map[scaledMobileKey]*ebiten.Image)
	scaledPictureBatches = make(map[scaledPictureBatchKey]struct{})
	scaledMobileBatches = make(map[scaledMobileBatchKey]struct{})
}

var (
	soundsPrecached      atomic.Bool
	soundPrecacheRunning atomic.Bool
)

func precacheSounds() {
	if soundsPrecached.Load() || !soundPrecacheRunning.CompareAndSwap(false, true) {
		return
	}
	defer soundPrecacheRunning.Store(false)
	for {
		soundMu.Lock()
		sounds := clSounds
		context := audioContext
		highQuality := highQualityResampling
		cacheGeneration := soundCacheGeneration
		soundMu.Unlock()
		if sounds != nil && context != nil {
			ids := sounds.IDs()
			precacheSoundIDs(ids, context.SampleRate(), highQuality)
			soundMu.Lock()
			current := sounds == clSounds && context == audioContext && highQuality == highQualityResampling && cacheGeneration == soundCacheGeneration
			soundMu.Unlock()
			if current {
				soundsPrecached.Store(true)
				consoleMessage("All sounds have been loaded.")
				return
			}
			continue
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func precacheSoundIDs(ids []uint32, outputRate int, highQuality bool) {
	consoleMessage("Precaching game sounds...")
	total := len(ids)
	if total == 0 {
		return
	}

	workerCount := soundPrecacheWorkerCount(runtime.GOMAXPROCS(0))
	jobs := make(chan uint32)
	completed := make(chan struct{}, total)
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for range workerCount {
		go func() {
			defer workers.Done()
			for id := range jobs {
				loadSoundForPlayback(uint16(id), outputRate, highQuality)
				completed <- struct{}{}
			}
		}()
	}
	for _, id := range ids {
		jobs <- id
	}
	close(jobs)
	for range total {
		<-completed
	}
	workers.Wait()
}

func soundPrecacheWorkerCount(gomaxprocs int) int {
	return min(2, max(1, gomaxprocs-1))
}
