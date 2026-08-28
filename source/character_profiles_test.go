package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gothoom/eui"
)

func TestCharacterProfileAppliesOnlyProfileSettings(t *testing.T) {
	base := gsdef
	base.ServerAddress = "global.example:5010"
	base.MasterVolume = 0.25
	base.Notifications = false
	base.Theme = "Global Theme"
	base.MessageTextColors = map[string]eui.Color{"speech": eui.NewColor(1, 2, 3, 255)}

	character := cloneSettings(base)
	character.ServerAddress = "should-not-be-profiled:5010"
	character.MasterVolume = 0.8
	character.Notifications = true
	character.Theme = "Character Theme"
	character.MessageTextColors = map[string]eui.Color{"speech": eui.NewColor(9, 8, 7, 255)}
	character.GameWindow.Position = WindowPoint{X: 0.2, Y: 0.3}

	profile, err := captureCharacterProfile("Hardia", character)
	if err != nil {
		t.Fatal(err)
	}
	got, err := applyCharacterProfile(base, profile)
	if err != nil {
		t.Fatal(err)
	}
	if got.ServerAddress != base.ServerAddress {
		t.Fatalf("server address = %q, want global %q", got.ServerAddress, base.ServerAddress)
	}
	if got.MasterVolume != character.MasterVolume || got.Notifications != character.Notifications || got.Theme != character.Theme {
		t.Fatalf("profile settings not applied: volume=%v notifications=%v theme=%q", got.MasterVolume, got.Notifications, got.Theme)
	}
	if got.GameWindow.Position != character.GameWindow.Position {
		t.Fatalf("window position = %+v, want %+v", got.GameWindow.Position, character.GameWindow.Position)
	}
	if got.MessageTextColors["speech"] != character.MessageTextColors["speech"] {
		t.Fatal("message appearance did not come from character profile")
	}
}

func TestCharacterProfilesDocumentRoundTrip(t *testing.T) {
	originalDir := dataDirPath
	originalProfiles := characterProfiles
	dataDirPath = t.TempDir()
	t.Cleanup(func() {
		dataDirPath = originalDir
		characterProfiles = originalProfiles
	})

	profile, err := captureCharacterProfile("Gaia O'Neill", gsdef)
	if err != nil {
		t.Fatal(err)
	}
	characterProfiles = characterProfilesDocument{
		Version: characterProfilesVersion,
		Enabled: map[string]bool{"gaia o'neill": true},
		Profiles: map[string]characterProfile{
			characterProfileKey(profile.Name): profile,
		},
	}
	if err := saveCharacterProfilesDocument(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dataDirPath, characterProfilesFile))
	if err != nil {
		t.Fatal(err)
	}
	var stored characterProfilesDocument
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatal(err)
	}
	got, ok := stored.Profiles["gaia o'neill"]
	if !ok || got.Name != "Gaia O'Neill" {
		t.Fatalf("stored profile = %#v", stored.Profiles)
	}
	if !stored.Enabled["gaia o'neill"] {
		t.Fatal("stored profile was not enabled")
	}
}

func TestSettingsForSaveKeepsProfileValuesOutOfGlobalSettings(t *testing.T) {
	originalDir := dataDirPath
	originalSettings := gs
	originalBase := globalSettingsBase
	originalBaseReady := globalSettingsBaseReady
	originalActive := activeCharacterProfile
	originalProfiles := characterProfiles
	dataDirPath = t.TempDir()
	t.Cleanup(func() {
		dataDirPath = originalDir
		gs = originalSettings
		globalSettingsBase = originalBase
		globalSettingsBaseReady = originalBaseReady
		activeCharacterProfile = originalActive
		characterProfiles = originalProfiles
	})

	globalSettingsBase = cloneSettings(gsdef)
	globalSettingsBase.MasterVolume = 0.2
	globalSettingsBase.ServerAddress = "global.example:5010"
	globalSettingsBaseReady = true
	activeCharacterProfile = "hardia"
	characterProfiles = characterProfilesDocument{
		Version:  characterProfilesVersion,
		Enabled:  map[string]bool{"hardia": true},
		Profiles: make(map[string]characterProfile),
	}
	gs = cloneSettings(globalSettingsBase)
	gs.LastCharacter = "Hardia"
	gs.MasterVolume = 0.9
	gs.ServerAddress = "new-global.example:5010"

	global := settingsForSave()
	if global.MasterVolume != 0.2 {
		t.Fatalf("global master volume = %v, want 0.2", global.MasterVolume)
	}
	if global.ServerAddress != gs.ServerAddress {
		t.Fatalf("global server address = %q, want %q", global.ServerAddress, gs.ServerAddress)
	}
	profile := characterProfiles.Profiles["hardia"]
	profiled, err := applyCharacterProfile(globalSettingsBase, profile)
	if err != nil {
		t.Fatal(err)
	}
	if profiled.MasterVolume != 0.9 {
		t.Fatalf("profile master volume = %v, want 0.9", profiled.MasterVolume)
	}
}

