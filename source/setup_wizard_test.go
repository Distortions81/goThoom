package main

import (
	"net"
	"testing"
)

func TestShouldShowSetupWizard(t *testing.T) {
	tests := []struct {
		name             string
		configLoaded     bool
		completedVersion int
		currentVersion   int
		want             bool
	}{
		{name: "first run", configLoaded: false, completedVersion: 36, currentVersion: 36, want: true},
		{name: "current release reviewed", configLoaded: true, completedVersion: 36, currentVersion: 36, want: false},
		{name: "new release", configLoaded: true, completedVersion: 35, currentVersion: 36, want: true},
		{name: "downgrade", configLoaded: true, completedVersion: 37, currentVersion: 36, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldShowSetupWizard(tt.configLoaded, tt.completedVersion, tt.currentVersion)
			if got != tt.want {
				t.Fatalf("shouldShowSetupWizard() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetupWizardMovieHasCompatibleSnapshot(t *testing.T) {
	previousRevision := movieRevision
	t.Cleanup(func() {
		movieRevision = previousRevision
		resetDrawState()
	})

	frames, err := parseMovieZipBytes(setupWizardMovieZip, clVersion)
	if err != nil {
		t.Fatalf("parse setup wizard movie: %v", err)
	}
	if len(frames) == 0 {
		t.Fatal("setup wizard movie contains no frames")
	}
	want := uint16(flagMobileData | flagPictureTable)
	if frames[0].flags&want != want {
		t.Fatalf("first frame flags = %#04x, want mobile and picture snapshots (%#04x)", frames[0].flags, want)
	}
}

func TestSetupWizardPreviewDoesNotStartOrStopOnlinePlayback(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	previousConn := tcpConn
	previousActive := setupWizardPreviewActive
	previousPlaying := playingMovie
	tcpConn = client
	setupWizardPreviewActive = false
	playingMovie = false
	t.Cleanup(func() {
		tcpConn = previousConn
		setupWizardPreviewActive = previousActive
		playingMovie = previousPlaying
	})

	startSetupWizardPreview()
	if setupWizardPreviewActive || playingMovie {
		t.Fatal("setup wizard started movie playback during an online session")
	}

	stopSetupWizardPreview()
	if setupWizardPreviewActive || playingMovie {
		t.Fatal("setup wizard stop path changed online playback state")
	}
}

func TestSetupAndNetworkSettingsPersist(t *testing.T) {
	originalDir := dataDirPath
	originalSettings := gs
	originalLoaded := settingsLoaded
	originalDirty := settingsDirty
	originalHost := host
	dataDirPath = t.TempDir()
	t.Cleanup(func() {
		dataDirPath = originalDir
		gs = originalSettings
		settingsLoaded = originalLoaded
		settingsDirty = originalDirty
		host = originalHost
		setHighQualityResamplingEnabled(gs.HighQualityResampling)
		syncTTSBlocklist()
	})

	gs = gsdef
	gs.SetupWizardVersion = 36
	gs.AltNetMode = false
	gs.AltNetDelay = 42
	gs.VSync = false
	gs.PrecacheImages = true
	gs.PrecacheSounds = true
	saveSettings()

	gs = gsdef
	if !loadSettings() {
		t.Fatal("loadSettings() = false after saving valid settings")
	}
	if gs.SetupWizardVersion != 36 {
		t.Errorf("SetupWizardVersion = %d, want 36", gs.SetupWizardVersion)
	}
	if gs.AltNetMode {
		t.Error("AltNetMode = true, want false")
	}
	if gs.AltNetDelay != 42 {
		t.Errorf("AltNetDelay = %d, want 42", gs.AltNetDelay)
	}
	if gs.VSync {
		t.Error("VSync = true, want false")
	}
	if !gs.PrecacheImages || !gs.PrecacheSounds {
		t.Errorf("precache settings = images:%v sounds:%v, want both true", gs.PrecacheImages, gs.PrecacheSounds)
	}
}
