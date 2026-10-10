package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"gothoom/eui"
)

func preserveMixerTestState(t *testing.T) {
	t.Helper()
	initFont()
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
	originalDownloadWin := downloadWin
	t.Cleanup(func() {
		if downloadWin != nil && downloadWin != originalDownloadWin {
			downloadWin.RemoveWindow()
		}
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
		downloadWin = originalDownloadWin
	})
	mixerWin, downloadWin = nil, nil
}

func TestMixerEnhancementCheckboxesLoadSavedState(t *testing.T) {
	preserveMixerTestState(t)
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

type mixerTestRoundTripper func(*http.Request) (*http.Response, error)

func (fn mixerTestRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestMixerEnablesIncludedSoundFontWithoutDownloads(t *testing.T) {
	preserveMixerTestState(t)
	preserveStoragePathTestState(t)
	originalClient, originalStatus := http.DefaultClient, status
	t.Cleanup(func() {
		http.DefaultClient, status = originalClient, originalStatus
	})
	http.DefaultClient = &http.Client{Transport: mixerTestRoundTripper(func(request *http.Request) (*http.Response, error) {
		t.Errorf("enabling included music made an HTTP request: %s", request.URL)
		return nil, fmt.Errorf("unexpected mixer HTTP request")
	})}
	gs = gsdef
	gs.Music = false
	gs.AssetsPath = filepath.Join(t.TempDir(), "missing-assets")
	storagePathsActivated = false
	status.NeedSoundfont = true
	makeMixerWindow()

	musicMixCB.Handler.Emit(eui.UIEvent{Item: musicMixCB, Type: eui.EventCheckboxChanged, Checked: true})
	if !gs.Music || musicMixSlider.Disabled {
		t.Fatal("included SoundFont did not enable music")
	}
	if downloadWin != nil {
		t.Fatal("included SoundFont opened Downloads")
	}
}
