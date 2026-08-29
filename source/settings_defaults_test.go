package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestEnhancedRenderingDefaultsEnabled(t *testing.T) {
	defaults := map[string]bool{
		"fade obscuring pictures":    gsdef.FadeObscuringPictures,
		"smooth movement":            gsdef.MotionSmoothing,
		"floating-point coordinates": gsdef.FloatingPointSpriteCoords,
		"world animation blending":   gsdef.BlendPicts,
		"shader master":              gsdef.ShadersEnabled,
		"shader lighting":            gsdef.ShaderLighting,
		"flame light flicker":        gsdef.FlameLightFlicker,
		"character shadows":          gsdef.CharacterShadows,
		"characters receive shadows": gsdef.MobilesReceiveSunShadows,
		"artwork upscale filter":     gsdef.SpriteUpscaleFilter,
		"sprite gamma correction":    gsdef.SpriteGammaCorrection,
		"throttle sounds":            gsdef.ThrottleSounds,
		"alternate row backgrounds":  gsdef.AlternateRowBackgrounds,
		"window shadows":             gsdef.WindowShadows,
		"animated chat bubbles":      gsdef.AnimatedChatBubbles,
	}
	for name, enabled := range defaults {
		if !enabled {
			t.Errorf("%s should be enabled by default", name)
		}
	}
	if gsdef.DenoiseImages {
		t.Error("blend image dithering should be disabled by default")
	}
	if gsdef.BlendMobiles {
		t.Error("character animation blending should be disabled by default")
	}
	if gsdef.DenoiseSharpness != 10 || gsdef.DenoiseAmount != 0.35 {
		t.Errorf("de-dither defaults = sharpness %v, strength %v; want 10 and 0.35", gsdef.DenoiseSharpness, gsdef.DenoiseAmount)
	}
}

func TestArtworkUpscaleDefaults(t *testing.T) {
	if gsdef.GameScale != 4 || gsdef.SpriteUpscale != 4 || gsdef.SpriteUpscaleMode != artworkUpscaleBalanced {
		t.Fatalf("default artwork upscale = (%v, %d, %d), want 4x Balanced", gsdef.GameScale, gsdef.SpriteUpscale, gsdef.SpriteUpscaleMode)
	}
}

func TestFontSizeDefaults(t *testing.T) {
	want := map[string]struct {
		got  float64
		want float64
	}{
		"name":         {gsdef.MainFontSize, 8},
		"chat bubble":  {gsdef.BubbleFontSize, 20},
		"console":      {gsdef.ConsoleFontSize, 12},
		"chat window":  {gsdef.ChatFontSize, 12},
		"inventory":    {gsdef.InventoryFontSize, 14},
		"players list": {gsdef.PlayersFontSize, 14},
	}
	for name, size := range want {
		if size.got != size.want {
			t.Errorf("default %s font size = %v, want %v", name, size.got, size.want)
		}
	}
}

func TestNameHealthBarDefaultsAbove(t *testing.T) {
	if !gsdef.NameHealthBarModern || !gsdef.NameHealthBarAbove || gsdef.NameHealthBarThickness != 3 {
		t.Fatalf("name health bar defaults = modern %v, above %v, thickness %d", gsdef.NameHealthBarModern, gsdef.NameHealthBarAbove, gsdef.NameHealthBarThickness)
	}
}

func TestOwnNameTagVisibleByDefault(t *testing.T) {
	if gsdef.HideSelfNameTag {
		t.Fatal("own name tag should be visible by default")
	}
}

func TestSmallMovingPictureInterpolationDefaultsOff(t *testing.T) {
	if gsdef.InterpolateSmallMovingPictures {
		t.Fatal("small moving-picture interpolation should default off")
	}
}

func TestInputBarDefaultsOpen(t *testing.T) {
	if !gsdef.InputBarAlwaysOpen {
		t.Fatal("input bar should be open by default")
	}
}

func TestCharacterShadowDarknessDefault(t *testing.T) {
	if gsdef.CharacterShadowDarkness != 1.8 {
		t.Errorf("default character shadow darkness = %v, want 1.8", gsdef.CharacterShadowDarkness)
	}
}

func TestFlameFlickerStrengthDefault(t *testing.T) {
	if gsdef.FlameFlickerStrength != 1 {
		t.Errorf("flame flicker strength = %v, want 1", gsdef.FlameFlickerStrength)
	}
}

