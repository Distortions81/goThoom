package main

import (
	"runtime"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/remeh/sizedwaitgroup"
)

func clearCaches() {
	imageMu.Lock()
	imageCache = make(map[imageKey]*ebiten.Image)
	sheetCache = make(map[sheetKey]*ebiten.Image)
	mobileCache = make(map[mobileKey]*ebiten.Image)
	mobileBlendCache = make(map[mobileBlendKey]*ebiten.Image)
	pictBlendCache = make(map[pictBlendKey]*ebiten.Image)
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
	if clSounds != nil {
		clSounds.ClearCache()
	}
}

var assetsPrecached = false

func precacheAssets() {

	for {
		if (gs.precacheImages && clImages == nil) || (gs.precacheSounds && clSounds == nil) {
			time.Sleep(time.Millisecond * 100)
		} else {
			break
		}
	}

	var preloadMsg string
	switch {
	case gs.precacheImages && gs.precacheSounds:
		preloadMsg = "Precaching game sounds and images..."
	case gs.precacheImages:
		preloadMsg = "Precaching game images..."
	case gs.precacheSounds:
		preloadMsg = "Precaching game sounds..."
	}
	if preloadMsg != "" {
		consoleMessage(preloadMsg)
	}

	wg := sizedwaitgroup.New(runtime.NumCPU())
	if gs.precacheImages && clImages != nil {
		for _, id := range clImages.IDs() {
			wg.Add()
			go func(id uint32) {
				loadSheet(uint16(id), nil, false)
				wg.Done()
			}(id)
		}
	}

	if gs.precacheSounds && clSounds != nil {
		for _, id := range clSounds.IDs() {
			wg.Add()
			go func(id uint32) {
				loadSound(uint16(id))
				wg.Done()
			}(id)
		}
	}
	wg.Wait()
	assetsPrecached = true

	var doneMsg string
	switch {
	case gs.precacheImages && gs.precacheSounds:
		doneMsg = "All images and sounds have been loaded."
	case gs.precacheImages:
		doneMsg = "All images have been loaded."
	case gs.precacheSounds:
		doneMsg = "All sounds have been loaded."
	}
	if doneMsg != "" {
		consoleMessage(doneMsg)
	}
}
