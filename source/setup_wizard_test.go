package main

import (
	"net"
	"slices"
	"testing"
	"time"

	"gothoom/eui"
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

func TestSetupWizardSceneDefaultsFollowEffectPages(t *testing.T) {
	previousPage := setupWizardScenePage
	previousMode := setupWizardSceneModeValue
	t.Cleanup(func() {
		setupWizardScenePage = previousPage
		setupWizardSceneModeValue = previousMode
	})
	for _, test := range []struct {
		page int
		want setupWizardSceneMode
	}{
		{page: 3, want: setupWizardSceneIndoor},
		{page: 4, want: setupWizardSceneDay},
		{page: 5, want: setupWizardSceneMotion},
		{page: 6, want: setupWizardSceneNight},
	} {
		setupWizardScenePage = -1
		selectSetupWizardSceneForPage(test.page)
		if setupWizardSceneModeValue != test.want {
			t.Fatalf("page %d scene = %d, want %d", test.page, setupWizardSceneModeValue, test.want)
		}
	}
}

func TestSetupWizardSyntheticSceneHasDemonstrationSubjects(t *testing.T) {
	previousStart := setupWizardSceneStarted
	previousMode := setupWizardSceneModeValue
	previousPage := setupWizardPage
	originalNight := captureMovieNightState()
	t.Cleanup(func() {
		setupWizardSceneStarted = previousStart
		setupWizardSceneModeValue = previousMode
		setupWizardPage = previousPage
		restoreMovieNightState(originalNight)
	})
	setupWizardSceneModeValue = setupWizardSceneMotion
	setupWizardPage = 3
	setupWizardSceneStarted = time.Unix(1000, 0)
	var snap drawSnapshot
	prepareSetupWizardSceneSnapshot(&snap, setupWizardSceneStarted.Add(500*time.Millisecond))
	if len(snap.mobiles) < 3 || len(snap.prevMobiles) != len(snap.mobiles) {
		t.Fatalf("synthetic mobiles=%d previous=%d", len(snap.mobiles), len(snap.prevMobiles))
	}
	if len(snap.picsNeg)+len(snap.picsZero)+len(snap.picsPos) < 10 {
		t.Fatal("synthetic scene has too little scenery")
	}
	var walker frameMobile
	for _, mobile := range snap.mobiles {
		if mobile.Index == 1 {
			walker = mobile
			break
		}
	}
	if walker.H == snap.prevMobiles[1].H {
		t.Fatal("synthetic walking character did not move between updates")
	}
	if len(snap.bubbles) != 1 {
		t.Fatal("visibility scene has no deterministic speech bubble")
	}
}

func TestSetupWizardIncludesArtworkUpscaleStyles(t *testing.T) {
	initFont()
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })
	setArtworkUpscaleMode(artworkUpscaleSmooth)

	root := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	buildSetupGraphicsPage(root)
	for _, item := range root.Contents {
		if item.Label != "Artwork upscale style" {
			continue
		}
		if item.ItemType != eui.ITEM_DROPDOWN {
			t.Fatalf("artwork upscale control type = %v, want dropdown", item.ItemType)
		}
		if !slices.Equal(item.Options, artworkUpscaleModeNames) {
			t.Fatalf("artwork upscale options = %v, want %v", item.Options, artworkUpscaleModeNames)
		}
		if item.Selected != artworkUpscaleSmooth {
			t.Fatalf("selected artwork upscale mode = %d, want Smooth", item.Selected)
		}
		return
	}
	t.Fatal("setup wizard has no artwork upscale style dropdown")
}

func TestSetupWizardOffersBlendImageDithering(t *testing.T) {
	initFont()
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })
	gs.DenoiseImages = false

	root := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	buildSetupGraphicsPage(root)
	for _, group := range root.Contents {
		for _, item := range group.Contents {
			if item.Text != "Blend image dithering" {
				continue
			}
			if item.ItemType != eui.ITEM_CHECKBOX {
				t.Fatalf("blend image dithering control type = %v, want checkbox", item.ItemType)
			}
			if item.Checked {
				t.Fatal("blend image dithering should reflect its default-off setting")
			}
			return
		}
	}
	t.Fatal("setup wizard has no Blend image dithering option")
}

func TestSetupWizardOffersGraphicsPerformanceTest(t *testing.T) {
	initFont()
	originalRecommendation := setupWizardGraphicsRecommendation
	setupWizardGraphicsRecommendation = "Full Quality (Recommended)"
	t.Cleanup(func() { setupWizardGraphicsRecommendation = originalRecommendation })
	root := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	buildSetupGraphicsPage(root)
	foundButton := false
	foundRecommendation := false
	for _, group := range root.Contents {
		for _, item := range append([]*eui.ItemData{group}, group.Contents...) {
			if item.Text == "Test Graphics Performance" && item.ItemType == eui.ITEM_BUTTON {
				foundButton = true
			}
			if item.Text == "Full Quality (Recommended)" {
				foundRecommendation = true
			}
		}
	}
	if !foundButton || !foundRecommendation {
		t.Fatalf("graphics test button=%v recommendation=%v", foundButton, foundRecommendation)
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
