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
		{startupLoadSounds, "Loading sounds"},
		{startupLoadInterface, "Building interface"},
		{startupLoadScripts, "Loading scripts"},
		{startupLoadCoreDone, "Preparing lighting"},
	}
	for _, test := range tests {
		startupLoader.stage = test.stage
		if got := startupLoadingLabel(); got != test.want {
			t.Errorf("startupLoadingLabel() = %q, want %q", got, test.want)
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
	if lines[0].text != "Scripts loaded" || lines[2].text != "Artwork renderer ready" {
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
	startupLoader.stage = startupLoadSounds
	if got := startupLoadingProgress(); got != 1.0/6.0 {
		t.Fatalf("sound-stage progress = %v, want %v", got, 1.0/6.0)
	}
	startupLoader.stage = startupLoadCoreDone
	startupShaderLoader.lightingAttempted = true
	startupShaderLoader.upscaleAttempted = true
	if got := startupLoadingProgress(); got != 1 {
		t.Fatalf("completed progress = %v, want 1", got)
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
