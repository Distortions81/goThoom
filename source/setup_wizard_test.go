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

func TestSetupWizardVSyncBypassPreservesSavedSetting(t *testing.T) {
	originalSettings := gs
	originalBypass := setupWizardVSyncBypass
	t.Cleanup(func() {
		gs = originalSettings
		setupWizardVSyncBypass = originalBypass
	})
	gs.VSync = true
	setupWizardVSyncBypass = false
	if !effectiveVSyncEnabled() {
		t.Fatal("VSync should follow the saved setting outside the wizard")
	}
	setupWizardVSyncBypass = true
	if effectiveVSyncEnabled() {
		t.Fatal("wizard did not bypass VSync")
	}
	if !gs.VSync {
		t.Fatal("wizard bypass changed the saved VSync setting")
	}
}

func TestSetupWizardGraphicsDetectionStartsOnSecondPage(t *testing.T) {
	if shouldStartSetupWizardGraphicsDetection(0, false, false) {
		t.Fatal("graphics detection starts on the first page")
	}
	if !shouldStartSetupWizardGraphicsDetection(1, false, false) {
		t.Fatal("graphics detection does not start on the second page")
	}
	if shouldStartSetupWizardGraphicsDetection(1, true, false) {
		t.Fatal("graphics detection restarts while pending")
	}
	if shouldStartSetupWizardGraphicsDetection(1, false, true) {
		t.Fatal("graphics detection restarts after completion")
	}
}

