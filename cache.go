package main

import (
	"github.com/hajimehoshi/ebiten/v2"
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
