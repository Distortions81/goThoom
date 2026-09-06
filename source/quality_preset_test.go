package main

import "testing"

func TestQualityPresetPersisted(t *testing.T) {
	originalSettings := gs
	origDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() {
		gs = originalSettings
		dataDirPath = origDir
		setHighQualityResamplingEnabled(gs.HighQualityResampling)
	})

	gs = gsdef
	setHighQualityResamplingEnabled(gs.HighQualityResampling)
	applyQualityPreset("Low")
	saveSettings()

	gs = gsdef
	setHighQualityResamplingEnabled(gs.HighQualityResampling)
	loadSettings()

	if gs.ShaderLighting {
		t.Errorf("ShaderLighting loaded as true, want false")
	}
	if gs.ShadersEnabled {
		t.Errorf("ShadersEnabled loaded as true, want false")
	}
	if gs.HighQualityResampling {
		t.Errorf("HighQualityResampling loaded as true, want false")
	}
	if gs.SoundEnhancement {
		t.Errorf("SoundEnhancement loaded as true, want false")
	}
	if gs.MusicEnhancement {
		t.Errorf("MusicEnhancement loaded as true, want false")
	}
	if preset := detectQualityPreset(); preset != 1 {
		t.Errorf("detectQualityPreset()=%d, want 1", preset)
	}
}

func TestQualityPresetsApplyCumulativeTiers(t *testing.T) {
	originalSettings := gs
	t.Cleanup(func() {
		gs = originalSettings
		setHighQualityResamplingEnabled(gs.HighQualityResampling)
	})

	tests := []struct {
		name   string
		preset qualityPreset
		want   qualityPreset
		index  int
	}{
		{
			name:   "Lowest",
			preset: lowestPreset,
			want:   qualityPreset{artworkUpscaleMode: artworkUpscaleOff},
			index:  0,
		},
		{
			name:   "Low",
			preset: lowPreset,
			want: qualityPreset{
				artworkUpscaleMode: artworkUpscaleBalanced,
				characterShadows:   true,
			},
			index: 1,
		},
		{
			name:   "Medium",
			preset: mediumPreset,
			want: qualityPreset{
				artworkUpscaleMode: artworkUpscaleBalanced,
				precacheSounds:     true, windowShadows: true, characterShadows: true,
				shadersEnabled: true, shaderLighting: true,
			},
			index: 2,
		},
		{
			name:   "High",
			preset: highPreset,
			want: qualityPreset{
				artworkUpscaleMode:    artworkUpscaleBalanced,
				fadeObscuringPictures: true, precacheSounds: true, windowShadows: true,
				characterShadows: true, shadersEnabled: true, shaderLighting: true,
				blendPicts: true, mobilesReceiveSunShadows: true, musicEnhancement: true,
			},
			index: 3,
		},
		{
			name:   "Ultra",
			preset: ultraPreset,
			want: qualityPreset{
				artworkUpscaleMode:    artworkUpscaleBalanced,
				fadeObscuringPictures: true, precacheSounds: true, windowShadows: true,
				characterShadows: true, shadersEnabled: true, shaderLighting: true,
				blendPicts: true, mobilesReceiveSunShadows: true, musicEnhancement: true,
				soundEnhancement: true, highQualityResampling: true,
			},
			index: 4,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.preset != test.want {
				t.Fatalf("%s preset definition = %+v, want %+v", test.name, test.preset, test.want)
			}
			gs = gsdef
			gs.GameScale = 4
			gs.SpriteUpscale = 4
			applyQualityPreset(test.name)
			if gs.GameScale != 4 || gs.SpriteUpscale != 4 {
				t.Fatal("quality preset changed the manual artwork scale override")
			}
			if !matchesPreset(test.preset) {
				t.Fatalf("%s preset settings do not match its definition", test.name)
			}
			if got := detectQualityPreset(); got != test.index {
				t.Fatalf("detectQualityPreset()=%d after %s, want %d", got, test.name, test.index)
			}
		})
	}
}

func TestQualityPresetsPreserveUnrelatedSettings(t *testing.T) {
	originalSettings := gs
	t.Cleanup(func() {
		gs = originalSettings
		setHighQualityResamplingEnabled(gs.HighQualityResampling)
	})

	gs = gsdef
	gs.PotatoGPU = true
	gs.DenoiseImages = true
	gs.MotionSmoothing = false
	gs.BlendMobiles = true
	gs.FasterCharacterShadows = true
	gs.AnimatedChatBubbles = false
	gs.SoundEnhancementAmount = 1.75
	gs.MusicEnhancementAmount = 1.6
	applyQualityPreset("High")
	if !gs.PotatoGPU || !gs.DenoiseImages || gs.MotionSmoothing || !gs.BlendMobiles ||
		!gs.FasterCharacterShadows || gs.AnimatedChatBubbles ||
		gs.SoundEnhancementAmount != 1.75 || gs.MusicEnhancementAmount != 1.6 {
		t.Fatal("quality preset changed a setting outside the preset contract")
	}
}

func TestQualityPresetDetectionIgnoresDitherSetting(t *testing.T) {
	originalSettings := gs
	t.Cleanup(func() {
		gs = originalSettings
		setHighQualityResamplingEnabled(gs.HighQualityResampling)
	})

	gs = gsdef
	applyQualityPreset("High")
	gs.DenoiseImages = false
	if preset := detectQualityPreset(); preset != 3 {
		t.Fatalf("detectQualityPreset()=%d with dithering off, want High", preset)
	}
	gs.DenoiseImages = true
	if preset := detectQualityPreset(); preset != 3 {
		t.Fatalf("detectQualityPreset()=%d with dithering on, want High", preset)
	}
}

func TestApplySettingsDoesNotChangeGraphicsOptionsInPotatoMode(t *testing.T) {
	originalSettings := gs
	t.Cleanup(func() {
		gs = originalSettings
		applySettings()
	})

	gs.PotatoGPU = true
	gs.ShadersEnabled = true
	gs.BlendMobiles = true
	gs.BlendPicts = true
	gs.ShaderLighting = true
	gs.SpriteUpscaleFilter = true
	gs.WindowShadows = true
	gs.CharacterShadows = true
	applySettings()

	if !gs.ShadersEnabled || !gs.BlendMobiles || !gs.BlendPicts || !gs.ShaderLighting {
		t.Fatal("Potato GPU changed graphics quality options")
	}
	if !gs.SpriteUpscaleFilter || !gs.WindowShadows || !gs.CharacterShadows {
		t.Fatal("Potato GPU changed upscale or shadow options")
	}
}