func TestStartSetupWizardGraphicsDetectionResetsFiveSecondSample(t *testing.T) {
	originalSettings := gs
	originalPending := setupWizardGraphicsPending
	originalTested := setupWizardGraphicsTested
	originalStarted := setupWizardGraphicsStarted
	originalSum := setupWizardGraphicsFPSSum
	originalCount := setupWizardGraphicsFPSCount
	originalRecommendation := setupWizardGraphicsRecommendation
	originalBypass := setupWizardVSyncBypass
	t.Cleanup(func() {
		gs = originalSettings
		setHighQualityResamplingEnabled(gs.HighQualityResampling)
		setupWizardVSyncBypass = originalBypass
		applyVSyncSetting()
		setupWizardGraphicsPending = originalPending
		setupWizardGraphicsTested = originalTested
		setupWizardGraphicsStarted = originalStarted
		setupWizardGraphicsFPSSum = originalSum
		setupWizardGraphicsFPSCount = originalCount
		setupWizardGraphicsRecommendation = originalRecommendation
	})

	setupWizardGraphicsTested = true
	setupWizardGraphicsStarted = time.Now()
	setupWizardGraphicsFPSSum = 400
	setupWizardGraphicsFPSCount = 10
	setupWizardGraphicsRecommendation = "old result"
	startSetupWizardGraphicsDetection()

	if !setupWizardGraphicsPending || setupWizardGraphicsTested || !setupWizardGraphicsStarted.IsZero() {
		t.Fatal("graphics detection did not restart in its pending state")
	}
	if setupWizardGraphicsFPSSum != 0 || setupWizardGraphicsFPSCount != 0 || setupWizardGraphicsRecommendation != "" {
		t.Fatal("graphics detection retained samples from the previous run")
	}
	if !gs.BlendMobiles || !gs.BlendPicts || !gs.ShaderLighting || !gs.CharacterShadows || !gs.AnimatedChatBubbles || artworkUpscaleMode() != artworkUpscaleUltraSmooth {
		t.Fatal("graphics detection did not apply Full Quality before sampling")
	}
	if !setupWizardVSyncBypass {
		t.Fatal("graphics detection did not bypass VSync while sampling")
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
	lowHealth := uint8(kColorCodeBackRed << 4)
	colors := make(map[uint8]uint8, len(snap.mobiles))
	for _, mobile := range snap.mobiles {
		colors[mobile.Index] = mobile.Colors
	}
	if colors[2] != lowHealth || colors[3] != lowHealth {
		t.Fatal("synthetic companion and creature are not visibly low on health")
	}
	if _, visible := mobileHealthBarColor(colors[3], snap.descriptors[3].Type); !visible {
		t.Fatal("synthetic creature's low-health bar is not visible with modern health bars")
	}
}

func TestSetupWizardWalkerFacesCurrentTravelDirection(t *testing.T) {
	previousStart := setupWizardSceneStarted
	originalNight := captureMovieNightState()
	t.Cleanup(func() {
		setupWizardSceneStarted = previousStart
		restoreMovieNightState(originalNight)
	})
	const interval = 420 * time.Millisecond
	setupWizardSceneStarted = time.Unix(1000, 0)

	for _, test := range []struct {
		step       int
		facesRight bool
	}{
		{step: 9, facesRight: true},
		{step: 10, facesRight: false},
	} {
		var snap drawSnapshot
		prepareSetupWizardSceneSnapshot(&snap, setupWizardSceneStarted.Add(time.Duration(test.step)*interval))
		var walker frameMobile
		for _, mobile := range snap.mobiles {
			if mobile.Index == 1 {
				walker = mobile
				break
			}
		}
		previous := snap.prevMobiles[walker.Index]
		movingRight := walker.H > previous.H
		facingRight := walker.State < 16 && previous.State < 16
		if movingRight != test.facesRight || facingRight != movingRight {
			t.Fatalf("step %d movement right=%v facing right=%v", test.step, movingRight, facingRight)
		}
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
	foundModeChoice := false
	for _, group := range root.Contents {
		for _, item := range append([]*eui.ItemData{group}, group.Contents...) {
			if item.Text == "Rerun Graphics Detection" && item.ItemType == eui.ITEM_BUTTON {
				foundButton = true
			}
			if item.Text == "Full Quality (Recommended)" {
				foundRecommendation = true
			}
			if item.Label == "Graphics performance mode" && slices.Equal(item.Options, []string{"iGPU Graphics", "Full Quality"}) {
				foundModeChoice = true
			}
		}
	}
	if !foundButton || !foundRecommendation || !foundModeChoice {
		t.Fatalf("graphics test button=%v recommendation=%v mode choice=%v", foundButton, foundRecommendation, foundModeChoice)
	}
}

func TestSetupWizardUsesTwoAsMinimumGameUpscale(t *testing.T) {
	initFont()
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })
	gs.GameScale = 1

	root := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	buildSetupGraphicsPage(root)
	if gs.GameScale != 2 {
		t.Fatalf("wizard game scale = %v, want minimum 2", gs.GameScale)
	}
	for _, group := range root.Contents {
		for _, item := range group.Contents {
			if item.Label == "Upscale game amount" {
				if item.MinValue != 2 || item.Value != 2 {
					t.Fatalf("upscale slider min/value = %v/%v, want 2/2", item.MinValue, item.Value)
				}
				return
			}
		}
	}
	t.Fatal("setup wizard has no game upscale slider")
}

func TestSetupWizardPrecacheRecommendationPill(t *testing.T) {
	initFont()
	recommended := setupWizardRecommendedCheckbox("Precache images", "description", false, true, nil)
	if len(recommended.Contents) < 1 || len(recommended.Contents[0].Contents) != 2 {
		t.Fatalf("recommended checkbox layout = %#v", recommended.Contents)
	}
	if pill := recommended.Contents[0].Contents[1]; pill.Text != "  Recommended" || !pill.Filled {
		t.Fatalf("recommendation pill = %#v", pill)
	}

	notRecommended := setupWizardRecommendedCheckbox("Precache images", "description", false, false, nil)
	if len(notRecommended.Contents) < 1 || len(notRecommended.Contents[0].Contents) != 0 {
		t.Fatal("non-recommended checkbox displayed a recommendation pill")
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
	if !gs.PrecacheSounds {
		t.Error("PrecacheSounds = false, want true")
	}
}
