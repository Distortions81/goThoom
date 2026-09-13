package main

import (
	"image"
	"testing"
)

func TestHandleInfoTextParsesNight(t *testing.T) {
	*primarySession.night = NightInfo{}
	handleInfoText([]byte("/nt 83 /sa -1 /cl 1\r"))
	primarySession.night.mu.Lock()
	lvl := primarySession.night.BaseLevel
	az := primarySession.night.Azimuth
	cloudy := primarySession.night.Cloudy
	primarySession.night.mu.Unlock()
	if lvl != 83 || az != -1 || !cloudy {
		t.Fatalf("unexpected night values: level=%d az=%d cloudy=%v", lvl, az, cloudy)
	}
}

func TestParseLegacyNightCommandKeepsServerShadowLevel(t *testing.T) {
	*primarySession.night = NightInfo{}
	if !parseNightCommand("/nt 20 75 135 0") {
		t.Fatal("legacy night command was not parsed")
	}
	primarySession.night.mu.Lock()
	level := primarySession.night.Level
	shadows := primarySession.night.Shadows
	azimuth := primarySession.night.Azimuth
	primarySession.night.mu.Unlock()
	if level != 20 || shadows != 75 || azimuth != 135 {
		t.Fatalf("unexpected legacy night values: level=%d shadows=%d azimuth=%d", level, shadows, azimuth)
	}
}

func TestNightCommandUpdatesShadowProjection(t *testing.T) {
	*primarySession.night = NightInfo{}
	t.Cleanup(func() { *primarySession.night = NightInfo{} })

	handleInfoText([]byte("/nt 0 /sa 30 /cl 0\r"))
	first := newCharacterShadowProjection(primarySession.night.Azimuth)

	handleInfoText([]byte("/nt 0 /sa 90 /cl 0\r"))
	second := newCharacterShadowProjection(primarySession.night.Azimuth)

	if first.angle == second.angle || first.length == second.length {
		t.Fatalf("parsed sun update did not change projection: first=%+v second=%+v", first, second)
	}
}

func TestNightDarkInterpolationSettlesAtZero(t *testing.T) {
	originalNight := captureMovieNightState()
	originalForce := gs.forceNightLevel
	originalMax := gs.MaxNightLevel
	t.Cleanup(func() {
		restoreMovieNightState(originalNight)
		gs.forceNightLevel = originalForce
		gs.MaxNightLevel = originalMax
	})
	gs.forceNightLevel = -1
	gs.MaxNightLevel = 100
	state := &viewportRenderState{}

	primarySession.night.mu.Lock()
	primarySession.night.BaseLevel = 25
	primarySession.night.Level = 25
	primarySession.night.Flags = 0
	primarySession.night.mu.Unlock()
	addNightDarkSourcesForViewport(state, image.Rect(0, 0, 100, 100), 0.5, primarySession.night.snapshot())
	if state.nightTransition.current <= 0 {
		t.Fatal("positive night level did not initialize smoothing")
	}

	primarySession.night.mu.Lock()
	primarySession.night.BaseLevel = 0
	primarySession.night.Level = 0
	primarySession.night.mu.Unlock()
	state.lighting.darks = state.lighting.darks[:0]
	addNightDarkSourcesForViewport(state, image.Rect(0, 0, 100, 100), 0, primarySession.night.snapshot())
	if state.nightTransition.current != 0 || len(state.lighting.darks) == 0 {
		t.Fatalf("night transition to zero = target %v darks %d, want a one-frame fade", state.nightTransition.current, len(state.lighting.darks))
	}
	state.lighting.darks = state.lighting.darks[:0]
	addNightDarkSourcesForViewport(state, image.Rect(0, 0, 100, 100), 1, primarySession.night.snapshot())
	if len(state.lighting.darks) != 0 {
		t.Fatalf("night transition still dark at its endpoint: %d sources", len(state.lighting.darks))
	}

	// Starting the following game update must collapse both endpoints to zero;
	// otherwise the previous fade repeats from 0 to 100% every update.
	state.lighting.darks = state.lighting.darks[:0]
	addNightDarkSourcesForViewport(state, image.Rect(0, 0, 100, 100), 0, primarySession.night.snapshot())
	if state.nightTransition.previous != 0 || state.nightTransition.current != 0 || len(state.lighting.darks) != 0 {
		t.Fatalf("zero night repeated stale fade: prev=%v current=%v darks=%d", state.nightTransition.previous, state.nightTransition.current, len(state.lighting.darks))
	}
}
