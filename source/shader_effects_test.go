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
	gs.FasterCharacterShadows = false

	if !mobileFrameBlendingEnabled() || !pictureFrameBlendingEnabled() ||
		!artworkUpscaleEnabled() || !shaderLightingEnabled() ||
		!characterShadowCompositeEnabled() || !replacementEffectsEnabled() ||
		!layeredCharacterShadowsEnabled() || !perFrameShaderEffectsEnabled() {
		t.Fatal("an enabled shader group did not become active")
	}

	gs.ShadersEnabled = false
	if mobileFrameBlendingEnabled() || pictureFrameBlendingEnabled() ||
		shaderLightingEnabled() ||
		characterShadowCompositeEnabled() || layeredCharacterShadowsEnabled() || replacementEffectsEnabled() ||
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

func TestFasterCharacterShadowsSelectsBatchedComposite(t *testing.T) {
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })

	gs = gsdef
	if !layeredCharacterShadowsEnabled() {
		t.Fatal("default shader shadows are not draw-order-correct")
	}
	gs.FasterCharacterShadows = true
	if layeredCharacterShadowsEnabled() {
		t.Fatal("faster character shadows retained the layered path")
	}
	if !characterShadowCompositeEnabled() {
		t.Fatal("faster character shadows disabled the batched composite")
	}
}
