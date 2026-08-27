package main

import (
	"image"
	"math"
	"testing"
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
	defer func() { startupLoader.stage = originalStage }()

	tests := []struct {
		stage startupLoadStage
		want  string
	}{
		{startupLoadImages, "Loading CL_Images..."},
		{startupLoadSounds, "Loading CL_Sounds..."},
		{startupLoadInterface, "Loading interface..."},
		{startupLoadScripts, "Loading scripts..."},
		{startupLoadCoreDone, "Loading shaders..."},
	}
	for _, test := range tests {
		startupLoader.stage = test.stage
		if got := startupLoadingLabel(); got != test.want {
			t.Errorf("startupLoadingLabel() = %q, want %q", got, test.want)
		}
	}
}

func TestStartupLoadingTextLayoutIsCenteredAndScalesWithWidth(t *testing.T) {
	check := func(bounds image.Rectangle) float64 {
		t.Helper()
		const textWidth, textHeight = 240.0, 30.0
		scale, x, baselineY := startupLoadingTextLayout(bounds, textWidth, textHeight)
		scaledWidth := textWidth * scale
		scaledHeight := textHeight * scale
		centerX := x + scaledWidth/2
		centerY := baselineY - scaledHeight/2
		wantX := float64(bounds.Min.X+bounds.Max.X) / 2
		wantY := float64(bounds.Min.Y+bounds.Max.Y) / 2
		if math.Abs(centerX-wantX) > 0.001 || math.Abs(centerY-wantY) > 0.001 {
			t.Fatalf("text center = (%.2f, %.2f), want (%.2f, %.2f)", centerX, centerY, wantX, wantY)
		}
		if scaledWidth > float64(bounds.Dx())*0.8+0.001 {
			t.Fatalf("scaled text width %.2f exceeds 80%% of window width %d", scaledWidth, bounds.Dx())
		}
		return scale
	}

	smallScale := check(image.Rect(0, 0, 800, 500))
	largeScale := check(image.Rect(37, 29, 1637, 929))
	if largeScale <= smallScale {
		t.Fatalf("large-window scale %.2f is not greater than small-window scale %.2f", largeScale, smallScale)
	}
}
