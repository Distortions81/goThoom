package main

import "testing"

func TestMedianOccupiedRowWidthIgnoresWideArmRows(t *testing.T) {
	const width, height = 12, 7
	pixels := make([]byte, width*height*4)
	rowWidths := []int{4, 4, 10, 4, 4, 4, 4}
	for y, occupied := range rowWidths {
		for x := 0; x < occupied; x++ {
			pixels[(y*width+x)*4+3] = 255
		}
	}
	if got := medianOccupiedRowWidth(pixels, width, height); got != 4 {
		t.Fatalf("medianOccupiedRowWidth() = %d, want torso width 4", got)
	}
}

func TestOpaqueFootYIgnoresTransparentBottomRows(t *testing.T) {
	const width, height = 4, 6
	pixels := make([]byte, width*height*4)
	pixels[(3*width+2)*4+3] = 255
	if got := opaqueFootY(pixels, width, height); got != 4 {
		t.Fatalf("opaqueFootY() = %d, want bottom of occupied row 4", got)
	}
}

func TestBuildLightShadowsUsesNearbyCasters(t *testing.T) {
	lights := []lightSource{{X: 100, Y: 100, Radius: 80, R: 1, G: 0.5, B: 0.25, Intensity: 1}}
	casters := []lightCaster{
		{X: 140, Y: 100, Radius: 10},
		{X: 600, Y: 100, Radius: 10},
		{X: 102, Y: 100, Radius: 10},
	}

	shadows := buildLightShadows(lights, casters, nil)
	if len(shadows) != 1 {
		t.Fatalf("buildLightShadows returned %d shadows, want 1", len(shadows))
	}
	shadow := shadows[0]
	if shadow.CasterX != 140 || shadow.CasterY != 100 {
		t.Fatalf("shadow caster = (%v, %v), want (140, 100)", shadow.CasterX, shadow.CasterY)
	}
	if shadow.LightRadius != 100 {
		t.Fatalf("effective light radius = %v, want 100", shadow.LightRadius)
	}
}

func TestMobileLightConeShadowSettingGatesCasters(t *testing.T) {
	originalSettings := gs
	originalCasters := frameLightCasters
	t.Cleanup(func() {
		gs = originalSettings
		frameLightCasters = originalCasters
	})

	gs.ShadersEnabled = true
	gs.ShaderLighting = true
	gs.GameScale = 2
	frameLightCasters = nil
	metrics := mobileSpriteMetrics{widthFraction: 0.5, footFraction: 0.9}

	gs.MobileLightConeShadows = false
	addMobileLightCaster(100, 100, 40, metrics)
	if len(frameLightCasters) != 0 {
		t.Fatal("disabled mobile light-cone shadows registered a caster")
	}

	gs.MobileLightConeShadows = true
	addMobileLightCaster(100, 100, 40, metrics)
	if len(frameLightCasters) != 1 {
		t.Fatalf("enabled mobile light-cone shadows registered %d casters, want 1", len(frameLightCasters))
	}
}

func TestBuildLightShadowsIncludesGlowTail(t *testing.T) {
	lights := []lightSource{{X: 0, Y: 0, Radius: 80, Intensity: 1}}
	// The shader radius is 100 after scaling. This caster is outside that
	// nominal radius but still within the visible inverse-square glow tail.
	casters := []lightCaster{{X: 300, Y: 0, Radius: 6}}
	if shadows := buildLightShadows(lights, casters, nil); len(shadows) != 1 {
		t.Fatalf("glow-tail caster produced %d cone shadows, want one", len(shadows))
	}
}

func TestBuildLightShadowsStopsAtLightCutoff(t *testing.T) {
	lights := []lightSource{{X: 0, Y: 0, Radius: 80, Intensity: 1}}
	// Effective radius is 100 and the shader reaches zero at 4x that radius.
	casters := []lightCaster{{X: 400, Y: 0, Radius: 6}}
	if shadows := buildLightShadows(lights, casters, nil); len(shadows) != 0 {
		t.Fatalf("cutoff caster produced %d cone shadows, want none", len(shadows))
	}
}

func TestBuildLightShadowsIgnoresCharacterCenteredLights(t *testing.T) {
	lights := []lightSource{{X: 100, Y: 100, Radius: 80, Intensity: 1}}
	casters := []lightCaster{{X: 100, Y: 116, Radius: 6, LightExclusionRadius: 29}}

	if shadows := buildLightShadows(lights, casters, nil); len(shadows) != 0 {
		t.Fatalf("character-centered light produced %d cone shadows, want none", len(shadows))
	}

	casters[0].X = 140
	if shadows := buildLightShadows(lights, casters, nil); len(shadows) != 1 {
		t.Fatalf("external light produced %d cone shadows, want one", len(shadows))
	}
}

func TestBuildLightShadowsHonorsBudget(t *testing.T) {
	lights := []lightSource{{X: 0, Y: 0, Radius: 1000, Intensity: 1}}
	casters := make([]lightCaster, maxLightShadows+8)
	for i := range casters {
		casters[i] = lightCaster{X: float32(20 + i), Radius: 2}
	}

	shadows := buildLightShadows(lights, casters, nil)
	if len(shadows) != maxLightShadows {
		t.Fatalf("buildLightShadows returned %d shadows, want budget %d", len(shadows), maxLightShadows)
	}
}
