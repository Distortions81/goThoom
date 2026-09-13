package main

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"gothoom/climg"
)

func TestSceneArtworkPreparationDoesNotReuseAnotherSessionGeneration(t *testing.T) {
	originalImages := clImages
	clImages = &climg.CLImages{}
	t.Cleanup(func() { clImages = originalImages })

	imageMu.Lock()
	originalSheets := sheetCache
	sheetCache = make(map[sheetKey]*ebiten.Image)
	imageMu.Unlock()
	t.Cleanup(func() {
		imageMu.Lock()
		sheetCache = originalSheets
		imageMu.Unlock()
	})

	sceneArtworkRequests.Lock()
	originalRequests := sceneArtworkRequests.bySession
	sceneArtworkRequests.bySession = nil
	sceneArtworkRequests.Unlock()
	t.Cleanup(func() {
		sceneArtworkRequests.Lock()
		sceneArtworkRequests.bySession = originalRequests
		sceneArtworkRequests.Unlock()
	})

	first := drawSnapshot{source: 1, worldGeneration: 42, picsNeg: []framePicture{{PictID: 100}}}
	second := drawSnapshot{source: 2, worldGeneration: 42, picsNeg: []framePicture{{PictID: 200}}}
	if prepared := prepareSceneArtwork(first); prepared != 1 {
		t.Fatalf("first session prepared %d sheets, want 1", prepared)
	}
	if prepared := prepareSceneArtwork(second); prepared != 1 {
		t.Fatalf("second session prepared %d sheets, want 1 despite matching generation", prepared)
	}
}
