package main

import (
	"encoding/binary"
	"fmt"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"gothoom/climg"
)

const performanceTourFixture = "tour-2025.08.02.clMov"

type tourPerformanceFixture struct {
	images          *climg.CLImages
	frameData       [][]byte
	initialScene    drawState
	busyScene       drawState
	obscuringScene  drawState
	longestBubble   string
	drawPacketCount int
}

var (
	performanceFixtureOnce sync.Once
	performanceFixtureData tourPerformanceFixture
	performanceFixtureErr  error
)

func loadTourPerformanceFixture(t testing.TB) *tourPerformanceFixture {
	t.Helper()
	performanceFixtureOnce.Do(func() {
		performanceFixtureData, performanceFixtureErr = buildTourPerformanceFixture(t)
	})
	if performanceFixtureErr != nil {
		t.Fatalf("load tour performance fixture: %v", performanceFixtureErr)
	}
	return &performanceFixtureData
}

func buildTourPerformanceFixture(t testing.TB) (tourPerformanceFixture, error) {
	t.Helper()
	originalSettings := gs
	originalImages := clImages
	originalMovieMode := movieMode
	originalEncrypted := drawStateEncrypted
	originalState := cloneCurrentDrawState()
	originalInitialState := cloneInitialDrawState()
	originalMovieVersion := movieVersion
	originalMovieRevision := movieRevision
	originalFrameCounter := frameCounter
	originalMovieDropped := movieDropped
	defer func() {
		gs = originalSettings
		clImages = originalImages
		movieMode = originalMovieMode
		drawStateEncrypted = originalEncrypted
		restoreDrawState(originalState)
		restoreInitialDrawState(originalInitialState)
		movieVersion = originalMovieVersion
		movieRevision = originalMovieRevision
		frameCounter = originalFrameCounter
		movieDropped = originalMovieDropped
		initFont()
	}()

	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		return tourPerformanceFixture{}, fmt.Errorf("could not locate performance benchmark source")
	}
	images, err := climg.Load(filepath.Join(filepath.Dir(sourceFile), "data", CL_ImagesFile))
	if err != nil {
		return tourPerformanceFixture{}, fmt.Errorf("load CL_Images: %w", err)
	}

	gs = gsdef
	clImages = images
	movieMode = true
	drawStateEncrypted = false
	initFont()
	frames, err := parseMovie(movieFixturePath(t, performanceTourFixture), clVersion)
	if err != nil {
		return tourPerformanceFixture{}, fmt.Errorf("parse movie: %w", err)
	}

	fixture := tourPerformanceFixture{
		images:       images,
		initialScene: cloneDrawState(initialState),
		frameData:    make([][]byte, 0, len(frames)),
	}
	bestSceneScore := -1
	bestObscuringScore := -1
	for _, frame := range frames {
		fixture.frameData = append(fixture.frameData, frame.data)
		if len(frame.data) < 2 || binary.BigEndian.Uint16(frame.data[:2]) != 2 {
			frameCounter++
			continue
		}
		fixture.drawPacketCount++
		handleDrawState(frame.data, true)

		stateMu.Lock()
		sceneScore := len(state.pictures) + len(state.mobiles)
		if sceneScore > bestSceneScore {
			bestSceneScore = sceneScore
			fixture.busyScene = cloneDrawState(state)
		}
		obscuringScore := len(state.pictures) * len(state.mobiles)
		if obscuringScore > bestObscuringScore {
			bestObscuringScore = obscuringScore
			fixture.obscuringScene = cloneDrawState(state)
		}
		for _, bubble := range state.bubbles {
			if len(bubble.Text) > len(fixture.longestBubble) {
				fixture.longestBubble = bubble.Text
			}
		}
		stateMu.Unlock()
	}
	if fixture.drawPacketCount == 0 {
		return tourPerformanceFixture{}, fmt.Errorf("movie contains no draw-state packets")
	}
	if fixture.longestBubble == "" {
		return tourPerformanceFixture{}, fmt.Errorf("movie contains no speech bubbles")
	}
	return fixture, nil
}

func cloneCurrentDrawState() drawState {
	stateMu.Lock()
	defer stateMu.Unlock()
	return cloneDrawState(state)
}

func cloneInitialDrawState() drawState {
	stateMu.Lock()
	defer stateMu.Unlock()
	return cloneDrawState(initialState)
}

func restoreDrawState(restored drawState) {
	stateMu.Lock()
	state = cloneDrawState(restored)
	stateMu.Unlock()
}

func restoreInitialDrawState(restored drawState) {
	stateMu.Lock()
	initialState = cloneDrawState(restored)
	stateMu.Unlock()
}

