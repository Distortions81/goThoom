package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestEnhancedRenderingDefaultsEnabled(t *testing.T) {
	defaults := map[string]bool{
		"fade obscuring pictures":      gsdef.FadeObscuringPictures,
		"blend image dithering":        gsdef.DenoiseImages,
		"smooth movement":              gsdef.MotionSmoothing,
		"character animation blending": gsdef.BlendMobiles,
		"world animation blending":     gsdef.BlendPicts,
		"shader lighting":              gsdef.ShaderLighting,
		"lighting plane order":         gsdef.LightingPlaneOrder,
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

func TestNewConfigUsesEnhancedRenderingDefaults(t *testing.T) {
	originalSettings := gs
	originalDataDir := dataDirPath
	originalLoaded := settingsLoaded
	dataDirPath = t.TempDir()
	t.Cleanup(func() {
		gs = originalSettings
		dataDirPath = originalDataDir
		settingsLoaded = originalLoaded
		setHighQualityResamplingEnabled(gs.HighQualityResampling)
	})

	if loadSettings() {
		t.Fatal("loadSettings() = true without a settings file")
	}
	for name, enabled := range map[string]bool{
		"blend image dithering":        gs.DenoiseImages,
		"smooth movement":              gs.MotionSmoothing,
		"character animation blending": gs.BlendMobiles,
		"world animation blending":     gs.BlendPicts,
		"shader effects":               gs.ShaderLighting,
		"lighting plane order":         gs.LightingPlaneOrder,
		"sound enhancement":            gs.SoundEnhancement,
		"high quality resampling":      gs.HighQualityResampling,
		"music enhancement":            gs.MusicEnhancement,
	} {
		if !enabled {
			t.Errorf("new config has %s disabled", name)
		}
	}
}

func TestExistingConfigDefaultsLightingPlaneOrderOn(t *testing.T) {
	originalSettings := gs
	originalDataDir := dataDirPath
	originalLoaded := settingsLoaded
	originalHost := host
	dataDirPath = t.TempDir()
	t.Cleanup(func() {
		gs = originalSettings
		dataDirPath = originalDataDir
		settingsLoaded = originalLoaded
		host = originalHost
		setHighQualityResamplingEnabled(gs.HighQualityResampling)
	})

	data := []byte(fmt.Sprintf(`{"Version":%d}`, SETTINGS_VERSION))
	if err := os.WriteFile(filepath.Join(dataDirPath, settingsFile), data, 0o644); err != nil {
		t.Fatal(err)
	}

	gs.LightingPlaneOrder = false
	if !loadSettings() {
		t.Fatal("loadSettings() = false for current-version settings")
	}
	if !gs.LightingPlaneOrder {
		t.Error("settings without LightingPlaneOrder should default it on")
	}
}
