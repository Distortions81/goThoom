package main

import (
	"image"
	"testing"
	"time"
)

func TestStartupShadersLoadRegardlessOfPreset(t *testing.T) {
	originalSettings := gs
	originalLoader := startupShaderLoader
	originalLighting := lightingShader
	originalUpscale := spriteUpscaleShader
	originalFrameBlend := frameBlendShader
	originalReplacementReady := replacementEffectsShadersReady
	originalReplacementAttempted := replacementEffectsShaderInitAttempted
	defer func() {
		gs = originalSettings
		startupShaderLoader = originalLoader
		lightingShader = originalLighting
		spriteUpscaleShader = originalUpscale
		frameBlendShader = originalFrameBlend
		replacementEffectsShadersReady = originalReplacementReady
		replacementEffectsShaderInitAttempted = originalReplacementAttempted
	}()

	lightingShader = nil
	spriteUpscaleShader = nil
	frameBlendShader = nil
	replacementEffectsShadersReady = false
	replacementEffectsShaderInitAttempted = false
	startupShaderLoader.lightingAttempted = false
	startupShaderLoader.upscaleAttempted = false
	gs.ShaderLighting = false
	gs.ShadersEnabled = true
	gs.SpriteUpscaleFilter = false
	gs.SpriteUpscaleMode = artworkUpscaleOff
	gs.ReplacementEffects = false

	if !startupShaderPending() {
		t.Fatal("disabled preset should still schedule core shader compilation during startup")
	}

	startupShaderLoader.lightingAttempted = true
	startupShaderLoader.upscaleAttempted = true
	if startupShaderPending() {
		t.Fatal("disabled replacement effects should not extend startup loading")
	}

	gs.ReplacementEffects = true
	if !startupShaderPending() {
		t.Fatal("enabled replacement effects should be scheduled")
	}
}

func TestStartupLoadingLabels(t *testing.T) {
	originalStage := startupLoader.stage
	originalShaderLoader := startupShaderLoader
	defer func() {
		startupLoader.stage = originalStage
		startupShaderLoader = originalShaderLoader
	}()
	startupShaderLoader.lightingAttempted = false
	startupShaderLoader.upscaleAttempted = false

	tests := []struct {
		stage startupLoadStage
		want  string
	}{
		{startupLoadImages, "Loading artwork"},
		{startupLoadInterface, "Building windows and controls"},
		{startupLoadCommonAssets, "Precaching common artwork and sounds"},
		{startupLoadCoreDone, "Preparing lighting"},
	}
	for _, test := range tests {
		startupLoader.stage = test.stage
		if got := startupLoadingLabel(); got != test.want {
			t.Errorf("startupLoadingLabel() = %q, want %q", got, test.want)
		}
	}
}

func TestStartupArtworkPreloadExcludesPaletteVariants(t *testing.T) {
	if len(startupArtworkPreloadIDs) == 0 {
		t.Fatal("startup artwork preload is empty")
	}
	seen := make(map[uint16]struct{}, len(startupArtworkPreloadIDs))
	for _, id := range startupArtworkPreloadIDs {
		if id == 0 || id == 0xffff {
			t.Fatalf("invalid startup artwork ID %d", id)
		}
		if _, exists := seen[id]; exists {
			t.Fatalf("duplicate startup artwork ID %d", id)
		}
		seen[id] = struct{}{}
		key := makeSheetKey(id, nil, false)
		if key.colorsLen != 0 || key.forceTransparent {
			t.Fatalf("startup artwork ID %d produced a palette/mobile key: %#v", id, key)
		}
	}
	for _, id := range []uint16{635, 1580, 417, 624, 1408, 2252, 1528, 127, 4069} {
		if _, ok := seen[id]; !ok {
			t.Errorf("profession icon %d is not in the startup artwork preload", id)
		}
	}
}

func TestStartupNamedMobilePreloadUsesUncoloredMobileSheets(t *testing.T) {
	if got := len(startupNamedMobileBasePreloadIDs); got != 52 {
		t.Fatalf("named mobile preload count = %d, want 52", got)
	}
	seen := make(map[uint16]struct{}, len(startupNamedMobileBasePreloadIDs))
	for _, id := range startupNamedMobileBasePreloadIDs {
		if id == 0 || id == 0xffff {
			t.Fatalf("invalid named mobile preload ID %d", id)
		}
		if _, exists := seen[id]; exists {
			t.Fatalf("duplicate named mobile preload ID %d", id)
		}
		seen[id] = struct{}{}
		key := makeSheetKey(id, nil, true)
		if key.colorsLen != 0 || !key.forceTransparent {
			t.Fatalf("named mobile preload ID %d produced the wrong key: %#v", id, key)
		}
	}
}

func TestStartupLoadingPanelLayoutStaysCenteredAndInset(t *testing.T) {
	for _, bounds := range []image.Rectangle{
		image.Rect(0, 0, 512, 384),
		image.Rect(0, 0, 1280, 720),
		image.Rect(37, 29, 1637, 929),
	} {
		panel := startupLoadingPanelLayout(bounds)
		if !panel.In(bounds) || panel.Empty() {
			t.Fatalf("panel %v does not fit bounds %v", panel, bounds)
		}
		if panel.Min.X+panel.Max.X != bounds.Min.X+bounds.Max.X || panel.Min.Y+panel.Max.Y != bounds.Min.Y+bounds.Max.Y {
			t.Fatalf("panel %v is not centered in %v", panel, bounds)
		}
		if panel == bounds {
			t.Fatalf("panel %v has no inset within %v", panel, bounds)
		}
	}
}

