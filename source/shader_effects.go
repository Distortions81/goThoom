package main

// Each effect keeps its own preference while the master switch is off. This
// makes disabling all custom shaders cheap and reversible without losing the
// user's per-effect choices.
func shaderLightingEnabled() bool {
	return gs.ShadersEnabled && gs.ShaderLighting
}

func replacementEffectsEnabled() bool {
	return gs.ShadersEnabled && gs.ReplacementEffects
}

func characterShadowCompositeEnabled() bool {
	return gs.ShadersEnabled
}

// layeredCharacterShadowsEnabled keeps each projected shadow at its caster's
// painter-order position. The faster mode batches every directional shadow
// into one final mask and therefore cannot preserve foreground occlusion.
func layeredCharacterShadowsEnabled() bool {
	return characterShadowCompositeEnabled() && !gs.FasterCharacterShadows
}

func perFrameShaderEffectsEnabled() bool {
	return shaderLightingEnabled() || mobileFrameBlendingEnabled() || pictureFrameBlendingEnabled() ||
		(gs.CharacterShadows && characterShadowCompositeEnabled()) || replacementEffectsEnabled()
}
