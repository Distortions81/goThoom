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
		*primarySession.night = NightInfo{}
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
	*primarySession.night = NightInfo{}
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
		primarySession.night.mu.Lock()
		level := primarySession.night.BaseLevel
		azimuth := primarySession.night.Azimuth
		cloudy := primarySession.night.Cloudy
		primarySession.night.mu.Unlock()
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
	originalState := cloneDrawState(primarySession.draw.current)
	t.Cleanup(func() {
		*primarySession.night = NightInfo{}
		primarySession.draw.mu.Lock()
		primarySession.draw.current = originalState
		primarySession.draw.mu.Unlock()
	})

	emptyState := drawState{
		descriptors: make(map[uint8]frameDescriptor),
		mobiles:     make(map[uint8]frameMobile),
		prevMobiles: make(map[uint8]frameMobile),
		prevDescs:   make(map[uint8]frameDescriptor),
	}
	*primarySession.night = NightInfo{Azimuth: 0, Shadows: 50}
	firstNight := captureMovieNightState()
	*primarySession.night = NightInfo{Azimuth: 90, Shadows: 50}
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
	second := newCharacterShadowProjection(primarySession.night.Azimuth)
	player.seek(0)
	first := newCharacterShadowProjection(primarySession.night.Azimuth)
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

	want := movieNightState{azimuth: 90, shadows: 50, oldAzimuth: 90, redshift: 1, shadow: shadowCasterState{azimuth: 90, level: 50, length: 1, initialized: true}}
	if got := captureMovieNightState(); got != want {
		t.Fatalf("night state after reset = %+v, want classic daylight %+v", got, want)
	}
}

func TestNewSessionStartsWithClassicDaylight(t *testing.T) {
	session := mustNewSession(2)
	want := movieNightState{azimuth: 90, shadows: 50, oldAzimuth: 90, redshift: 1, shadow: shadowCasterState{azimuth: 90, level: 50, length: 1, initialized: true}}
	if got := captureMovieNightStateForSession(session); got != want {
		t.Fatalf("new session lighting = %+v, want %+v", got, want)
	}
}

