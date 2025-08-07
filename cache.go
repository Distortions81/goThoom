package main

import (
	"log"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"

	"go_client/clsnd"
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

func precacheAssets() {
	if gs.LowMemory {
		return
	}
	if clImages != nil {
		for _, id := range clImages.IDs() {
			loadSheet(uint16(id), nil, false)
		}
	}

	soundMu.Lock()
	if clSounds == nil {
		snds, err := clsnd.Load(filepath.Join(dataDir, "CL_Sounds"))
		if err != nil {
			log.Printf("load CL_Sounds: %v", err)
		} else {
			snds.NoCache = gs.LowMemory
			clSounds = snds
		}
	}
	soundMu.Unlock()

	if clSounds != nil {
		for _, id := range clSounds.IDs() {
			loadSound(uint16(id))
		}
	}
}
