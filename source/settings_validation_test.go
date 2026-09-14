package main

import "testing"

func TestNormalizeLoadedNumericSettingsClampsToSupportedRanges(t *testing.T) {
	value := gsdef
	value.KBWalkSpeed = -1
	value.MainFontSize = 100
	value.BubbleFontSize = -1
	value.BubbleOpacity = 2
	value.BubbleBaseLife = -1
	value.BubbleLifePerWord = 4
	value.NameBgOpacity = -1
	value.BarOpacity = 0
	value.ObscuringPictureOpacity = 1
	value.NameHealthBarThickness = 20
	value.MaxNightLevel = 200
	value.GameScale = 3.6
	value.BlendAmount = 2
	value.MobileBlendAmount = 0
	value.DenoiseAmount = 1
	value.ShaderLightStrength = 3
	value.MasterVolume = 2
	value.GameVolume = -1
	value.ChatTTSSpeed = 4
	value.NotificationDuration = 0
	value.UIScale = 10
	value.JoystickWalkDeadzone = 0
	value.PowerSaveFPS = 500
	value.SpriteCacheMiB = 10000
	value.SpriteGamma = 2.3

	if !normalizeLoadedNumericSettings(&value) {
		t.Fatal("out-of-range settings were not reported as changed")
	}
	if value.KBWalkSpeed != 0.1 || value.MainFontSize != 48 || value.BubbleFontSize != 4 {
		t.Fatalf("input/font normalization = speed:%v main:%v bubble:%v", value.KBWalkSpeed, value.MainFontSize, value.BubbleFontSize)
	}
	if value.BubbleOpacity != 1 || value.BubbleBaseLife != 1 || value.BubbleLifePerWord != 2 {
		t.Fatalf("bubble normalization = opacity:%v base:%v word:%v", value.BubbleOpacity, value.BubbleBaseLife, value.BubbleLifePerWord)
	}
	if value.NameBgOpacity != 0 || value.BarOpacity != 0.1 || value.ObscuringPictureOpacity != 0.7 {
		t.Fatalf("opacity normalization = name:%v bar:%v artwork:%v", value.NameBgOpacity, value.BarOpacity, value.ObscuringPictureOpacity)
	}
	if value.NameHealthBarThickness != 8 || value.MaxNightLevel != 100 || value.GameScale != 3.6 {
		t.Fatalf("display normalization = bar:%d night:%d scale:%v", value.NameHealthBarThickness, value.MaxNightLevel, value.GameScale)
	}
	if value.BlendAmount != 1 || value.MobileBlendAmount != 0.1 || value.DenoiseAmount != 0.5 || value.ShaderLightStrength != 2 {
		t.Fatalf("rendering normalization = world:%v mobile:%v denoise:%v light:%v", value.BlendAmount, value.MobileBlendAmount, value.DenoiseAmount, value.ShaderLightStrength)
	}
	if value.MasterVolume != 1 || value.GameVolume != 0 || value.ChatTTSSpeed != 2 || value.NotificationDuration != 1 {
		t.Fatalf("audio normalization = master:%v game:%v TTS:%v notification:%v", value.MasterVolume, value.GameVolume, value.ChatTTSSpeed, value.NotificationDuration)
	}
	if value.UIScale != 4 || value.JoystickWalkDeadzone != 0.01 || value.PowerSaveFPS != 250 || value.SpriteCacheMiB != 8192 {
		t.Fatalf("system normalization = UI:%v deadzone:%v FPS:%d cache:%d", value.UIScale, value.JoystickWalkDeadzone, value.PowerSaveFPS, value.SpriteCacheMiB)
	}
	if value.SpriteGamma != 2.2 {
		t.Fatalf("sprite gamma = %v, want nearest supported 2.2", value.SpriteGamma)
	}
}

func TestNormalizeLoadedNumericSettingsPreservesValidPreferences(t *testing.T) {
	value := gsdef
	value.GameScale = 3
	value.PowerSaveFPS = 240
	value.SpriteCacheMiB = 768
	value.BubbleBaseLife = 5
	value.MotionSmoothing = false

	if normalizeLoadedNumericSettings(&value) {
		t.Fatal("valid settings were unexpectedly normalized")
	}
	if value.GameScale != 3 || value.PowerSaveFPS != 240 || value.SpriteCacheMiB != 768 || value.BubbleBaseLife != 5 || value.MotionSmoothing {
		t.Fatalf("valid preferences changed: %+v", value)
	}
}

func TestNormalizeLoadedNumericSettingsUsesDefaultCacheForNonPositiveValue(t *testing.T) {
	value := gsdef
	value.SpriteCacheMiB = 0
	if !normalizeLoadedNumericSettings(&value) || value.SpriteCacheMiB != defaultSpriteCacheMiB {
		t.Fatalf("non-positive sprite cache normalized to %d MiB, want %d", value.SpriteCacheMiB, defaultSpriteCacheMiB)
	}
}
