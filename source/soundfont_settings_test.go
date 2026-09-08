package main

import (
	"os"
	"path/filepath"
	"testing"

	"gothoom/eui"
)

func TestConfiguredSoundFontFileFallsBackToDefault(t *testing.T) {
	original := gs
	t.Cleanup(func() { gs = original })

	gs = gsdef
	for _, name := range []string{"", "../outside.sf2", "not-a-soundfont.txt"} {
		gs.SoundFontFile = name
		if got := configuredSoundFontFile(); got != soundFontFile {
			t.Fatalf("configured soundfont for %q = %q, want %q", name, got, soundFontFile)
		}
	}
	gs.SoundFontFile = "orchestra.SF2"
	if got := configuredSoundFontFile(); got != "orchestra.SF2" {
		t.Fatalf("configured soundfont = %q, want selected filename", got)
	}
}

func TestListSoundFontsSkipsInvalidFiles(t *testing.T) {
	preserveStoragePathTestState(t)
	dir := t.TempDir()
	gs = gsdef
	gs.AssetsPath = dir
	activeStoragePaths.assets = dir
	storagePathsActivated = true
	if err := os.WriteFile(filepath.Join(dir, "not-a-font.sf2"), []byte("not an SF2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not an SF2"), 0o644); err != nil {
		t.Fatal(err)
	}
	fonts, err := listSoundFonts()
	if err != nil {
		t.Fatal(err)
	}
	if len(fonts) != 0 {
		t.Fatalf("invalid files appeared in soundfont choices: %v", fonts)
	}
}

func TestMixerAudioSettingsButtonOpensAudioTab(t *testing.T) {
	initFont()
	originalMixer, originalSettings := mixerWin, settingsWin
	originalGameMixSlider := gameMixSlider
	originalMusicMixSlider := musicMixSlider
	originalTTSMixSlider := ttsMixSlider
	originalNotifMixSlider := notifMixSlider
	originalSoundEnhanceMixCB := soundEnhanceMixCB
	originalSoundEnhanceSlider := soundEnhanceSlider
	originalMusicEnhanceMixCB := musicEnhanceMixCB
	originalMusicEnhanceSlider := musicEnhanceSlider
	originalMixMuteBtn := mixMuteBtn
	originalMusicMixCB := musicMixCB
	originalTTSMixCB := ttsMixCB
	mixerWin, settingsWin = nil, nil
	t.Cleanup(func() {
		if mixerWin != nil {
			mixerWin.RemoveWindow()
		}
		if settingsWin != nil {
			settingsWin.RemoveWindow()
		}
		mixerWin, settingsWin = originalMixer, originalSettings
		gameMixSlider = originalGameMixSlider
		musicMixSlider = originalMusicMixSlider
		ttsMixSlider = originalTTSMixSlider
		notifMixSlider = originalNotifMixSlider
		soundEnhanceMixCB = originalSoundEnhanceMixCB
		soundEnhanceSlider = originalSoundEnhanceSlider
		musicEnhanceMixCB = originalMusicEnhanceMixCB
		musicEnhanceSlider = originalMusicEnhanceSlider
		mixMuteBtn = originalMixMuteBtn
		musicMixCB = originalMusicMixCB
		ttsMixCB = originalTTSMixCB
	})
	makeMixerWindow()

	var find func([]*eui.ItemData) *eui.ItemData
	find = func(items []*eui.ItemData) *eui.ItemData {
		for _, item := range items {
			if item.Text == "Audio Settings" {
				return item
			}
			if found := find(item.Contents); found != nil {
				return found
			}
		}
		return nil
	}
	button := find(mixerWin.Contents)
	if button == nil || button.Handler == nil {
		t.Fatal("mixer is missing its Audio Settings button")
	}
	button.Handler.Emit(eui.UIEvent{Item: button, Type: eui.EventClick})
	if settingsWin == nil || !settingsWin.IsOpen() || selectedSettingsTab() != "Audio" {
		t.Fatal("Audio Settings button did not open Settings on the Audio tab")
	}
}
