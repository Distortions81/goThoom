package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

func TestStockMoviesUpdateNightState(t *testing.T) {
	initFont()
	originalImages := clImages
	originalMovieMode := movieMode
	originalMovieVersion := movieVersion
	originalMovieRevision := movieRevision
	clImages = testCLImages(nil)
	t.Cleanup(func() {
		gNight = NightInfo{}
		movieMode = originalMovieMode
		movieVersion = originalMovieVersion
		movieRevision = originalMovieRevision
		clImages = originalImages
		resetDrawState()
	})

	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate movie test source")
	}
	for _, fixture := range []struct {
		name        string
		wantUpdates int
	}{
		{name: "2004.clMov.zip", wantUpdates: 18},
		{name: "lore1.clMov.zip", wantUpdates: 166},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			assertStockMovieNightUpdates(t, filepath.Join(filepath.Dir(sourceFile), "clmovFiles", fixture.name), fixture.wantUpdates)
		})
	}
}

func assertStockMovieNightUpdates(t *testing.T, path string, wantUpdates int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	frames, err := parseMovieZipBytes(data, clVersion)
	if err != nil {
		t.Fatal(err)
	}
	gNight = NightInfo{}
	movieMode = true
	updates := 0
	for _, frame := range frames {
		if len(frame.data) < 2 || binary.BigEndian.Uint16(frame.data[:2]) != 2 {
			continue
		}
		commandOffset := bytes.Index(frame.data, []byte("/nt "))
		if commandOffset < 0 {
			continue
		}
		match := nightRE.FindStringSubmatch(string(frame.data[commandOffset:]))
		if match == nil {
			t.Fatalf("frame %d has malformed night command", frame.index)
		}
		wantLevel, _ := strconv.Atoi(match[1])
		wantAzimuth, _ := strconv.Atoi(match[2])
		wantCloudy := match[3] != "0"

		handleDrawState(frame.data, false)
		gNight.mu.Lock()
		level := gNight.BaseLevel
		azimuth := gNight.Azimuth
		cloudy := gNight.Cloudy
		gNight.mu.Unlock()
		if level != wantLevel || azimuth != wantAzimuth || cloudy != wantCloudy {
			t.Fatalf("frame %d night state = (%d, %d, %v), want (%d, %d, %v)", frame.index, level, azimuth, cloudy, wantLevel, wantAzimuth, wantCloudy)
		}
		updates++
	}
	if updates != wantUpdates {
		t.Fatalf("parsed %d night updates, want %d", updates, wantUpdates)
	}
}

func TestMovieSeekRestoresNightProjection(t *testing.T) {
	originalState := cloneDrawState(state)
	t.Cleanup(func() {
		gNight = NightInfo{}
		stateMu.Lock()
		state = originalState
		stateMu.Unlock()
	})

	emptyState := drawState{
		descriptors: make(map[uint8]frameDescriptor),
		mobiles:     make(map[uint8]frameMobile),
		prevMobiles: make(map[uint8]frameMobile),
		prevDescs:   make(map[uint8]frameDescriptor),
	}
	gNight = NightInfo{Azimuth: 0, Shadows: 50}
	firstNight := captureMovieNightState()
	gNight = NightInfo{Azimuth: 90, Shadows: 50}
	secondNight := captureMovieNightState()

	player := &moviePlayer{
		frames:  make([]movieFrame, 100),
		fps:     10,
		playing: false,
		checkpoints: []movieCheckpoint{
			{idx: 0, state: cloneDrawState(emptyState), night: firstNight},
			{idx: 100, state: cloneDrawState(emptyState), night: secondNight},
		},
	}
	player.seek(100)
	second := newCharacterShadowProjection(gNight.Azimuth)
	player.seek(0)
	first := newCharacterShadowProjection(gNight.Azimuth)
	if first.angle == second.angle || first.length == second.length {
		t.Fatalf("movie seek retained projection: first=%+v second=%+v", first, second)
	}
}

func TestResetNightStateDiscardsSessionTime(t *testing.T) {
	originalNight := captureMovieNightState()
	t.Cleanup(func() { restoreMovieNightState(originalNight) })

	restoreMovieNightState(movieNightState{
		baseLevel: 87, azimuth: -1, cloudy: true, flags: 3,
		level: 62, shadows: 25, oldAzimuth: -2, redshift: 1.2,
		startOfTwilight: 1234,
	})
	resetNightState()

	if got := captureMovieNightState(); got != (movieNightState{}) {
		t.Fatalf("night state after reset = %+v, want zero state", got)
	}
}
