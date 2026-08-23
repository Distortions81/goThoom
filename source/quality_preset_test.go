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

func TestPotatoGPUQualityPresetUsesCurrentSettings(t *testing.T) {
	originalSettings := gs
	t.Cleanup(func() {
		gs = originalSettings
		setHighQualityResamplingEnabled(gs.HighQualityResampling)
	})

	gs = gsdef
	applyQualityPreset("iGPU / Low-VRAM (Potato GPU)")

	if !gs.DenoiseImages || !gs.MotionSmoothing {
		t.Error("iGPU / Low-VRAM preset should retain CPU-side graphics improvements")
	}
	if gs.BlendMobiles || gs.BlendPicts || gs.ShaderLighting {
		t.Error("iGPU / Low-VRAM preset enabled a GPU-intensive graphics effect")
	}
	if !gs.SpriteUpscaleFilter {
		t.Error("iGPU / Low-VRAM preset should retain sharp sprite upscaling")
	}
	if !gs.PotatoGPU {
		t.Error("iGPU / Low-VRAM preset did not enable low-VRAM mode")
	}
	if gs.HighQualityResampling {
		t.Error("iGPU / Low-VRAM preset enabled high-quality resampling")
	}
	if gs.SoundEnhancement || gs.SoundEnhancementAmount != 1.0 || gs.MusicEnhancement {
		t.Error("iGPU / Low-VRAM preset enabled sound or music enhancement")
	}
	if preset := detectQualityPreset(); preset != 0 {
		t.Errorf("detectQualityPreset()=%d, want 0", preset)
	}

	applyQualityPreset("High")
	if gs.PotatoGPU {
		t.Error("High preset retained low-VRAM mode")
	}
	if preset := detectQualityPreset(); preset != 4 {
		t.Errorf("detectQualityPreset()=%d after High, want 4", preset)
	}
}

func TestApplySettingsDisablesExpensiveGPUOptionsInPotatoMode(t *testing.T) {
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
	applySettings()

	if gs.BlendMobiles || gs.BlendPicts || gs.ShaderLighting {
		t.Fatal("potato mode retained an expensive GPU option")
	}
	if !gs.SpriteUpscaleFilter {
		t.Fatal("potato mode disabled the sharp 2x sprite upscaler")
	}
}
