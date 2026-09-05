package main

// Opt-in, metadata-only replay of the production slot policy. This measures
// reload pressure and allocation area, not CPU upscale time or physical VRAM.
import (
	"encoding/binary"
	"encoding/json"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"gothoom/climg"
)

type spriteMovieRegion struct {
	index int
	size  image.Point
}

type spriteMovieSheet struct {
	regions []spriteMovieRegion
	mask    bool
}

type spriteMovieResult struct {
	Scale, ReserveMiB                     int
	FirstIDs, ReloadIDs, EvictedIDs       uint64
	FirstBatches, ReloadBatches           int
	BatchRequests, BatchHits              int
	FirstSlots, ReloadSlots, ReloadFrames int
	FirstUploadMiB, ReloadUploadMiB       float64
	LargestReloadFrameMiB                 float64
	AllocatedMiB, IdleMiB                 float64
}

type spriteMovieCache struct {
	pool                  spriteSlotPool
	result                spriteMovieResult
	cached                map[sheetKey]uint64 // residency generation for completed batches
	history               map[sheetKey]bool
	firstBytes, redoBytes int64
}

func newSpriteMovieCache(scale, reserve int) *spriteMovieCache {
	c := &spriteMovieCache{result: spriteMovieResult{Scale: scale, ReserveMiB: reserve}, cached: make(map[sheetKey]uint64), history: make(map[sheetKey]bool)}
	c.pool.init()
	// Use the real reserve distribution and allocation bookkeeping, but do not
	// WritePixels. Ebitengine images remain unallocated on the GPU in this probe.
	forEachReservedSpriteSlot(scale, int64(reserve)<<20, func(size image.Point) {
		c.pool.free[size] = append(c.pool.free[size], c.pool.allocate(size))
	})
	return c
}

func (c *spriteMovieCache) request(key sheetKey, sheet spriteMovieSheet) {
	if len(sheet.regions) == 0 {
		return
	}
	c.result.BatchRequests++
	if generation, ok := c.cached[key]; ok && len(c.pool.owners[key.id]) != 0 && generation == c.pool.loadCounts[key.id] {
		c.result.BatchHits++
		return
	}
	reload := c.history[key]
	if reload {
		c.result.ReloadBatches++
	} else {
		c.result.FirstBatches++
	}
	c.history[key] = true
	for _, region := range sheet.regions {
		width, height := region.size.X*c.result.Scale, region.size.Y*c.result.Scale
		keyForSlot := spriteSlotKey{picture: scaledImageKey{imageKey: makeImageKey(key.id, region.index), scale: uint8(c.result.Scale), mode: artworkUpscaleBalanced}}
		if key.forceTransparent {
			keyForSlot = spriteSlotKey{kind: 1, mobile: scaledMobileKey{mobileKey: makeMobileKey(key.id, uint8(region.index), sheetKeyColors(&key)), scale: uint8(c.result.Scale), mode: artworkUpscaleBalanced}}
		}
		upload := func(k spriteSlotKey) {
			slot := c.pool.take(spriteSlotSize(image.Rect(0, 0, width, height)), int64(c.result.ReserveMiB)<<20)
			c.pool.noteUpload(slot, k, int64(width)*int64(height)*4)
			// Production writes the entire parent, including transparent padding.
			bytes := spriteSlotBytes(slot.size)
			if reload {
				c.result.ReloadSlots++
				c.redoBytes += bytes
			} else {
				c.result.FirstSlots++
				c.firstBytes += bytes
			}
		}
		upload(keyForSlot)
		if sheet.mask {
			keyForSlot.kind = 2
			upload(keyForSlot)
		}
	}
	c.cached[key] = c.pool.loadCounts[key.id]
}

func (c *spriteMovieCache) finish() spriteMovieResult {
	r := c.result
	r.FirstIDs, r.ReloadIDs, r.EvictedIDs = c.pool.loads, c.pool.reloads, c.pool.evictions
	r.FirstUploadMiB, r.ReloadUploadMiB = float64(c.firstBytes)/(1<<20), float64(c.redoBytes)/(1<<20)
	r.AllocatedMiB = float64(c.pool.bytes) / (1 << 20)
	for size, slots := range c.pool.free {
		r.IdleMiB += float64(spriteSlotBytes(size)*int64(len(slots))) / (1 << 20)
	}
	return r
}

