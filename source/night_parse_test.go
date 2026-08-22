package main

import "testing"

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
