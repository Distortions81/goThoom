package main

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestViewportLightingScratchIsIndependent(t *testing.T) {
	states := [maxSessions]viewportRenderState{}
	scratches := make([]*ebiten.Image, 0, maxSessions)
	for index := range states {
		bounds := image.Rect(0, 0, 100, 80)
		if index == len(states)-1 {
			bounds = image.Rect(0, 0, 120, 80)
		}
		scratch := ensureViewportLightingTmp(&states[index], bounds)
		for _, existing := range scratches {
			if scratch == existing {
				t.Fatalf("viewport %d shared a lighting scratch texture", index+1)
			}
		}
		scratches = append(scratches, scratch)
	}
	t.Cleanup(func() {
		for _, scratch := range scratches {
			scratch.Deallocate()
		}
	})

	if got := ensureViewportLightingTmp(&states[0], image.Rect(0, 0, 100, 80)); got != scratches[0] {
		t.Fatal("rendering the other viewports replaced the first viewport's lighting scratch texture")
	}
}

func TestSceneLightingScanIsIndependentPerSession(t *testing.T) {
	original := sceneLightingScans
	sceneLightingScans = [maxSessions + 1]sceneLightingScanState{}
	t.Cleanup(func() { sceneLightingScans = original })

	first := sceneLightingScanForSession(1)
	first.valid = true
	first.worldGeneration = 42
	first.hasEmitters = true

	second := sceneLightingScanForSession(2)
	if second == first {
		t.Fatal("two sessions shared a lighting scan")
	}
	if second.valid || second.worldGeneration != 0 || second.hasEmitters {
		t.Fatalf("second session inherited first session lighting scan: %+v", second)
	}
}

func TestSessionNightStateKeepsIndoorAndOutdoorLightingIndependent(t *testing.T) {
	outdoor, err := newSession(2)
	if err != nil {
		t.Fatal(err)
	}
	indoor, err := newSession(3)
	if err != nil {
		t.Fatal(err)
	}

	handleSessionInfoText(outdoor, []byte("/nt 80 /sa 90 /cl 0\r"))
	handleSessionInfoText(indoor, []byte("/nt 80 /sa 90 /cl 0\r"))
	outdoor.night.setFlags(0, 100)
	indoor.night.setFlags(kLightNoNightMods|kLightNoShadows, 100)

	outdoorNight := outdoor.night.snapshot()
	indoorNight := indoor.night.snapshot()
	if got := effectiveNightLevel(outdoorNight); got != 80 {
		t.Fatalf("outdoor night level = %d, want 80", got)
	}
	if got := effectiveNightLevel(indoorNight); got != 0 {
		t.Fatalf("indoor night level = %d, want 0", got)
	}
	if outdoorNight.flags == indoorNight.flags {
		t.Fatal("indoor session inherited the outdoor session's area lighting flags")
	}

	var outdoorSnap, indoorSnap drawSnapshot
	captureSessionDrawSnapshot(outdoor, &outdoorSnap)
	captureSessionDrawSnapshot(indoor, &indoorSnap)
	if outdoorSnap.night.level == indoorSnap.night.level {
		t.Fatal("session draw snapshots collapsed distinct effective night levels")
	}
}

func TestViewportNightTransitionsAreIndependent(t *testing.T) {
	var first, second viewportRenderState
	first.nightTransition.update(shaderNightStrength, 0.5)
	second.nightTransition.update(0, 0.5)
	if first.nightTransition.current == second.nightTransition.current {
		t.Fatal("two viewports shared a night transition target")
	}
	if second.nightTransition.previous != 0 {
		t.Fatal("second viewport inherited the first viewport's fade state")
	}
}

func TestViewportLightingFramesAreIndependent(t *testing.T) {
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })
	gs.ShaderLighting = true
	gs.MobileLightConeShadows = true
	gs.GameScale = 1

	var first, second viewportRenderState
	metrics := mobileSpriteMetrics{widthFraction: 0.5, footFraction: 0.9}
	addMobileLightCasterForViewport(&first, 20, 30, 40, metrics)
	addMobileLightCasterForViewport(&second, 80, 90, 20, metrics)
	if len(first.lighting.casters) != 1 || len(second.lighting.casters) != 1 {
		t.Fatalf("caster counts = (%d, %d), want one per viewport", len(first.lighting.casters), len(second.lighting.casters))
	}
	if first.lighting.casters[0].X == second.lighting.casters[0].X {
		t.Fatal("second viewport inherited the first viewport's caster position")
	}

	firstNight := nightRenderState{level: 75}
	secondNight := nightRenderState{level: 25}
	addNightDarkSourcesForViewport(&first, image.Rect(0, 0, 100, 80), 1, firstNight)
	addNightDarkSourcesForViewport(&second, image.Rect(0, 0, 60, 40), 1, secondNight)
	if len(first.lighting.darks) == 0 || len(second.lighting.darks) == 0 {
		t.Fatalf("dark counts = (%d, %d), want sources in both viewports", len(first.lighting.darks), len(second.lighting.darks))
	}
	if first.lighting.darks[0].Radius == second.lighting.darks[0].Radius {
		t.Fatal("viewport dark sources used shared scene bounds")
	}

	first.lighting.lights = append(first.lighting.lights, lightSource{X: 12})
	first.lighting.shadows = append(first.lighting.shadows, lightShadow{CasterX: 13})
	if len(second.lighting.lights) != 0 || len(second.lighting.shadows) != 0 {
		t.Fatal("viewport lighting slices share backing state")
	}
}

func TestPackViewportRenderRectsChoosesTiledAtlas(t *testing.T) {
	sizes := []image.Point{{X: 320, Y: 240}, {X: 320, Y: 240}, {X: 320, Y: 240}, {X: 320, Y: 240}}
	rects, size, ok := packViewportRenderRects(sizes, 4096)
	if !ok {
		t.Fatal("four ordinary viewports did not fit in the render atlas")
	}
	wantSize := image.Pt(320*2+viewportRenderAtlasGap, 240*2+viewportRenderAtlasGap)
	if size != wantSize {
		t.Fatalf("atlas size = %v, want %v", size, wantSize)
	}
	for index, rect := range rects {
		if rect.Size() != sizes[index] {
			t.Fatalf("atlas rect %d size = %v, want %v", index, rect.Size(), sizes[index])
		}
		for other := range rects[:index] {
			if rect.Overlaps(rects[other]) {
				t.Fatalf("atlas rects %d and %d overlap: %v and %v", index, other, rect, rects[other])
			}
		}
	}
}

func TestPackViewportRenderRectsRejectsDeviceLimit(t *testing.T) {
	if _, _, ok := packViewportRenderRects([]image.Point{{X: 500, Y: 300}, {X: 500, Y: 300}}, 512); ok {
		t.Fatal("render atlas exceeded the device image limit")
	}
}
