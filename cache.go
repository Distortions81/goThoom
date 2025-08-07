package main

import (
	"runtime"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/remeh/sizedwaitgroup"
)

func clearCaches() {
	imageMu.Lock()
	imageCache = make(map[string]*ebiten.Image)
	sheetCache = make(map[string]*ebiten.Image)
	mobileCache = make(map[string]*ebiten.Image)
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

	poolMu.Lock()
	imgPool = make(map[int][]*ebiten.Image)
	poolMu.Unlock()
}

var assetsPrecached = false

func precacheAssets() {
	for {
		if clImages == nil || clSounds == nil {
			time.Sleep(time.Millisecond * 100)
		} else {
			break
		}
	}

	wg := sizedwaitgroup.New(runtime.NumCPU())
	if clImages != nil {
		for _, id := range clImages.IDs() {
			wg.Add()
			go func(id uint32) {
				loadSheet(uint16(id), nil, false)
				wg.Done()
			}(id)
		}
	}

	if clSounds != nil {
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
	addMessage("All images and sounds have been loaded.")
}
