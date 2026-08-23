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
		"flame light flicker":          gsdef.FlameLightFlicker,
		"character shadows":            gsdef.CharacterShadows,
		"detailed character shadows":   gsdef.DetailedCharacterShadows,
		"artwork upscale filter":       gsdef.SpriteUpscaleFilter,
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

func TestArtworkUpscaleDefaults(t *testing.T) {
	if gsdef.GameScale != 3 || gsdef.SpriteUpscale != 3 || gsdef.SpriteUpscaleMode != artworkUpscaleUltraSmooth {
		t.Fatalf("default artwork upscale = (%v, %d, %d), want 3x Ultra Smooth", gsdef.GameScale, gsdef.SpriteUpscale, gsdef.SpriteUpscaleMode)
	}
}

func TestDetailedCharacterShadowsDefaultOn(t *testing.T) {
	if !gsdef.DetailedCharacterShadows {
		t.Error("detailed character shadows should be enabled by default")
	}
}

func TestFlameFlickerStrengthDefault(t *testing.T) {
	if gsdef.FlameFlickerStrength != 1 {
		t.Errorf("flame flicker strength = %v, want 1", gsdef.FlameFlickerStrength)
	}
}

func TestStereoMusicDefaultOff(t *testing.T) {
	if gsdef.MusicStereoPan {
		t.Error("stereo music should be opt-in")
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
		"flame light flicker":          gs.FlameLightFlicker,
		"character shadows":            gs.CharacterShadows,
		"detailed character shadows":   gs.DetailedCharacterShadows,
		"artwork upscale filter":       gs.SpriteUpscaleFilter,
		"sound enhancement":            gs.SoundEnhancement,
		"high quality resampling":      gs.HighQualityResampling,
		"music enhancement":            gs.MusicEnhancement,
	} {
		if !enabled {
			t.Errorf("new config has %s disabled", name)
		}
	}
}

func TestExistingConfigDefaultsNewRenderingOptionsOn(t *testing.T) {
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

	gs.CharacterShadows = false
	gs.DetailedCharacterShadows = false
	gs.GameScale = 1
	gs.SpriteUpscale = 1
	gs.SpriteUpscaleFilter = false
	gs.SpriteUpscaleMode = artworkUpscaleOff
	gs.FlameLightFlicker = false
	if !loadSettings() {
		t.Fatal("loadSettings() = false for current-version settings")
	}
	if !gs.CharacterShadows {
		t.Error("settings without CharacterShadows should default it on")
	}
	if !gs.DetailedCharacterShadows {
		t.Error("settings without DetailedCharacterShadows should default it on")
	}
	if gs.GameScale != 3 || gs.SpriteUpscale != 3 || !gs.SpriteUpscaleFilter || gs.SpriteUpscaleMode != artworkUpscaleUltraSmooth {
		t.Errorf("settings without artwork upscale values defaulted to (%v, %d, %v, %d), want (3, 3, true, Ultra Smooth)", gs.GameScale, gs.SpriteUpscale, gs.SpriteUpscaleFilter, gs.SpriteUpscaleMode)
	}
	if !gs.FlameLightFlicker {
		t.Error("settings without FlameLightFlicker should default it on")
	}
	if gs.FlameFlickerStrength != 1 {
		t.Errorf("settings without FlameFlickerStrength defaulted to %v, want 1", gs.FlameFlickerStrength)
	}
}
