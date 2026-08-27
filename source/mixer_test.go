package main

import "testing"

func TestMixerEnhancementCheckboxesLoadSavedState(t *testing.T) {
	originalGS := gs
	originalMixerWin := mixerWin
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
	t.Cleanup(func() {
		gs = originalGS
		mixerWin = originalMixerWin
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

	gs = gsdef
	gs.SoundEnhancement = true
	gs.MusicEnhancement = true
	mixerWin = nil
	makeMixerWindow()

	if soundEnhanceMixCB == nil || !soundEnhanceMixCB.Checked {
		t.Fatal("sound enhancement checkbox did not load checked state")
	}
	if musicEnhanceMixCB == nil || !musicEnhanceMixCB.Checked {
		t.Fatal("music enhancement checkbox did not load checked state")
	}
}
