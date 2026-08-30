package main

import (
	"net"
	"slices"
	"testing"
	"time"

	"gothoom/climg"
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
	if setupWizardPageCount != 10 {
		t.Fatalf("setup wizard pages = %d, want 10", setupWizardPageCount)
	}
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

func TestSetupWizardInterfacePageIncludesCoreChoices(t *testing.T) {
	initFont()
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })
	gs.TiledWindows = false
	root := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	buildSetupInterfacePage(root)

	wantLabels := map[string]bool{
		"UI scale":              false,
		"Color theme":           false,
		"Style theme":           false,
		"Toolbar placement":     false,
		"Status bar placement":  false,
		"Player health display": false,
	}
	wantChecks := map[string]bool{
		"Tiled window mode":       false,
		"Dark mode names/bubbles": false,
		"Speech bubbles":          false,
		"Fade obscuring objects":  false,
		"Show toolbar info bar":   false,
	}
	var visit func(*eui.ItemData)
	visit = func(item *eui.ItemData) {
		if _, ok := wantLabels[item.Label]; ok {
			wantLabels[item.Label] = true
		}
		if _, ok := wantChecks[item.Text]; ok {
			wantChecks[item.Text] = true
		}
		for _, child := range item.Contents {
			visit(child)
		}
	}
	visit(root)
	for label, found := range wantLabels {
		if !found {
			t.Errorf("interface page missing %q dropdown", label)
		}
	}
	for label, found := range wantChecks {
		if !found {
			t.Errorf("interface page missing %q checkbox", label)
		}
	}
}

func TestSetupWizardDoesNotOfferClientModePreset(t *testing.T) {
	initFont()
	root := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	buildSetupWelcomePage(root)
	for _, item := range root.Contents {
		if item.Label == "Starting preset" {
			t.Fatal("welcome page still offers the Classic/Modern preset")
		}
	}
}

func TestSetupWizardThemeAndStyleSelectorsUseCurrentChoices(t *testing.T) {
	initFont()
	root := setupWizardThemeStyleSelectors()
	want := map[string]string{
		"Color theme": eui.CurrentThemeName(),
		"Style theme": eui.CurrentStyleName(),
	}
	for _, item := range root.Contents {
		current, ok := want[item.Label]
		if !ok {
			continue
		}
		if item.Selected < 0 || item.Selected >= len(item.Options) {
			t.Fatalf("%s selection %d is outside %d options", item.Label, item.Selected, len(item.Options))
		}
		if got := item.Options[item.Selected]; got != current {
			t.Fatalf("%s selection = %q, want %q", item.Label, got, current)
		}
		delete(want, item.Label)
	}
	for label := range want {
		t.Errorf("wizard is missing %q", label)
	}
}

func TestSetupWizardUsesTwoPanelsForTallPages(t *testing.T) {
	initFont()
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })
	gs.TiledWindows = true

	for _, test := range []struct {
		name  string
		page  int
		build func(*eui.ItemData)
	}{
		{name: "interface", page: 2, build: buildSetupInterfacePage},
		{name: "audio", page: 7, build: buildSetupAudioPage},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
			test.build(root)
			var panels *eui.ItemData
			for _, item := range root.Contents {
				if item.ItemType == eui.ITEM_FLOW && item.FlowType == eui.FLOW_HORIZONTAL && len(item.Contents) == 2 &&
					item.Contents[0].FlowType == eui.FLOW_VERTICAL && item.Contents[1].FlowType == eui.FLOW_VERTICAL {
					panels = item
					break
				}
			}
			if panels == nil {
				t.Fatal("wizard page does not contain two side-by-side panels")
			}
			if panels.Size.X < setupWizardTwoPanelWidth {
				t.Fatalf("two-panel width = %.0f, want at least %.0f", panels.Size.X, setupWizardTwoPanelWidth)
			}
			if setupWizardPageWidth(test.page) != setupWizardTwoPanelWidth {
				t.Fatalf("page width = %.0f, want %.0f", setupWizardPageWidth(test.page), setupWizardTwoPanelWidth)
			}
		})
	}
}

