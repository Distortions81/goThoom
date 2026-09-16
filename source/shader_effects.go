package main

// Each effect keeps its own preference while the master switch is off. This
// makes disabling all custom shaders cheap and reversible without losing the
// user's per-effect choices.
func shaderLightingEnabled() bool {
	return gs.ShaderLighting
}

func replacementEffectsEnabled() bool {
	return gs.ReplacementEffects
}

func replacementEffectEnabled(kind replacementEffectKind) bool {
	for _, disabled := range gs.DisabledReplacementEffects {
		if disabled == kind {
			return false
		}
	}
	return true
}

func setReplacementEffectEnabled(kind replacementEffectKind, enabled bool) {
	for index, disabled := range gs.DisabledReplacementEffects {
		if disabled != kind {
			continue
		}
		if enabled {
			gs.DisabledReplacementEffects = append(gs.DisabledReplacementEffects[:index], gs.DisabledReplacementEffects[index+1:]...)
		}
		return
	}
	if !enabled {
		gs.DisabledReplacementEffects = append(gs.DisabledReplacementEffects, kind)
	}
}

func characterShadowCompositeEnabled() bool {
	return true
}

// layeredCharacterShadowsEnabled keeps each projected shadow at its caster's
// painter-order position. The faster mode batches every directional shadow
// into one below-mobile mask and therefore cannot shade later foreground
// layers.
func layeredCharacterShadowsEnabled() bool {
	return characterShadowCompositeEnabled() && !gs.FasterCharacterShadows
}

func perFrameShaderEffectsEnabled() bool {
	return shaderLightingEnabled() || mobileFrameBlendingEnabled() || pictureFrameBlendingEnabled() ||
		(gs.CharacterShadows && characterShadowCompositeEnabled()) || replacementEffectsEnabled()
}