func TestMovieInitialLightingSurvivesSessionAndRewind(t *testing.T) {
	initFont()
	originalNight := captureMovieNightState()
	originalVersion, originalRevision := movieVersion, movieRevision
	originalMode, originalPlaying := movieMode, playingMovie
	originalRate, originalPaused := currentMovieMusicTempoRate(), movieMusicPaused.Load()
	t.Cleanup(func() {
		restoreMovieNightState(originalNight)
		movieVersion, movieRevision = originalVersion, originalRevision
		movieMode, playingMovie = originalMode, originalPlaying
		setMovieMusicTempoRate(originalRate)
		movieMusicPaused.Store(originalPaused)
		resetDrawState()
	})

	for _, tc := range []struct {
		name           string
		command        string
		fixture        string
		previousShadow shadowCasterState
		want           movieNightState
	}{
		{
			name: "no initial sun update",
			want: movieNightState{azimuth: 90, shadows: 50, oldAzimuth: 90, redshift: 1, shadow: shadowCasterState{azimuth: 90, level: 50, length: 1, initialized: true}},
		},
		{
			name:    "lore1 before its first sun update",
			fixture: "lore1.clMov",
			want:    movieNightState{azimuth: 90, shadows: 50, oldAzimuth: 90, redshift: 1, shadow: shadowCasterState{azimuth: 90, level: 50, length: 1, initialized: true}},
		},
		{
			name:           "lore1 retains the previous shadow caster",
			fixture:        "lore1.clMov",
			previousShadow: shadowCasterState{azimuth: 30, level: 40, length: uprightShadowLength(30), initialized: true},
			want:           movieNightState{azimuth: 90, shadows: 50, oldAzimuth: 90, redshift: 1, shadow: shadowCasterState{azimuth: 30, level: 40, length: uprightShadowLength(30), initialized: true}},
		},
		{
			name:    "recorded initial sun update",
			command: "/nt 20 /sa 35 /cl 1",
			want:    movieNightState{baseLevel: 20, level: 20, azimuth: 35, cloudy: true, shadows: 25, oldAzimuth: 35, redshift: 1, shadow: shadowCasterState{azimuth: 35, level: 25, length: uprightShadowLength(35), initialized: true}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Logical time resets; a fresh renderer starts at the classic default.
			restoreMovieNightState(movieNightState{baseLevel: 80, azimuth: 175, cloudy: true, flags: kLightNoShadows, shadow: tc.previousShadow})
			data := make([]byte, 24)
			binary.BigEndian.PutUint32(data, movieSignature)
			binary.BigEndian.PutUint16(data[4:], uint16(clVersion))
			binary.BigEndian.PutUint16(data[6:], 24)
			if tc.command != "" {
				frame := make([]byte, 12)
				binary.BigEndian.PutUint32(frame, movieSignature)
				binary.BigEndian.PutUint16(frame[10:], flagGameState)
				payload := append([]byte(tc.command), 0)
				data = append(data, frame...)
				data = append(data, gameStateBlock(0, 0, 0, len(payload), len(payload), len(payload), payload)...)
			}
			if tc.fixture != "" {
				data = readMovieFixture(t, tc.fixture)
			}
			frames, err := parseMovieData(data, clVersion)
			if err != nil {
				t.Fatal(err)
			}
			session := mustNewSession(2)
			p := newMoviePlayer(session, frames, movieRecordedUPS, nil)
			t.Cleanup(p.ticker.Stop)
			if got := captureMovieNightStateForSession(session); got != tc.want {
				t.Fatalf("movie startup lighting = %+v, want %+v", got, tc.want)
			}
			parseNightCommandForSession(session, "/nt 0 /sa 150 /cl 0")
			p.seek(0)
			if got := captureMovieNightStateForSession(session); got != tc.want {
				t.Fatalf("rewound movie lighting = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestClassicShadowCasterUpdates(t *testing.T) {
	session := mustNewSession(2)
	night := session.night
	night.setFlags(0, 1)
	if got := characterShadowProjectionForNight(night.snapshot()); got.length != 1 {
		t.Fatalf("fresh classic shadow length = %v, want 1", got.length)
	}
	parseNightCommandForSession(session, "/nt 10 /sa 30 /cl 0")
	previous := night.snapshot().shadow
	previousProjection := characterShadowProjectionForNight(night.snapshot())
	night.reset()
	night.setFlags(0, 2)
	if got := night.snapshot(); got.azimuth != 90 || got.shadow != previous {
		t.Fatalf("reset with unchanged flags did not preserve caster: %+v", got)
	}
	if got := characterShadowProjectionForNight(night.snapshot()); got != previousProjection {
		t.Fatalf("reset changed rendered projection: %+v, want %+v", got, previousProjection)
	}
	night.setFlags(kLightAdjust25Pct, 3)
	if got := night.snapshot().shadow; got.azimuth != 90 || got.length != uprightShadowLength(90) || got.level != 50 {
		t.Fatalf("changed flags did not refresh caster: %+v", got)
	}
	// Even an update repeating the logical default must replace a stale caster.
	parseNightCommandForSession(session, "/nt 0 /sa 150 /cl 0")
	night.reset()
	parseNightCommandForSession(session, "/nt 0 /sa 90 /cl 0")
	if got := night.snapshot().shadow; got.azimuth != 90 || got.length != uprightShadowLength(90) {
		t.Fatalf("timekeeper did not refresh caster: %+v", got)
	}
	other := mustNewSession(3)
	if got := other.night.snapshot().shadow; got.azimuth != 90 || got.length != 1 {
		t.Fatalf("new live session inherited another session's caster: %+v", got)
	}
}
