package main

import "testing"

func TestWorldRenderCanBeReusedWithoutMotionSmoothing(t *testing.T) {
	originalMotion := gs.MotionSmoothing
	originalPreview := setupWizardPreviewActive
	originalTorture := bubbleTorture
	originalEffectsPreview := replacementEffectsPreview
	defer func() {
		gs.MotionSmoothing = originalMotion
		setupWizardPreviewActive = originalPreview
		bubbleTorture = originalTorture
		replacementEffectsPreview = originalEffectsPreview
	}()

	gs.MotionSmoothing = false
	setupWizardPreviewActive = false
	bubbleTorture = false
	replacementEffectsPreview = false
	key := worldRenderKey{worldGeneration: 10, renderGeneration: 20, width: 640, height: 480}
	game := &Game{lastWorldRenderKey: key, worldRenderValid: true}

	if !worldRenderCanBeReused(game, key) {
		t.Fatal("unchanged world was not reused with motion smoothing disabled")
	}
	changed := key
	changed.worldGeneration++
	if worldRenderCanBeReused(game, changed) {
		t.Fatal("new server world generation reused the old world image")
	}
	changed = key
	changed.renderGeneration++
	if worldRenderCanBeReused(game, changed) {
		t.Fatal("non-server render update reused the old world image")
	}
}

func TestWorldRenderIsContinuousWithMotionSmoothing(t *testing.T) {
	originalMotion := gs.MotionSmoothing
	defer func() { gs.MotionSmoothing = originalMotion }()

	gs.MotionSmoothing = true
	key := worldRenderKey{worldGeneration: 10, width: 640, height: 480}
	game := &Game{lastWorldRenderKey: key, worldRenderValid: true}
	if worldRenderCanBeReused(game, key) {
		t.Fatal("motion-smoothed world reused a frame")
	}
}
