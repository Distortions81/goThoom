package main

import (
	"image"
	"testing"
)

func TestHandleInfoTextParsesNight(t *testing.T) {
	gNight = NightInfo{}
	handleInfoText([]byte("/nt 83 /sa -1 /cl 1\r"))
	gNight.mu.Lock()
	lvl := gNight.BaseLevel
	az := gNight.Azimuth
	cloudy := gNight.Cloudy
	gNight.mu.Unlock()
	if lvl != 83 || az != -1 || !cloudy {
		t.Fatalf("unexpected night values: level=%d az=%d cloudy=%v", lvl, az, cloudy)
	}
}

func TestParseLegacyNightCommandKeepsServerShadowLevel(t *testing.T) {
	gNight = NightInfo{}
	if !parseNightCommand("/nt 20 75 135 0") {
		t.Fatal("legacy night command was not parsed")
	}
	gNight.mu.Lock()
	level := gNight.Level
	shadows := gNight.Shadows
	azimuth := gNight.Azimuth
	gNight.mu.Unlock()
	if level != 20 || shadows != 75 || azimuth != 135 {
		t.Fatalf("unexpected legacy night values: level=%d shadows=%d azimuth=%d", level, shadows, azimuth)
	}
}

func TestNightCommandUpdatesShadowProjection(t *testing.T) {
	gNight = NightInfo{}
	t.Cleanup(func() { gNight = NightInfo{} })

	handleInfoText([]byte("/nt 0 /sa 30 /cl 0\r"))
	first := newCharacterShadowProjection(gNight.Azimuth)

	handleInfoText([]byte("/nt 0 /sa 90 /cl 0\r"))
	second := newCharacterShadowProjection(gNight.Azimuth)

	if first.angle == second.angle || first.length == second.length {
		t.Fatalf("parsed sun update did not change projection: first=%+v second=%+v", first, second)
	}
}

func TestNightDarkInterpolationSettlesAtZero(t *testing.T) {
	originalNight := captureMovieNightState()
	originalForce := gs.forceNightLevel
	originalMax := gs.MaxNightLevel
	originalInited := nightAlphaInited
	originalLastT := nightLastT
	originalPrev := nightPrevTarget
	originalCurrent := nightCurTarget
	originalDarks := frameDarks
	t.Cleanup(func() {
		restoreMovieNightState(originalNight)
		gs.forceNightLevel = originalForce
		gs.MaxNightLevel = originalMax
		nightAlphaInited = originalInited
		nightLastT = originalLastT
		nightPrevTarget = originalPrev
		nightCurTarget = originalCurrent
		frameDarks = originalDarks
	})
	gs.forceNightLevel = -1
	gs.MaxNightLevel = 100
	nightAlphaInited = false
	frameDarks = frameDarks[:0]

	gNight.mu.Lock()
	gNight.BaseLevel = 25
	gNight.Level = 25
	gNight.Flags = 0
	gNight.mu.Unlock()
	addNightDarkSources(image.Rect(0, 0, 100, 100), 0.5)
	if nightCurTarget <= 0 {
		t.Fatal("positive night level did not initialize smoothing")
	}

	gNight.mu.Lock()
	gNight.BaseLevel = 0
	gNight.Level = 0
	gNight.mu.Unlock()
	frameDarks = frameDarks[:0]
	addNightDarkSources(image.Rect(0, 0, 100, 100), 0)
	if nightCurTarget != 0 || len(frameDarks) == 0 {
		t.Fatalf("night transition to zero = target %v darks %d, want a one-frame fade", nightCurTarget, len(frameDarks))
	}
	frameDarks = frameDarks[:0]
	addNightDarkSources(image.Rect(0, 0, 100, 100), 1)
	if len(frameDarks) != 0 {
		t.Fatalf("night transition still dark at its endpoint: %d sources", len(frameDarks))
	}

	// Starting the following game update must collapse both endpoints to zero;
	// otherwise the previous fade repeats from 0 to 100% every update.
	frameDarks = frameDarks[:0]
	addNightDarkSources(image.Rect(0, 0, 100, 100), 0)
	if nightPrevTarget != 0 || nightCurTarget != 0 || len(frameDarks) != 0 {
		t.Fatalf("zero night repeated stale fade: prev=%v current=%v darks=%d", nightPrevTarget, nightCurTarget, len(frameDarks))
	}
}
