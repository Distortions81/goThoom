package main

import "testing"

func TestShaderMasterDisablesEveryShaderGroupWithoutChangingPreferences(t *testing.T) {
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })

	gs = gsdef
	gs.MotionSmoothing = true
	gs.BlendMobiles = true
	gs.BlendPicts = true
	gs.SpriteUpscaleFilter = true
	gs.SpriteUpscaleMode = artworkUpscaleBalanced
	gs.ShaderLighting = true
	gs.ReplacementEffects = true

	if !mobileFrameBlendingEnabled() || !pictureFrameBlendingEnabled() ||
		!artworkUpscaleEnabled() || !shaderLightingEnabled() ||
		!characterShadowCompositeEnabled() || !replacementEffectsEnabled() ||
		!perFrameShaderEffectsEnabled() {
		t.Fatal("an enabled shader group did not become active")
	}

	gs.ShadersEnabled = false
	if mobileFrameBlendingEnabled() || pictureFrameBlendingEnabled() ||
		shaderLightingEnabled() ||
		characterShadowCompositeEnabled() || replacementEffectsEnabled() ||
		perFrameShaderEffectsEnabled() {
		t.Fatal("the shader master left a shader group active")
	}
	if !artworkUpscaleEnabled() {
		t.Fatal("the shader master disabled CPU artwork upscaling")
	}
	if !gs.BlendMobiles || !gs.BlendPicts || !gs.SpriteUpscaleFilter ||
		!gs.ShaderLighting || !gs.ReplacementEffects {
		t.Fatal("the shader master changed an individual effect preference")
	}
}
