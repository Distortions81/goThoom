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
	applyQualityPreset("Potato GPU / iGPU")

	if !gs.DenoiseImages || !gs.MotionSmoothing {
		t.Error("Potato GPU / iGPU preset should retain CPU-side graphics improvements")
	}
	if gs.BlendMobiles || gs.BlendPicts || gs.ShaderLighting || gs.SpriteUpscaleFilter {
		t.Error("Potato GPU / iGPU preset enabled a GPU-intensive graphics effect")
	}
	if gs.HighQualityResampling {
		t.Error("Potato GPU / iGPU preset enabled high-quality resampling")
	}
	if gs.SoundEnhancement || gs.SoundEnhancementAmount != 1.0 || gs.MusicEnhancement {
		t.Error("Potato GPU / iGPU preset enabled sound or music enhancement")
	}
	if preset := detectQualityPreset(); preset != 0 {
		t.Errorf("detectQualityPreset()=%d, want 0", preset)
	}
}
