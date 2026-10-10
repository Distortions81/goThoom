package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func preserveSoundFontMigrationTestState(t *testing.T) {
	t.Helper()
	originalSettings, originalDataDir := gs, dataDirPath
	originalLoaded, originalDirty, originalHost := settingsLoaded, settingsDirty, host
	originalResampling := highQualityResamplingEnabled()
	t.Cleanup(func() {
		gs, dataDirPath = originalSettings, originalDataDir
		settingsLoaded, settingsDirty, host = originalLoaded, originalDirty, originalHost
		setHighQualityResamplingEnabled(originalResampling)
		syncTTSBlocklist()
	})
	dataDirPath = t.TempDir()
	gs = gsdef
	settingsDirty = false
}

func TestSoundFontDefaultMigrationOnce(t *testing.T) {
	for _, test := range []struct {
		name, document, want string
	}{
		{"modern previous default", fmt.Sprintf(`{"version":%d,"audio":{"music_soundfont":"soundfont.sf2"}}`, SETTINGS_VERSION), soundFontFile},
		{"modern explicit pending marker", fmt.Sprintf(`{"version":%d,"audio":{"music_soundfont":"soundfont.sf2","soundfont_default_migrated":false}}`, SETTINGS_VERSION), soundFontFile},
		{"modern no selection", fmt.Sprintf(`{"version":%d}`, SETTINGS_VERSION), soundFontFile},
		{"modern empty selection", fmt.Sprintf(`{"version":%d,"audio":{"music_soundfont":""}}`, SETTINGS_VERSION), soundFontFile},
		{"modern custom", fmt.Sprintf(`{"version":%d,"audio":{"music_soundfont":"custom.sf2"}}`, SETTINGS_VERSION), "custom.sf2"},
		{"legacy previous default", `{"Version":3,"SoundFontFile":"soundfont.sf2"}`, soundFontFile},
		{"legacy no selection", `{"Version":3}`, soundFontFile},
		{"legacy custom", `{"Version":3,"SoundFontFile":"custom.sf2"}`, "custom.sf2"},
	} {
		t.Run(test.name, func(t *testing.T) {
			preserveSoundFontMigrationTestState(t)
			if err := os.WriteFile(filepath.Join(dataDirPath, settingsFile), []byte(test.document), 0o600); err != nil {
				t.Fatal(err)
			}
			if !loadSettings() {
				t.Fatal("valid existing settings did not load")
			}
			if gs.SoundFontFile != test.want || !gs.SoundFontDefaultMigrated || !settingsDirty {
				t.Fatalf("first load: SoundFont=%q, migrated=%v, dirty=%v; want %q and persisted migration", gs.SoundFontFile, gs.SoundFontDefaultMigrated, settingsDirty, test.want)
			}
			if err := writeSettingsFile(gs); err != nil {
				t.Fatal(err)
			}
			assertSoundFontMigrationSaved(t)
			gs = gsdef
			if !loadSettings() || gs.SoundFontFile != test.want || !gs.SoundFontDefaultMigrated {
				t.Fatalf("reloaded migration: SoundFont=%q, migrated=%v; want %q", gs.SoundFontFile, gs.SoundFontDefaultMigrated, test.want)
			}

			// Selecting the previous bank after the upgrade must survive the
			// next startup rather than being mistaken for an unmigrated default.
			gs.SoundFontFile = alternateSoundFontFile
			if err := writeSettingsFile(gs); err != nil {
				t.Fatal(err)
			}
			gs = gsdef
			if !loadSettings() || gs.SoundFontFile != alternateSoundFontFile || !gs.SoundFontDefaultMigrated {
				t.Fatalf("saved alternate was migrated again: SoundFont=%q, migrated=%v", gs.SoundFontFile, gs.SoundFontDefaultMigrated)
			}
		})
	}
}