func TestMusicEnhancementAmountDefaultAndClamp(t *testing.T) {
	if gsdef.MusicEnhancementAmount != 1 {
		t.Fatalf("music enhancement default = %v, want 1", gsdef.MusicEnhancementAmount)
	}
	for _, test := range []struct {
		value float64
		want  float64
	}{
		{value: -1, want: 0.1},
		{value: 0.7, want: 0.7},
		{value: 3, want: 2},
	} {
		if got := clampMusicEnhancementAmount(test.value); got != test.want {
			t.Errorf("clampMusicEnhancementAmount(%v) = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestUIScalePreferenceClamp(t *testing.T) {
	for _, test := range []struct {
		value float64
		want  float64
	}{
		{value: 0, want: 0.75},
		{value: 1.25, want: 1.25},
		{value: 5, want: 4},
	} {
		if got := clampUIScalePreference(test.value); got != test.want {
			t.Errorf("clampUIScalePreference(%v) = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestRecentPlayersGroupDefaultOn(t *testing.T) {
	if !gsdef.ShowRecentPlayers {
		t.Error("recently on-screen player group should default on")
	}
}

func TestPlayerShareIconsDefaultOff(t *testing.T) {
	if gsdef.PlayerShareIcons {
		t.Error("player-list sharing icons should default off")
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
		"smooth movement":            gs.MotionSmoothing,
		"floating-point coordinates": gs.FloatingPointSpriteCoords,
		"world animation blending":   gs.BlendPicts,
		"shader effects":             gs.ShadersEnabled,
		"shader lighting":            gs.ShaderLighting,
		"flame light flicker":        gs.FlameLightFlicker,
		"character shadows":          gs.CharacterShadows,
		"characters receive shadows": gs.MobilesReceiveSunShadows,
		"artwork upscale filter":     gs.SpriteUpscaleFilter,
		"sound enhancement":          gs.SoundEnhancement,
		"high quality resampling":    gs.HighQualityResampling,
		"music enhancement":          gs.MusicEnhancement,
	} {
		if !enabled {
			t.Errorf("new config has %s disabled", name)
		}
	}
	if gs.DenoiseImages {
		t.Error("new config has blend image dithering enabled")
	}
	if gs.BlendMobiles {
		t.Error("new config has character animation blending enabled")
	}
	if gs.DenoiseSharpness != 10 || gs.DenoiseAmount != 0.35 {
		t.Errorf("new config de-dither defaults = sharpness %v, strength %v; want 10 and 0.35", gs.DenoiseSharpness, gs.DenoiseAmount)
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

	data := []byte(fmt.Sprintf(`{"version":%d}`, SETTINGS_VERSION))
	if err := os.WriteFile(filepath.Join(dataDirPath, settingsFile), data, 0o644); err != nil {
		t.Fatal(err)
	}

	gs.CharacterShadows = false
	gs.MobilesReceiveSunShadows = false
	gs.ShadersEnabled = false
	gs.FloatingPointSpriteCoords = false
	gs.BlendMobiles = true
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
	if !gs.MobilesReceiveSunShadows {
		t.Error("settings without MobilesReceiveSunShadows should default it on")
	}
	if !gs.ShadersEnabled {
		t.Error("settings without ShadersEnabled should default it on")
	}
	if !gs.FloatingPointSpriteCoords {
		t.Error("settings without FloatingPointSpriteCoords should default it on")
	}
	if gs.BlendMobiles {
		t.Error("settings without BlendMobiles should default it off")
	}
	if gs.GameScale != 4 || gs.SpriteUpscale != 4 || !gs.SpriteUpscaleFilter || gs.SpriteUpscaleMode != artworkUpscaleBalanced {
		t.Errorf("settings without artwork upscale values defaulted to (%v, %d, %v, %d), want (4, 4, true, Balanced)", gs.GameScale, gs.SpriteUpscale, gs.SpriteUpscaleFilter, gs.SpriteUpscaleMode)
	}
	if !gs.FlameLightFlicker {
		t.Error("settings without FlameLightFlicker should default it on")
	}
	if gs.FlameFlickerStrength != 1 {
		t.Errorf("settings without FlameFlickerStrength defaulted to %v, want 1", gs.FlameFlickerStrength)
	}
}