func TestSetupWizardUIScaleUsesLongDetailedSlider(t *testing.T) {
	initFont()
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })

	root := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	buildSetupInterfacePage(root)
	var slider, apply *eui.ItemData
	var visit func(*eui.ItemData)
	visit = func(item *eui.ItemData) {
		if item.ItemType == eui.ITEM_SLIDER && item.Label == "UI scale" {
			slider = item
		}
		if item.ItemType == eui.ITEM_BUTTON && item.Text == "Apply" {
			apply = item
		}
		for _, child := range item.Contents {
			visit(child)
		}
	}
	visit(root)
	if slider == nil || apply == nil {
		t.Fatal("interface page is missing the UI scale slider or Apply button")
	}
	if slider.Size.X < 500 {
		t.Fatalf("UI scale slider width = %.0f, want at least 500", slider.Size.X)
	}
	if slider.MinValue != 0.75 || slider.MaxValue != 4 {
		t.Fatalf("UI scale range = %.2f-%.2f, want 0.75-4", slider.MinValue, slider.MaxValue)
	}
}

func TestSetupWizardShowsTiledSettingsOnlyWhenEnabled(t *testing.T) {
	initFont()
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })
	gs.MessagesToConsole = false

	wantLabels := []string{"Layout", "Inventory / Players", "Console / Chat", "Alternate game side"}
	wantChecks := []string{"Keep game window large", "Combine chat + console"}
	contains := func(root *eui.ItemData, label string) bool {
		var visit func(*eui.ItemData) bool
		visit = func(item *eui.ItemData) bool {
			if item.Label == label || item.Text == label {
				return true
			}
			for _, child := range item.Contents {
				if visit(child) {
					return true
				}
			}
			return false
		}
		return visit(root)
	}

	gs.TiledWindows = false
	disabled := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	buildSetupInterfacePage(disabled)
	for _, name := range append(wantLabels, wantChecks...) {
		if contains(disabled, name) {
			t.Errorf("interface page shows tiled setting %q while tiled mode is disabled", name)
		}
	}

	gs.TiledWindows = true
	enabled := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	buildSetupInterfacePage(enabled)
	for _, name := range append(wantLabels, wantChecks...) {
		if !contains(enabled, name) {
			t.Errorf("interface page is missing tiled setting %q while tiled mode is enabled", name)
		}
	}
}

func TestSetupWizardNamesMessagePlacementForCombinedState(t *testing.T) {
	initFont()
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })
	gs.TiledWindows = true

	for _, test := range []struct {
		name     string
		combined bool
		label    string
		options  []string
	}{
		{
			name:    "separate",
			label:   "Console / Chat",
			options: []string{"Console left, Chat right", "Chat left, Console right"},
		},
		{
			name:     "combined",
			combined: true,
			label:    "Combined chat + console",
			options:  []string{"Combined messages left", "Combined messages right"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			gs.MessagesToConsole = test.combined
			root := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
			buildSetupInterfacePage(root)

			var placement *eui.ItemData
			var visit func(*eui.ItemData)
			visit = func(item *eui.ItemData) {
				if item.ItemType == eui.ITEM_DROPDOWN && item.Label == test.label {
					placement = item
					return
				}
				for _, child := range item.Contents {
					visit(child)
				}
			}
			visit(root)
			if placement == nil {
				t.Fatalf("wizard is missing the %q placement dropdown", test.label)
			}
			if !slices.Equal(placement.Options, test.options) {
				t.Fatalf("%q options = %v, want %v", test.label, placement.Options, test.options)
			}
		})
	}
}

func TestSetupWizardAlternateGameSideDisabledForCenteredLayout(t *testing.T) {
	initFont()
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })

	findGameSide := func(root *eui.ItemData) *eui.ItemData {
		var found *eui.ItemData
		var visit func(*eui.ItemData)
		visit = func(item *eui.ItemData) {
			if found != nil {
				return
			}
			if item.Label == "Alternate game side" {
				found = item
				return
			}
			for _, child := range item.Contents {
				visit(child)
			}
		}
		visit(root)
		return found
	}

	gs.TiledWindows = true
	for _, test := range []struct {
		name     string
		layout   TiledLayout
		disabled bool
	}{
		{name: "centered", layout: TiledLayoutCenter, disabled: true},
		{name: "side", layout: TiledLayoutSide, disabled: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			gs.TiledLayout = test.layout
			root := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
			buildSetupInterfacePage(root)
			gameSide := findGameSide(root)
			if gameSide == nil {
				t.Fatal("wizard is missing the alternate game side control")
			}
			if gameSide.Disabled != test.disabled {
				t.Fatalf("alternate game side disabled = %t, want %t", gameSide.Disabled, test.disabled)
			}
		})
	}
}

