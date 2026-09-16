package main

import "testing"

func TestShaderFeaturesFollowTheirIndividualPreferences(t *testing.T) {
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

	gs.ShaderLighting, gs.ReplacementEffects = false, false
	if shaderLightingEnabled() || replacementEffectsEnabled() {
		t.Fatal("individual graphics preferences did not disable their feature")
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

func TestReplacementEffectsCanBeDisabledByFamily(t *testing.T) {
	originalSettings, originalReady := gs, replacementEffectsShadersReady
	gs = gsdef
	gs.ReplacementEffects = true
	replacementEffectsShadersReady = true
	t.Cleanup(func() {
		gs, replacementEffectsShadersReady = originalSettings, originalReady
	})

	if !replacementEffectReplacesPict(481) {
		t.Fatal("enabled fire plume should replace its source picture")
	}
	setReplacementEffectEnabled(replacementEffectFirePlume, false)
	if replacementEffectReplacesPict(481) {
		t.Fatal("disabled fire plume should use its original picture")
	}
	if !replacementEffectReplacesPict(1759) {
		t.Fatal("disabling fire plumes should not disable healing")
	}
}
