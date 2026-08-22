package main

import "testing"

func TestEnhancedRenderingDefaultsEnabled(t *testing.T) {
	defaults := map[string]bool{
		"fade obscuring pictures":      gsdef.FadeObscuringPictures,
		"blend image dithering":        gsdef.DenoiseImages,
		"smooth movement":              gsdef.MotionSmoothing,
		"character animation blending": gsdef.BlendMobiles,
		"world animation blending":     gsdef.BlendPicts,
		"shader lighting":              gsdef.ShaderLighting,
		"sprite gamma correction":      gsdef.SpriteGammaCorrection,
		"throttle sounds":              gsdef.ThrottleSounds,
		"alternate row backgrounds":    gsdef.AlternateRowBackgrounds,
	}
	for name, enabled := range defaults {
		if !enabled {
			t.Errorf("%s should be enabled by default", name)
		}
	}
}