func TestSetupWizardAudioChoicesOfferOffAndOnTests(t *testing.T) {
	initFont()
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })

	root := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	buildSetupAudioPage(root)
	for _, label := range []string{"Enhance sound effects", "High quality audio resampling", "Enhance bard music"} {
		var option *eui.ItemData
		var containsCheckbox func(*eui.ItemData) bool
		containsCheckbox = func(item *eui.ItemData) bool {
			if item.ItemType == eui.ITEM_CHECKBOX && item.Text == label {
				return true
			}
			for _, child := range item.Contents {
				if containsCheckbox(child) {
					return true
				}
			}
			return false
		}
		for _, item := range root.Contents {
			if containsCheckbox(item) {
				option = item
				break
			}
		}
		if option == nil {
			t.Fatalf("audio page is missing %q", label)
		}

		buttons := map[string]bool{"Test Off": false, "Test On": false}
		var visit func(*eui.ItemData)
		visit = func(item *eui.ItemData) {
			if item.ItemType == eui.ITEM_BUTTON {
				if _, ok := buttons[item.Text]; ok {
					buttons[item.Text] = true
				}
			}
			for _, child := range item.Contents {
				visit(child)
			}
		}
		visit(option)
		for button, found := range buttons {
			if !found {
				t.Errorf("audio option %q is missing %q", label, button)
			}
		}
	}
}

func TestSetupWizardAudioComparisonsUseNonNotificationSound(t *testing.T) {
	if setupWizardAudioTestSound == sndTink {
		t.Fatal("sound enhancement comparison still uses the notification sound")
	}
}

func TestSetupWizardSeparatesAudioAndNotificationSettings(t *testing.T) {
	initFont()
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })

	root := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	buildSetupAudioPage(root)
	wantAudio := map[string]bool{"Volume": false}
	notificationLabels := map[string]bool{"Game notifications": false, "Notification Settings": false}
	var visit func(*eui.ItemData)
	visit = func(item *eui.ItemData) {
		if _, ok := wantAudio[item.Text]; ok {
			wantAudio[item.Text] = true
		}
		if _, ok := notificationLabels[item.Text]; ok {
			notificationLabels[item.Text] = true
		}
		for _, child := range item.Contents {
			visit(child)
		}
	}
	visit(root)
	for label, found := range wantAudio {
		if !found {
			t.Errorf("audio page is missing %q", label)
		}
	}
	for label, found := range notificationLabels {
		if found {
			t.Errorf("audio page unexpectedly includes %q", label)
		}
	}

	notificationsRoot := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	buildSetupNotificationsPage(notificationsRoot)
	for label := range notificationLabels {
		notificationLabels[label] = false
	}
	visit(notificationsRoot)
	for label, found := range notificationLabels {
		if !found {
			t.Errorf("notifications page is missing %q", label)
		}
	}
}

func TestStartSetupWizardGraphicsDetectionResetsSample(t *testing.T) {
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
	if !gs.BlendPicts || !gs.ShaderLighting || !gs.CharacterShadows || !gs.WindowShadows ||
		!gs.MobilesReceiveSunShadows || !gs.FadeObscuringPictures || !gs.MusicEnhancement ||
		gs.GameScale != 3 || artworkUpscaleMode() != artworkUpscaleBalanced {
		t.Fatal("graphics detection did not apply High before sampling")
	}
	if !setupWizardVSyncBypass {
		t.Fatal("graphics detection did not bypass VSync while sampling")
	}
}

func TestSetupWizardGraphicsDetectionTiming(t *testing.T) {
	for _, test := range []struct {
		name     string
		elapsed  time.Duration
		sample   bool
		complete bool
	}{
		{name: "before hidden warmup", elapsed: 2*time.Second - time.Nanosecond},
		{name: "measurement starts", elapsed: 2 * time.Second, sample: true},
		{name: "measurement continues", elapsed: 3*time.Second - time.Nanosecond, sample: true},
		{name: "measurement completes", elapsed: 3 * time.Second, sample: true, complete: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			sample, complete := setupWizardGraphicsDetectionTiming(test.elapsed)
			if sample != test.sample || complete != test.complete {
				t.Fatalf("timing at %v = sample %t, complete %t; want %t, %t", test.elapsed, sample, complete, test.sample, test.complete)
			}
		})
	}
}

