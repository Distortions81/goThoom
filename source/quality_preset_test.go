package main

import "testing"

func TestQualityPresetPersisted(t *testing.T) {
	origDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = origDir })

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
	if gs.HighQualityResampling {
		t.Errorf("HighQualityResampling loaded as true, want false")
	}
	if gs.SoundEnhancement {
		t.Errorf("SoundEnhancement loaded as true, want false")
	}
	if gs.MusicEnhancement {
		t.Errorf("MusicEnhancement loaded as true, want false")
	}
	if preset := detectQualityPreset(); preset != 2 {
		t.Errorf("detectQualityPreset()=%d, want 2", preset)
	}
}

func TestPotatoGPUIsIndependentOfQualityPresets(t *testing.T) {
	originalSettings := gs
	t.Cleanup(func() {
		gs = originalSettings
		setHighQualityResamplingEnabled(gs.HighQualityResampling)
	})

	gs = gsdef
	gs.PotatoGPU = true
	applyQualityPreset("High")
	if !gs.PotatoGPU {
		t.Error("High preset changed the independent Potato GPU setting")
	}
	if preset := detectQualityPreset(); preset != 4 {
		t.Errorf("detectQualityPreset()=%d after High, want 4", preset)
	}
}

func TestIGPUGraphicsPresetUsesLowCostGraphicsAndAudio(t *testing.T) {
	originalSettings := gs
	t.Cleanup(func() {
		gs = originalSettings
		setHighQualityResamplingEnabled(gs.HighQualityResampling)
	})

	gs = gsdef
	gs.HighQualityResampling = true
	gs.SoundEnhancement = true
	gs.SoundEnhancementAmount = 1.75
	gs.MusicEnhancement = true
	gs.GameScale = 4
	gs.DenoiseImages = true
	gs.AnimatedChatBubbles = true
	applyQualityPreset("iGPU Graphics")

	if gs.PotatoGPU {
		t.Fatal("iGPU graphics preset enabled Potato GPU mode")
	}
	if gs.BlendMobiles || gs.BlendPicts || gs.ShaderLighting {
		t.Fatal("iGPU graphics preset retained an expensive graphics effect")
	}
	if !gs.SpriteUpscaleFilter || artworkUpscaleMode() != artworkUpscaleBalanced {
		t.Fatal("iGPU graphics preset did not select Balanced artwork upscaling")
	}
	if gs.CharacterShadows {
		t.Fatal("iGPU graphics preset retained character shadows")
	}
	if !gs.DetailedCharacterShadows {
		t.Fatal("iGPU graphics preset changed the accurate shadows preference")
	}
	if gs.AnimatedChatBubbles {
		t.Fatal("iGPU graphics preset retained animated chat bubbles")
	}
	if gs.GameScale != 2 || gs.DenoiseImages {
		t.Fatal("iGPU graphics preset does not match the current artwork scale and denoise settings")
	}
	if gs.HighQualityResampling || gs.SoundEnhancement || gs.SoundEnhancementAmount != 1 || gs.MusicEnhancement {
		t.Fatal("iGPU graphics preset retained audio enhancement or high-quality resampling")
	}
	if gs.WindowShadows {
		t.Fatal("iGPU graphics preset retained window shadows")
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
	if preset := detectQualityPreset(); preset != 4 {
		t.Fatalf("detectQualityPreset()=%d with dithering off, want High", preset)
	}
	gs.DenoiseImages = true
	if preset := detectQualityPreset(); preset != 4 {
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
	gs.BlendMobiles = true
	gs.BlendPicts = true
	gs.ShaderLighting = true
	gs.SpriteUpscaleFilter = true
	gs.WindowShadows = true
	gs.CharacterShadows = true
	gs.DetailedCharacterShadows = true
	applySettings()

	if !gs.BlendMobiles || !gs.BlendPicts || !gs.ShaderLighting {
		t.Fatal("Potato GPU changed graphics quality options")
	}
	if !gs.SpriteUpscaleFilter || !gs.WindowShadows || !gs.CharacterShadows || !gs.DetailedCharacterShadows {
		t.Fatal("Potato GPU changed upscale or shadow options")
	}
}