func TestSpriteCacheMovieReloadPressure(t *testing.T) {
	out := os.Getenv("GOTHOOM_SPRITE_CACHE_STUDY")
	if out == "" {
		t.Skip("set GOTHOOM_SPRITE_CACHE_STUDY to an output directory and GOTHOOM_SPRITE_CACHE_IMAGES to CL_Images")
	}
	isolateSpriteSlots(t)
	var err error
	clImages, err = climg.Load(os.Getenv("GOTHOOM_SPRITE_CACHE_IMAGES"))
	if err != nil {
		t.Fatal(err)
	}
	gs = gsdef
	gs.MotionSmoothing, gs.BlendMobiles, gs.BlendPicts = true, true, true
	gs.ShadersEnabled, gs.DenoiseImages, gs.PotatoGPU = true, false, false
	playingMovie, movieMode, drawStateEncrypted = true, true, false
	blockSound, blockMusic, blockTTS = true, true, true
	dataDirPath = t.TempDir()
	initFont()
	if err := ReloadSpriteUpscaleShader(); err != nil {
		t.Fatal(err)
	}
	_, sourceFile, _, _ := runtime.Caller(0)
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(sourceFile), "clmovFiles", "*.zip"))
	if err != nil {
		t.Fatal(err)
	}
	if selected := os.Getenv("GOTHOOM_SPRITE_CACHE_MOVIE"); selected != "" {
		paths = []string{selected}
	}
	if len(paths) == 0 {
		t.Fatal("no movie inputs")
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	sheets := make(map[sheetKey]spriteMovieSheet)
	getSheet := func(key sheetKey) spriteMovieSheet {
		if s, ok := sheets[key]; ok {
			return s
		}
		s := spriteMovieSheet{mask: mobileRecolorSourceEligible(key)}
		pixels := clImages.DecodeRGBA(uint32(key.id), sheetKeyColors(&key), key.forceTransparent)
		if pixels != nil {
			for index, rect := range artworkRegionRects(key, pixels) {
				visible := false
				for y := rect.Min.Y; y < rect.Max.Y && !visible; y++ {
					for x := rect.Min.X; x < rect.Max.X; x++ {
						if pixels.Pix[pixels.PixOffset(x, y)+3] != 0 {
							visible = true
							break
						}
					}
				}
				if visible {
					s.regions = append(s.regions, spriteMovieRegion{index: index, size: rect.Size()})
				}
			}
		}
		sheets[key] = s
		return s
	}
	for _, path := range paths {
		resetDrawState()
		frameCounter, lastAckFrame, movieDropped = 0, 0, 0
		spriteUsage.frame = 0
		clear(spriteUsage.ids[:])
		frames, err := parseMovie(path, clVersion)
		if err != nil {
			t.Fatal(err)
		}
		playerName = extractMoviePlayerName(frames)
		restoreDrawState(initialState)
		var caches []*spriteMovieCache
		for _, scale := range []int{2, 4} {
			for _, reserve := range []int{128, 256, 512, 1024, 2048} {
				caches = append(caches, newSpriteMovieCache(scale, reserve))
			}
		}
		packets, rejected := 0, 0
		seenIDs := make(map[uint16]bool)
		var keys []sheetKey
		for _, frame := range frames {
			movieDropped = updateFrameCounters(frame.index)
			if len(frame.data) < 2 || binary.BigEndian.Uint16(frame.data[:2]) != 2 {
				frameCounter++
				continue
			}
			if !handleDrawState(frame.data, true) {
				rejected++
				continue
			}
			packets++
			var snap drawSnapshot
			captureDrawSnapshot(&snap)
			keys = appendSceneArtworkKeys(keys[:0], snap, true)
			unique := make(map[sheetKey]bool)
			requested := keys[:0]
			for _, key := range keys {
				if key.id == 0xffff || replacementEffectReplacesPict(key.id) || unique[key] {
					continue
				}
				unique[key] = true
				requested = append(requested, key)
				if len(getSheet(key).regions) != 0 {
					seenIDs[key.id] = true
				}
			}
			for _, cache := range caches {
				cache.pool.pinScene(snap)
				before := cache.redoBytes
				for _, key := range requested {
					cache.request(key, sheets[key])
				}
				if delta := cache.redoBytes - before; delta != 0 {
					cache.result.ReloadFrames++
					cache.result.LargestReloadFrameMiB = max(cache.result.LargestReloadFrameMiB, float64(delta)/(1<<20))
				}
			}
		}
		frequency := make(map[string]int)
		if packets == 0 || len(seenIDs) == 0 {
			t.Fatalf("%s produced no usable sprite requests", path)
		}
		for id := range seenIDs {
			n := spriteUsage.ids[id].framesSeen
			switch {
			case n <= 5:
				frequency["1-5 game frames"]++
			case n <= 25:
				frequency["6-25 game frames"]++
			default:
				frequency["26+ game frames"]++
			}
		}
		var results []spriteMovieResult
		for _, cache := range caches {
			results = append(results, cache.finish())
			cache.pool.clear()
		}
		sort.Slice(results, func(i, j int) bool {
			if results[i].Scale != results[j].Scale {
				return results[i].Scale < results[j].Scale
			}
			return results[i].ReserveMiB < results[j].ReserveMiB
		})
		report := map[string]any{"movie": filepath.Base(path), "packets": packets, "rejected": rejected, "uniqueIDs": len(seenIDs), "frequency": frequency, "results": results}
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, filepath.Base(path)+".json"), data, 0600); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %d game frames, %d IDs, %d rejected packets", filepath.Base(path), packets, len(seenIDs), rejected)
	}
}