func TestInitializeCharacterProfilesOverlaysLastCharacter(t *testing.T) {
	originalDir := dataDirPath
	originalBase := globalSettingsBase
	originalBaseReady := globalSettingsBaseReady
	originalActive := activeCharacterProfile
	originalProfiles := characterProfiles
	dataDirPath = t.TempDir()
	t.Cleanup(func() {
		dataDirPath = originalDir
		globalSettingsBase = originalBase
		globalSettingsBaseReady = originalBaseReady
		activeCharacterProfile = originalActive
		characterProfiles = originalProfiles
	})

	global := cloneSettings(gsdef)
	global.LastCharacter = "Hardia"
	global.MasterVolume = 0.2
	profileSettings := cloneSettings(global)
	profileSettings.MasterVolume = 0.85
	profile, err := captureCharacterProfile("Hardia", profileSettings)
	if err != nil {
		t.Fatal(err)
	}
	characterProfiles = characterProfilesDocument{
		Version:  characterProfilesVersion,
		Enabled:  map[string]bool{"hardia": true},
		Profiles: map[string]characterProfile{"hardia": profile},
	}
	if err := saveCharacterProfilesDocument(); err != nil {
		t.Fatal(err)
	}

	got := initializeCharacterProfiles(global)
	if got.MasterVolume != 0.85 {
		t.Fatalf("profile master volume = %v, want 0.85", got.MasterVolume)
	}
	if globalSettingsBase.MasterVolume != 0.2 {
		t.Fatalf("global base master volume = %v, want 0.2", globalSettingsBase.MasterVolume)
	}
	if activeCharacterProfile != "hardia" {
		t.Fatalf("active profile = %q, want hardia", activeCharacterProfile)
	}
}

func TestInitializeCharacterProfilesDefaultsToGlobalSettings(t *testing.T) {
	originalDir := dataDirPath
	originalBase := globalSettingsBase
	originalBaseReady := globalSettingsBaseReady
	originalActive := activeCharacterProfile
	originalProfiles := characterProfiles
	dataDirPath = t.TempDir()
	t.Cleanup(func() {
		dataDirPath = originalDir
		globalSettingsBase = originalBase
		globalSettingsBaseReady = originalBaseReady
		activeCharacterProfile = originalActive
		characterProfiles = originalProfiles
	})

	global := cloneSettings(gsdef)
	global.LastCharacter = "Hardia"
	global.MasterVolume = 0.2
	profileSettings := cloneSettings(global)
	profileSettings.MasterVolume = 0.85
	profile, err := captureCharacterProfile("Hardia", profileSettings)
	if err != nil {
		t.Fatal(err)
	}
	characterProfiles = characterProfilesDocument{
		Version:  characterProfilesVersion,
		Profiles: map[string]characterProfile{"hardia": profile},
	}
	if err := saveCharacterProfilesDocument(); err != nil {
		t.Fatal(err)
	}

	got := initializeCharacterProfiles(global)
	if got.MasterVolume != 0.2 {
		t.Fatalf("disabled profile changed global volume to %v", got.MasterVolume)
	}
	if activeCharacterProfile != "" {
		t.Fatalf("active profile = %q, want global settings", activeCharacterProfile)
	}
}

func TestSwitchCharacterProfileSavesOutgoingAndLoadsIncoming(t *testing.T) {
	originalDir := dataDirPath
	originalSettings := gs
	originalBase := globalSettingsBase
	originalBaseReady := globalSettingsBaseReady
	originalActive := activeCharacterProfile
	originalProfiles := characterProfiles
	originalUIReady := uiReady
	originalDirty := settingsDirty
	dataDirPath = t.TempDir()
	t.Cleanup(func() {
		dataDirPath = originalDir
		gs = originalSettings
		globalSettingsBase = originalBase
		globalSettingsBaseReady = originalBaseReady
		activeCharacterProfile = originalActive
		characterProfiles = originalProfiles
		uiReady = originalUIReady
		settingsDirty = originalDirty
	})

	base := cloneSettings(gsdef)
	base.MasterVolume = 0.2
	base.LastCharacter = "Alpha"
	alphaSettings := cloneSettings(base)
	alphaSettings.MasterVolume = 0.6
	betaSettings := cloneSettings(base)
	betaSettings.MasterVolume = 0.9
	alpha, err := captureCharacterProfile("Alpha", alphaSettings)
	if err != nil {
		t.Fatal(err)
	}
	beta, err := captureCharacterProfile("Beta", betaSettings)
	if err != nil {
		t.Fatal(err)
	}

	globalSettingsBase = base
	globalSettingsBaseReady = true
	activeCharacterProfile = "alpha"
	characterProfiles = characterProfilesDocument{
		Version:  characterProfilesVersion,
		Enabled:  map[string]bool{"alpha": true, "beta": true},
		Profiles: map[string]characterProfile{"alpha": alpha, "beta": beta},
	}
	gs = alphaSettings
	gs.LastCharacter = "Alpha"
	gs.MasterVolume = 0.7 // Unsaved outgoing change.
	uiReady = false

	switchCharacterProfile("Beta")
	if gs.LastCharacter != "Beta" || gs.MasterVolume != 0.9 {
		t.Fatalf("active beta profile = character %q volume %v", gs.LastCharacter, gs.MasterVolume)
	}
	savedAlpha, err := applyCharacterProfile(base, characterProfiles.Profiles["alpha"])
	if err != nil {
		t.Fatal(err)
	}
	if savedAlpha.MasterVolume != 0.7 {
		t.Fatalf("outgoing alpha volume = %v, want 0.7", savedAlpha.MasterVolume)
	}
	data, err := os.ReadFile(filepath.Join(dataDirPath, settingsFile))
	if err != nil {
		t.Fatal(err)
	}
	global, err := unmarshalSettingsDocument(data, gsdef)
	if err != nil {
		t.Fatal(err)
	}
	if global.MasterVolume != 0.2 || global.LastCharacter != "Beta" {
		t.Fatalf("global settings = volume %v character %q", global.MasterVolume, global.LastCharacter)
	}
}