func TestSetupWizardGraphicsRecommendationUsesRenderingDefaults(t *testing.T) {
	originalSettings := gs
	originalRecommendation := setupWizardGraphicsRecommendation
	t.Cleanup(func() {
		gs = originalSettings
		setupWizardGraphicsRecommendation = originalRecommendation
		setHighQualityResamplingEnabled(gs.HighQualityResampling)
	})

	gs = gsdef
	applySetupWizardGraphicsRecommendation(graphicsBenchmarkResult{ActualFPS: 120})
	if gs.BlendMobiles {
		t.Fatal("graphics recommendation enabled character animation blending")
	}
	if got := artworkUpscaleMode(); got != artworkUpscaleBalanced {
		t.Fatalf("graphics recommendation selected artwork upscale mode %d, want Balanced", got)
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
		{page: 2, want: setupWizardSceneIndoor},
		{page: 3, want: setupWizardSceneDay},
		{page: 4, want: setupWizardSceneMotion},
		{page: 5, want: setupWizardSceneDay},
		{page: 6, want: setupWizardSceneNight},
	} {
		setupWizardScenePage = -1
		selectSetupWizardSceneForPage(test.page)
		if setupWizardSceneModeValue != test.want {
			t.Fatalf("page %d scene = %d, want %d", test.page, setupWizardSceneModeValue, test.want)
		}
	}
}

func TestSetupWizardSeparatesDaylightAndNightControls(t *testing.T) {
	initFont()
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })
	gs = gsdef

	shadows := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	buildSetupShadowsPage(shadows)
	night := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	buildSetupNightLightingPage(night)

	shadowChecks := map[string]bool{
		"Character shadows":              false,
		"Characters receive sun shadows": false,
	}
	nightChecks := map[string]bool{
		"Shader lighting effects": false,
		"Flame light flicker":     false,
	}
	nightSliders := map[string]bool{
		"Maximum night darkness": false,
		"Light strength":         false,
		"Glow strength":          false,
	}
	var visit func(*eui.ItemData, map[string]bool, map[string]bool)
	visit = func(item *eui.ItemData, checks, sliders map[string]bool) {
		if _, ok := checks[item.Text]; ok && item.ItemType == eui.ITEM_CHECKBOX {
			checks[item.Text] = true
		}
		if _, ok := sliders[item.Label]; ok && item.ItemType == eui.ITEM_SLIDER {
			sliders[item.Label] = true
		}
		for _, child := range item.Contents {
			visit(child, checks, sliders)
		}
	}
	visit(shadows, shadowChecks, map[string]bool{})
	visit(night, nightChecks, nightSliders)
	shadowHasShader := false
	shadowHasAccurateToggle := false
	var findShader func(*eui.ItemData)
	findShader = func(item *eui.ItemData) {
		if item.Text == "Shader lighting effects" {
			shadowHasShader = true
		}
		if item.Text == "Accurate character shadows" {
			shadowHasAccurateToggle = true
		}
		for _, child := range item.Contents {
			findShader(child)
		}
	}
	findShader(shadows)

	for label, found := range shadowChecks {
		if !found {
			t.Errorf("daylight page missing %q", label)
		}
	}
	for label, found := range nightChecks {
		if !found {
			t.Errorf("night page missing %q", label)
		}
	}
	for label, found := range nightSliders {
		if !found {
			t.Errorf("night page missing %q", label)
		}
	}
	if shadowHasShader {
		t.Fatal("daylight page still contains night shader controls")
	}
	if shadowHasAccurateToggle {
		t.Fatal("daylight page still exposes the always-on accurate shadow composite")
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
	setupWizardPage = 2
	setupWizardSceneStarted = time.Unix(1000, 0)
	var snap drawSnapshot
	prepareSetupWizardSceneSnapshot(&snap, setupWizardSceneStarted.Add(500*time.Millisecond))
	if len(snap.mobiles) < 3 || len(snap.prevMobiles) != len(snap.mobiles) {
		t.Fatalf("synthetic mobiles=%d previous=%d", len(snap.mobiles), len(snap.prevMobiles))
	}
	if len(snap.picsNeg)+len(snap.picsZero)+len(snap.picsPos) < 10 {
		t.Fatal("synthetic scene has too little scenery")
	}
	for _, pictures := range [][]framePicture{snap.picsNeg, snap.picsZero, snap.picsPos} {
		for _, picture := range pictures {
			if picture.Moving {
				t.Fatalf("synthetic scene still contains independently moving picture %d", picture.PictID)
			}
		}
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
	if colors[3] != lowHealth || colors[4] != lowHealth {
		t.Fatal("synthetic companion and apprentice are not visibly low on health")
	}
	if _, visible := mobileHealthBarColor(colors[3], snap.descriptors[3].Type); !visible {
		t.Fatal("synthetic companion's low-health bar is not visible with modern health bars")
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
		{step: 17, facesRight: true},
		{step: 18, facesRight: false},
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

func TestSetupWizardGreysShaderChoicesWhenMasterIsOff(t *testing.T) {
	initFont()
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })
	gs = gsdef
	gs.ShadersEnabled = false

	graphics := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	buildSetupGraphicsPage(graphics)
	motion := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	buildSetupMotionPage(motion)
	shadows := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	buildSetupShadowsPage(shadows)
	night := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	buildSetupNightLightingPage(night)

	wantDisabled := map[string]bool{
		"Character animation blending": false,
		"World animation blending":     false,
		"Shader lighting effects":      false,
	}
	foundUpscale := false
	var visit func(*eui.ItemData)
	visit = func(item *eui.ItemData) {
		name := item.Text
		if name == "" {
			name = item.Label
		}
		if _, ok := wantDisabled[name]; ok && item.Disabled {
			wantDisabled[name] = true
		}
		if name == "Artwork upscale style" {
			foundUpscale = true
			if item.Disabled {
				t.Error("CPU artwork upscaling was greyed out by the shader master")
			}
		}
		for _, child := range item.Contents {
			visit(child)
		}
	}
	for _, page := range []*eui.ItemData{graphics, motion, shadows, night} {
		visit(page)
	}
	for name, disabled := range wantDisabled {
		if !disabled {
			t.Errorf("setup wizard shader control %q was not greyed out", name)
		}
	}
	if !foundUpscale {
		t.Error("setup wizard is missing the artwork upscale control")
	}
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
	setupWizardGraphicsRecommendation = "High (Recommended)"
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
			if item.Text == "High (Recommended)" {
				foundRecommendation = true
			}
			if item.Label == "Graphics performance mode" && slices.Equal(item.Options, []string{"Lowest", "Low", "Medium", "High", "Ultra", "Custom"}) {
				foundModeChoice = true
			}
		}
	}
	if !foundButton || !foundRecommendation || !foundModeChoice {
		t.Fatalf("graphics test button=%v recommendation=%v mode choice=%v", foundButton, foundRecommendation, foundModeChoice)
	}
}

