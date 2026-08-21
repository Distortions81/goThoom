package main

import "testing"

func TestMobileSizeUsesImageMetadata(t *testing.T) {
	origImages := clImages
	clImages = mockCLImages(160, 80)
	defer func() { clImages = origImages }()

	if got := mobileSize(1); got != 10 {
		t.Fatalf("mobileSize(1) = %d, want 10", got)
	}
}

func TestUpdateWorldHoverCachesUnchangedQuery(t *testing.T) {
	stateMu.Lock()
	origState := state
	state = drawState{
		descriptors: map[uint8]frameDescriptor{1: {Index: 1, Name: "Bob", PictID: 100}},
		liveMobs:    []frameMobile{{Index: 1, H: 0, V: 0}},
	}
	stateMu.Unlock()

	origMobileSizeFunc := mobileSizeFunc
	sizeCalls := 0
	mobileSizeFunc = func(uint16) int {
		sizeCalls++
		return 10
	}

	origGeneration := worldStateGeneration.Load()
	lastHoverMu.Lock()
	origHover := lastHover
	origHoverGeneration := lastHoverGeneration
	origHoverQueryValid := lastHoverQueryValid
	lastHoverQueryValid = false
	lastHoverMu.Unlock()
	defer func() {
		stateMu.Lock()
		state = origState
		stateMu.Unlock()
		mobileSizeFunc = origMobileSizeFunc
		worldStateGeneration.Store(origGeneration)
		lastHoverMu.Lock()
		lastHover = origHover
		lastHoverGeneration = origHoverGeneration
		lastHoverQueryValid = origHoverQueryValid
		lastHoverMu.Unlock()
	}()

	updateWorldHover(0, 0)
	updateWorldHover(0, 0)
	if sizeCalls != 1 {
		t.Fatalf("unchanged hover query performed %d size lookups, want 1", sizeCalls)
	}

	stateMu.Lock()
	markWorldStateChanged()
	stateMu.Unlock()
	updateWorldHover(0, 0)
	if sizeCalls != 2 {
		t.Fatalf("state change did not invalidate hover query; calls = %d", sizeCalls)
	}
}
