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
	if preset := detectQualityPreset(); preset != 1 {
		t.Errorf("detectQualityPreset()=%d, want 1", preset)
	}
}
