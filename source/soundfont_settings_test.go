package main

import (
	"os"
	"path/filepath"
	"testing"

	"gothoom/eui"

	"github.com/sinshu/go-meltysynth/meltysynth"
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

func TestMusicSoundFontFallsBackForMissingBankZeroProgram(t *testing.T) {
	selected := &meltysynth.SoundFont{Presets: []*meltysynth.Preset{
		{BankNumber: 0, PatchNumber: 47},
		{BankNumber: 128, PatchNumber: 73},
	}}
	fallback := &meltysynth.SoundFont{Presets: []*meltysynth.Preset{{BankNumber: 0, PatchNumber: 73}}}

	if got := musicSoundFontForProgram(selected, fallback, 47); got != selected {
		t.Fatal("available custom program did not use the selected SoundFont")
	}
	if got := musicSoundFontForProgram(selected, fallback, 73); got != fallback {
		t.Fatal("program present only outside Bank 0 did not use the fallback SoundFont")
	}
	if got := musicSoundFontForProgram(selected, fallback, 106); got != nil {
		t.Fatal("missing custom program with no default preset unexpectedly found a SoundFont")
	}
}

func TestDefaultSoundFontPathIgnoresCustomSelection(t *testing.T) {
	preserveStoragePathTestState(t)
	dir := t.TempDir()
	gs = gsdef
	gs.AssetsPath = dir
	gs.SoundFontFile = "custom.sf2"
	activeStoragePaths.assets = dir
	storagePathsActivated = true
	if got, want := defaultSoundFontPath(), filepath.Join(dir, soundFontFile); got != want {
		t.Fatalf("default soundfont path = %q, want %q", got, want)
	}
}

func TestMissingFallbackProgramReportsConsoleErrorOnce(t *testing.T) {
	originalSettings := gs
	originalEntries := consoleLog.entries
	const generation = 987654321
	key := programGainKey{generation: generation, program: 106}
	t.Cleanup(func() {
		gs = originalSettings
		consoleLog.entries = originalEntries
		missingProgramMu.Lock()
		delete(missingProgramReported, key)
		missingProgramMu.Unlock()
	})
	gs.SoundFontFile = "custom.sf2"
	consoleLog.entries = nil

	want := `Music SoundFont "custom.sf2" is missing Bank 0 preset 106, and "soundfont.sf2" cannot provide a fallback.`
	if got := reportMissingSoundFontProgram(generation, 106); got != want {
		t.Fatalf("fallback error = %q, want %q", got, want)
	}
	reportMissingSoundFontProgram(generation, 106)
	if len(consoleLog.entries) != 1 || consoleLog.entries[0].Text != want {
		t.Fatalf("console errors = %#v, want one fallback error", consoleLog.entries)
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