func TestStartupLoadingBackdropIgnoresSelectedSplash(t *testing.T) {
	originalSplash := splashImg
	defer func() { splashImg = originalSplash }()

	splashImg = nil
	if embeddedSplashImg == nil {
		t.Fatal("embedded splash was not initialized")
	}
	if got := startupLoadingBackdropImage(); got != embeddedSplashImg {
		t.Fatal("startup loading backdrop did not retain the embedded splash")
	}
}

func TestStartupLoadingActivityScrollsToNewestLines(t *testing.T) {
	originalStage := startupLoader.stage
	originalShaderLoader := startupShaderLoader
	originalSettings := gs
	defer func() {
		startupLoader.stage = originalStage
		startupShaderLoader = originalShaderLoader
		gs = originalSettings
	}()

	startupLoader.stage = startupLoadCoreDone
	startupShaderLoader.lightingAttempted = true
	startupShaderLoader.upscaleAttempted = true
	gs.ReplacementEffects = false
	lines := visibleStartupLoadingLogLines(3)
	if len(lines) != 3 {
		t.Fatalf("visible activity lines = %d, want 3", len(lines))
	}
	if lines[0].text != "Common artwork and sounds ready" || lines[2].text != "Artwork renderer ready" {
		t.Fatalf("visible activity lines = %#v, want newest three entries", lines)
	}
}

func TestStartupLoadingProgressTracksCompletedSteps(t *testing.T) {
	originalStage := startupLoader.stage
	originalShaderLoader := startupShaderLoader
	originalSettings := gs
	defer func() {
		startupLoader.stage = originalStage
		startupShaderLoader = originalShaderLoader
		gs = originalSettings
	}()
	gs.ReplacementEffects = false
	startupShaderLoader = struct {
		lastCompileFrame  uint64
		lightingAttempted bool
		upscaleAttempted  bool
	}{}

	startupLoader.stage = startupLoadImages
	if got := startupLoadingProgress(); got != 0 {
		t.Fatalf("initial progress = %v, want 0", got)
	}
	startupLoader.stage = startupLoadInterface
	if got := startupLoadingProgress(); got != 1.0/5.0 {
		t.Fatalf("interface-stage progress = %v, want %v", got, 1.0/5.0)
	}
	startupLoader.stage = startupLoadCommonAssets
	if got := startupLoadingProgress(); got != 2.0/5.0 {
		t.Fatalf("common-precache progress = %v, want %v", got, 2.0/5.0)
	}
	startupLoader.stage = startupLoadCoreDone
	startupShaderLoader.lightingAttempted = true
	startupShaderLoader.upscaleAttempted = true
	if got := startupLoadingProgress(); got != 1 {
		t.Fatalf("completed progress = %v, want 1", got)
	}
}

func TestScriptsWaitForFirstUsableFrameInsteadOfStartup(t *testing.T) {
	originalLoader := startupLoader
	defer func() { startupLoader = originalLoader }()

	startupLoader.complete = true
	startupLoader.scriptsLoaded = false
	startupLoader.readyFrame = 12
	startupLoader.drawnFrames = 12
	if postStartupScriptLoadDue() {
		t.Fatal("scripts were due before the first usable interface frame")
	}
	startupLoader.drawnFrames++
	if !postStartupScriptLoadDue() {
		t.Fatal("scripts were not due after the first usable interface frame")
	}
	startupLoader.scriptsLoaded = true
	if postStartupScriptLoadDue() {
		t.Fatal("scripts remained due after loading")
	}
}

func TestSoundsWaitForFirstUsableFrameInsteadOfStartup(t *testing.T) {
	originalLoader := startupLoader
	defer func() { startupLoader = originalLoader }()

	startupLoader.complete = true
	startupLoader.soundsStarted = false
	startupLoader.readyFrame = 12
	startupLoader.drawnFrames = 12
	if postStartupSoundLoadDue() {
		t.Fatal("sounds were due before the first usable interface frame")
	}
	startupLoader.drawnFrames++
	if !postStartupSoundLoadDue() {
		t.Fatal("sounds were not due after the first usable interface frame")
	}
	startupLoader.soundsStarted = true
	if postStartupSoundLoadDue() {
		t.Fatal("sounds remained due after loading started")
	}
}

func TestStartupLoadingDelayKeepsEachStepVisible(t *testing.T) {
	originalDelay := startupLoadingDelay
	originalNextWork := startupLoader.nextWork
	defer func() {
		startupLoadingDelay = originalDelay
		startupLoader.nextWork = originalNextWork
	}()

	startupLoadingDelay = time.Second
	startupLoader.nextWork = time.Time{}
	now := time.Unix(100, 0)
	if !startupLoadingDelayPending(now) {
		t.Fatal("new startup step was not delayed")
	}
	if !startupLoadingDelayPending(now.Add(500 * time.Millisecond)) {
		t.Fatal("startup step delay ended too early")
	}
	if startupLoadingDelayPending(now.Add(time.Second)) {
		t.Fatal("startup step delay did not expire")
	}
	if !startupLoader.nextWork.IsZero() {
		t.Fatal("expired startup step delay was not reset")
	}
}