func TestDisabledCharactersShareGlobalSettings(t *testing.T) {
	originalDir := dataDirPath
	originalSettings := gs
	originalBase := globalSettingsBase
	originalBaseReady := globalSettingsBaseReady
	originalActive := activeCharacterProfile
	originalProfiles := characterProfiles
	originalUIReady := uiReady
	dataDirPath = t.TempDir()
	t.Cleanup(func() {
		dataDirPath = originalDir
		gs = originalSettings
		globalSettingsBase = originalBase
		globalSettingsBaseReady = originalBaseReady
		activeCharacterProfile = originalActive
		characterProfiles = originalProfiles
		uiReady = originalUIReady
	})

	gs = cloneSettings(gsdef)
	gs.LastCharacter = "Alpha"
	gs.MasterVolume = 0.4
	globalSettingsBase = cloneSettings(gs)
	globalSettingsBaseReady = true
	activeCharacterProfile = ""
	characterProfiles = characterProfilesDocument{Version: characterProfilesVersion}
	uiReady = false

	gs.MasterVolume = 0.65
	switchCharacterProfile("Beta")
	if gs.LastCharacter != "Beta" || gs.MasterVolume != 0.65 {
		t.Fatalf("global settings after switch = character %q volume %v", gs.LastCharacter, gs.MasterVolume)
	}
	if activeCharacterProfile != "" || len(characterProfiles.Profiles) != 0 {
		t.Fatalf("disabled switch created profile state: active=%q profiles=%d", activeCharacterProfile, len(characterProfiles.Profiles))
	}
}

func TestCharacterProfileTogglePreservesGlobalAndProfileSettings(t *testing.T) {
	originalDir := dataDirPath
	originalSettings := gs
	originalBase := globalSettingsBase
	originalBaseReady := globalSettingsBaseReady
	originalActive := activeCharacterProfile
	originalProfiles := characterProfiles
	originalUIReady := uiReady
	dataDirPath = t.TempDir()
	t.Cleanup(func() {
		dataDirPath = originalDir
		gs = originalSettings
		globalSettingsBase = originalBase
		globalSettingsBaseReady = originalBaseReady
		activeCharacterProfile = originalActive
		characterProfiles = originalProfiles
		uiReady = originalUIReady
	})

	gs = cloneSettings(gsdef)
	gs.LastCharacter = "Hardia"
	gs.MasterVolume = 0.2
	globalSettingsBase = cloneSettings(gs)
	globalSettingsBaseReady = true
	activeCharacterProfile = ""
	characterProfiles = characterProfilesDocument{Version: characterProfilesVersion}
	uiReady = false

	setCharacterProfileEnabled("Hardia", true)
	if !characterProfileEnabled("Hardia") || activeCharacterProfile != "hardia" {
		t.Fatalf("profile was not enabled: active=%q enabled=%v", activeCharacterProfile, characterProfileEnabled("Hardia"))
	}
	gs.MasterVolume = 0.8
	setCharacterProfileEnabled("Hardia", false)
	if characterProfileEnabled("Hardia") || activeCharacterProfile != "" || gs.MasterVolume != 0.2 {
		t.Fatalf("global settings not restored: active=%q enabled=%v volume=%v", activeCharacterProfile, characterProfileEnabled("Hardia"), gs.MasterVolume)
	}

	setCharacterProfileEnabled("Hardia", true)
	if gs.MasterVolume != 0.8 {
		t.Fatalf("saved profile volume = %v, want 0.8", gs.MasterVolume)
	}
}
