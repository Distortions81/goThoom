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
