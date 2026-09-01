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
	noteFrameAtlasFactorChange(scaledCacheFactor, 0)
	clearScaledArtworkCachesLocked()
	scaledCacheFactor = 0
	for _, img := range sheetCache {
		deallocateImage(img)
	}
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
		go precacheStartupSounds(true)
	}

	if clImages != nil {
		clImages.ClearCache()
	}
	imageCacheLifecycleMu.Unlock()
	artworkCacheGeneration.Add(1)
	// Image-backed UI rows reference textures owned by the caches above. Rebuild
	// them after a cache clear instead of leaving stale images in EUI until the
	// corresponding window is resized.
	inventoryDirty = true
	playersDirty = true
}

func clearScaledArtworkCachesLocked() {
	clearedImages := 0
	clearedBytes := 0
	trace := currentAssetLoadFrameTrace()
	for _, img := range scaledImageCache {
		if img != nil {
			if trace != nil {
				bounds := img.Bounds()
				clearedImages++
				clearedBytes += bounds.Dx() * bounds.Dy() * 4
			}
			deallocateImage(img)
		}
	}
	for _, img := range scaledMobileCache {
		if img != nil {
			if trace != nil {
				bounds := img.Bounds()
				clearedImages++
				clearedBytes += bounds.Dx() * bounds.Dy() * 4
			}
			deallocateImage(img)
		}
	}
	if trace != nil && clearedImages != 0 {
		noteFrameAtlasCacheClear(clearedImages, clearedBytes)
	}
	scaledImageCache = make(map[scaledImageKey]*ebiten.Image)
	scaledMobileCache = make(map[scaledMobileKey]*ebiten.Image)
	scaledPictureBatches = make(map[scaledPictureBatchKey]struct{})
	scaledMobileBatches = make(map[scaledMobileBatchKey]struct{})
}

var (
	artworkCacheGeneration atomic.Uint64
	soundsPrecached        atomic.Bool
	soundPrecacheRunning   atomic.Bool
)

var startupSoundPreloadIDs = [...]uint16{
	205, 131, 59, 43, 65, 132, 276, 278, 20, 48,
}

// precacheStartupSounds warms the small set most frequently referenced by the
// bundled clmov files before optionally continuing into the full sound cache.
func precacheStartupSounds(precacheAll bool) {
	soundMu.Lock()
	context := audioContext
	highQuality := highQualityResampling
	soundMu.Unlock()
	if context != nil {
		for _, id := range startupSoundPreloadIDs {
			loadSoundForPlayback(id, context.SampleRate(), highQuality)
		}
	}
	if precacheAll {
		precacheSounds()
	}
}

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