func TestSetupWizardUsesTwoAsMinimumMaxUpscale(t *testing.T) {
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
			if item.Label == "Max Upscale" {
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
	if gs.VSync {
		t.Error("VSync = true, want false")
	}
	if !gs.PrecacheSounds {
		t.Error("PrecacheSounds = false, want true")
	}
}

func TestRefreshLoginAfterAssetsAvailableRebuildsSelectedCharacterRows(t *testing.T) {
	initFont()
	originalWindow := loginWin
	originalList := charactersList
	originalCharacters := characters
	originalName := name
	originalPassHash := passHash
	originalPass := pass
	originalLastCharacter := gs.LastCharacter
	originalConn := tcpConn
	originalMovie := playingMovie
	originalCLMov := clmov
	originalPCAP := pcapPath
	originalFake := fake
	originalImages := clImages
	loginWin = nil
	charactersList = nil
	clImages = nil
	characters = []Character{{Name: "Alice"}}
	name = "Alice"
	passHash = ""
	pass = ""
	gs.LastCharacter = ""
	tcpConn = nil
	playingMovie = false
	clmov = ""
	pcapPath = ""
	fake = false
	t.Cleanup(func() {
		if loginWin != nil {
			loginWin.RemoveWindow()
		}
		loginWin = originalWindow
		charactersList = originalList
		characters = originalCharacters
		name = originalName
		passHash = originalPassHash
		pass = originalPass
		gs.LastCharacter = originalLastCharacter
		tcpConn = originalConn
		playingMovie = originalMovie
		clmov = originalCLMov
		pcapPath = originalPCAP
		fake = originalFake
		clImages = originalImages
	})

	makeLoginWindow()
	updateCharacterButtons()
	if len(charactersList.Contents) != 2 {
		t.Fatalf("character rows = %d, want Demo Character plus Alice", len(charactersList.Contents))
	}
	originalRow := charactersList.Contents[0]

	// Simulate the archive becoming available after Login was built during a
	// first-run launch with no assets.
	clImages = &climg.CLImages{}
	refreshLoginAfterAssetsAvailable()
	if len(charactersList.Contents) != 2 {
		t.Fatalf("refreshed character rows = %d, want Demo Character plus Alice", len(charactersList.Contents))
	}
	if charactersList.Contents[0] == originalRow {
		t.Fatal("login character row was not rebuilt")
	}
	if !loginWin.IsOpen() {
		t.Fatal("login window was not restored after assets became available")
	}
}