func installTourBenchmarkGlobals(b *testing.B, fixture *tourPerformanceFixture) {
	b.Helper()
	originalSettings := gs
	originalImages := clImages
	originalMovieMode := movieMode
	originalEncrypted := drawStateEncrypted
	originalState := cloneCurrentDrawState()
	originalFrameCounter := frameCounter
	originalMovieDropped := movieDropped

	gs = gsdef
	clImages = fixture.images
	movieMode = true
	drawStateEncrypted = false
	initFont()
	b.Cleanup(func() {
		gs = originalSettings
		clImages = originalImages
		movieMode = originalMovieMode
		drawStateEncrypted = originalEncrypted
		restoreDrawState(originalState)
		frameCounter = originalFrameCounter
		movieDropped = originalMovieDropped
		initFont()
	})
}

func replayTourDrawState(fixture *tourPerformanceFixture, buildRenderCaches bool) {
	restoreDrawState(fixture.initialScene)
	frameCounter = 0
	for _, data := range fixture.frameData {
		if len(data) >= 2 && binary.BigEndian.Uint16(data[:2]) == 2 {
			handleDrawState(data, buildRenderCaches)
		} else {
			frameCounter++
		}
	}
}

func BenchmarkTourDrawState(b *testing.B) {
	fixture := loadTourPerformanceFixture(b)
	installTourBenchmarkGlobals(b, fixture)

	b.Run("StateUpdate", func(b *testing.B) {
		replayTourDrawState(fixture, false)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			replayTourDrawState(fixture, false)
		}
		b.ReportMetric(float64(fixture.drawPacketCount), "draw-packets/op")
	})
	b.Run("StateUpdateAndRenderCaches", func(b *testing.B) {
		replayTourDrawState(fixture, true)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			replayTourDrawState(fixture, true)
		}
		b.ReportMetric(float64(fixture.drawPacketCount), "draw-packets/op")
	})
}

func BenchmarkPrepareBusySceneRenderCache(b *testing.B) {
	fixture := loadTourPerformanceFixture(b)
	installTourBenchmarkGlobals(b, fixture)
	restoreDrawState(fixture.busyScene)
	stateMu.Lock()
	prepareRenderCacheLocked()
	stateMu.Unlock()

	b.ReportAllocs()
	for b.Loop() {
		stateMu.Lock()
		prepareRenderCacheLocked()
		stateMu.Unlock()
	}
	b.ReportMetric(float64(len(fixture.busyScene.pictures)), "pictures/op")
	b.ReportMetric(float64(len(fixture.busyScene.mobiles)), "mobiles/op")
}

func BenchmarkCaptureBusySceneSnapshot(b *testing.B) {
	fixture := loadTourPerformanceFixture(b)
	installTourBenchmarkGlobals(b, fixture)
	restoreDrawState(fixture.busyScene)
	var snapshot drawSnapshot
	captureDrawSnapshot(&snapshot)

	b.ReportAllocs()
	for b.Loop() {
		captureDrawSnapshot(&snapshot)
	}
	b.ReportMetric(float64(len(fixture.busyScene.pictures)), "pictures/op")
	b.ReportMetric(float64(len(fixture.busyScene.mobiles)), "mobiles/op")
}

func BenchmarkBusyScenePictureObscuring(b *testing.B) {
	fixture := loadTourPerformanceFixture(b)
	installTourBenchmarkGlobals(b, fixture)
	scene := cloneDrawState(fixture.obscuringScene)
	mobiles := make([]frameMobile, 0, len(scene.mobiles))
	for _, mobile := range scene.mobiles {
		mobiles = append(mobiles, mobile)
	}
	cachePictureObscuring(scene.pictures, mobiles, scene.descriptors, scene.prevMobiles, scene.logicalFrame)

	b.ReportAllocs()
	for b.Loop() {
		cachePictureObscuring(scene.pictures, mobiles, scene.descriptors, scene.prevMobiles, scene.logicalFrame)
	}
	b.ReportMetric(float64(len(scene.pictures)), "pictures/op")
	b.ReportMetric(float64(len(mobiles)), "mobiles/op")
}

func BenchmarkTourBubbleTextLayout(b *testing.B) {
	fixture := loadTourPerformanceFixture(b)
	installTourBenchmarkGlobals(b, fixture)
	const maxWidth = 300

	b.Run("Uncached", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			wrapText(fixture.longestBubble, bubbleFont, maxWidth)
		}
	})
	b.Run("Cached", func(b *testing.B) {
		clearBubbleTextCaches()
		cachedBubbleTextLayout(fixture.longestBubble, bubbleFont, maxWidth)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			cachedBubbleTextLayout(fixture.longestBubble, bubbleFont, maxWidth)
		}
	})
}
