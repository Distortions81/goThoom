package main

import "testing"

func TestParseNightCommandRegex(t *testing.T) {
	gNight = NightInfo{}
	if !parseNightCommand("/nt 40 /sa 200 /cl 0") {
		t.Fatalf("parseNightCommand failed")
	}
	gNight.mu.Lock()
	defer gNight.mu.Unlock()
	if gNight.BaseLevel != 40 || gNight.Level != 40 || gNight.Shadows != 10 || gNight.Azimuth != 200 || gNight.Cloudy {
		t.Fatalf("unexpected night info: base=%d level=%d shadows=%d az=%d cloudy=%v", gNight.BaseLevel, gNight.Level, gNight.Shadows, gNight.Azimuth, gNight.Cloudy)
	}
}

func TestParseNightCommandV100(t *testing.T) {
	gNight = NightInfo{}
	if !parseNightCommand("/nt 30 15 250 5") {
		t.Fatalf("parseNightCommand failed")
	}
	gNight.mu.Lock()
	defer gNight.mu.Unlock()
	if gNight.BaseLevel != 30 || gNight.Level != 30 || gNight.Shadows != 20 || gNight.Azimuth != 250 {
		t.Fatalf("unexpected night info: base=%d level=%d shadows=%d az=%d", gNight.BaseLevel, gNight.Level, gNight.Shadows, gNight.Azimuth)
	}
}

func TestParseNightCommandLegacy(t *testing.T) {
	gNight = NightInfo{}
	if !parseNightCommand("/nt 20") {
		t.Fatalf("parseNightCommand failed")
	}
	gNight.mu.Lock()
	defer gNight.mu.Unlock()
	if gNight.BaseLevel != 20 || gNight.Level != 20 || gNight.Shadows != 30 {
		t.Fatalf("unexpected night info: base=%d level=%d shadows=%d", gNight.BaseLevel, gNight.Level, gNight.Shadows)
	}
}

func TestDrawNightOverlayDarkensPixels(t *testing.T) {
	t.Skip("requires graphical backend")
}