func TestSoundFontDefaultMigrationFreshInstallChoice(t *testing.T) {
	preserveSoundFontMigrationTestState(t)
	if loadSettings() {
		t.Fatal("missing settings file unexpectedly loaded")
	}
	if gs.SoundFontFile != soundFontFile || !gs.SoundFontDefaultMigrated || !settingsDirty {
		t.Fatalf("fresh defaults: SoundFont=%q, migrated=%v, dirty=%v", gs.SoundFontFile, gs.SoundFontDefaultMigrated, settingsDirty)
	}
	// The user's first save can already contain an alternate choice. There is
	// no earlier saved qt.sf2 preference on which the migration can depend.
	gs.SoundFontFile = alternateSoundFontFile
	if err := writeSettingsFile(gs); err != nil {
		t.Fatal(err)
	}
	assertSoundFontMigrationSaved(t)
	gs = gsdef
	if !loadSettings() || gs.SoundFontFile != alternateSoundFontFile || !gs.SoundFontDefaultMigrated {
		t.Fatalf("fresh install's first alternate choice was replaced: SoundFont=%q, migrated=%v", gs.SoundFontFile, gs.SoundFontDefaultMigrated)
	}
}

func TestExplicitSoundFontSelectionCompletesPendingMigration(t *testing.T) {
	preserveSoundFontMigrationTestState(t)
	preserveStoragePathTestState(t)
	storagePathsActivated = false
	oldFont, oldSynthSettings, oldSynthGeneration := sfntCached, synthSettings, synthGeneration
	musicPlayersMu.Lock()
	oldPlayers, oldMusicGeneration := musicPlayers, musicSettingsGeneration
	musicPlayers = nil
	musicPlayersMu.Unlock()
	t.Cleanup(func() {
		synthCacheMu.Lock()
		sfntCached, synthSettings, synthGeneration = oldFont, oldSynthSettings, oldSynthGeneration
		synthCacheMu.Unlock()
		musicPlayersMu.Lock()
		musicPlayers, musicSettingsGeneration = oldPlayers, oldMusicGeneration
		musicPlayersMu.Unlock()
	})
	if err := os.WriteFile(alternateSoundFontPath(), bundledSoundFont, 0o644); err != nil {
		t.Fatal(err)
	}
	// Defaults can also be restored from the UI during a running session.
	gs = gsdef
	if err := selectSoundFont(alternateSoundFontFile); err != nil {
		t.Fatal(err)
	}
	if !gs.SoundFontDefaultMigrated {
		t.Fatal("explicit selection left the default migration pending")
	}
	if err := writeSettingsFile(gs); err != nil {
		t.Fatal(err)
	}
	gs = gsdef
	if !loadSettings() || gs.SoundFontFile != alternateSoundFontFile {
		t.Fatalf("explicit selection was replaced on restart: %q", gs.SoundFontFile)
	}
}

func TestSoundFontDefaultMigrationFallbackLoads(t *testing.T) {
	for _, test := range []struct{ name, document string }{
		{"malformed JSON", `{"version":`},
		{"unsupported version", `{"version":999}`},
		{"invalid modern field", fmt.Sprintf(`{"version":%d,"audio":{"music_soundfont":17}}`, SETTINGS_VERSION)},
		{"invalid legacy field", `{"Version":3,"SoundFontFile":17}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			preserveSoundFontMigrationTestState(t)
			if err := os.WriteFile(filepath.Join(dataDirPath, settingsFile), []byte(test.document), 0o600); err != nil {
				t.Fatal(err)
			}
			if loadSettings() {
				t.Fatal("invalid settings unexpectedly loaded")
			}
			if gs.SoundFontFile != soundFontFile || !gs.SoundFontDefaultMigrated || !settingsDirty {
				t.Fatalf("fallback defaults: SoundFont=%q, migrated=%v, dirty=%v", gs.SoundFontFile, gs.SoundFontDefaultMigrated, settingsDirty)
			}
		})
	}
}

func assertSoundFontMigrationSaved(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dataDirPath, settingsFile))
	if err != nil {
		t.Fatal(err)
	}
	var saved struct {
		Version int `json:"version"`
		Audio   struct {
			Migrated bool `json:"soundfont_default_migrated"`
		} `json:"audio"`
	}
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Version != SETTINGS_VERSION || !saved.Audio.Migrated {
		t.Fatalf("migration marker not saved in current-version audio settings: version=%d, marker=%v", saved.Version, saved.Audio.Migrated)
	}
}
