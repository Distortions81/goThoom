package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSettingsV4SchemaCoversPersistedFields(t *testing.T) {
	mapped := make(map[string]bool)
	names := make(map[string]bool)
	for _, entry := range settingsSchema {
		if mapped[entry.field] {
			t.Errorf("settings field %s is mapped more than once", entry.field)
		}
		mapped[entry.field] = true
		fullName := entry.category + "." + entry.name
		if names[fullName] {
			t.Errorf("settings key %s is used more than once", fullName)
		}
		names[fullName] = true
	}

	special := map[string]bool{
		"Version":             true,
		"BarPlacement":        true,
		"SpriteUpscaleMode":   true,
		"SpriteUpscale":       true, // derived from artwork_scale
		"SpriteUpscaleFilter": true, // derived from artwork_upscale_style
	}
	typ := reflect.TypeOf(settings{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" { // Runtime/debug state was never persisted.
			continue
		}
		if !mapped[field.Name] && !special[field.Name] {
			t.Errorf("exported settings field %s is missing from the v4 schema", field.Name)
		}
	}
}

func TestBatchArtworkLoadingDefaultsOn(t *testing.T) {
	if !gsdef.BatchArtworkLoading {
		t.Fatal("room artwork batching must default on")
	}
}

func TestAssetActivityIndicatorsDefaultOff(t *testing.T) {
	if gsdef.AssetActivityIndicators {
		t.Fatal("asset activity indicators must default off")
	}
}

func TestMobileLightConeShadowsDefaultOff(t *testing.T) {
	if gsdef.MobileLightConeShadows {
		t.Fatal("mobile light-cone shadows must default off")
	}
}

func TestSettingsV4RoundTrip(t *testing.T) {
	want := gsdef
	want.LastCharacter = "Agratis"
	want.GameScale = 2
	want.SpriteUpscale = 2
	want.SpriteUpscaleMode = artworkUpscaleCrisp
	want.SpriteUpscaleFilter = true
	want.BarPlacement = BarPlacementUpperRight
	want.MasterVolume = 0.42
	want.MusicEnhancementAmount = 1.73
	want.AltNetMode = false
	want.AltNetDelay = 37
	want.Enabledscripts = map[string]any{"hello": "all"}
	want.LegacyMacroContinuous = true
	want.BatchArtworkLoading = false
	want.AssetActivityIndicators = true
	want.MobileLightConeShadows = true

	data, err := marshalSettingsDocument(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := unmarshalSettingsDocument(data, gsdef)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("v4 settings round trip differs\n got: %#v\nwant: %#v", got, want)
	}

	text := string(data)
	for _, expected := range []string{
		`"version": 4`,
		`"speech_bubbles"`,
		`"artwork_upscale_style": "crisp"`,
		`"status_bar_placement": "upper_right"`,
		`"alternate_network_delay_ms": 37`,
		`"dark_mode_names_and_bubbles"`,
		`"allow_continuous_legacy_macros": true`,
		`"music_enhancement_amount": 1.73`,
		`"batch_room_artwork_loading": false`,
		`"show_asset_activity_indicators": true`,
		`"mobile_light_cone_shadows": true`,
		`"shaders_enabled": true`,
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("v4 settings missing %s", expected)
		}
	}
	for _, legacy := range []string{`"Version"`, `"SpriteUpscale"`, `"BlendPicts"`, `"dark_bubbles_and_names"`} {
		if strings.Contains(text, legacy) {
			t.Errorf("v4 settings retained legacy key %s", legacy)
		}
	}
	for _, obsolete := range []string{`"character_animation_blend_frames"`, `"world_animation_blend_frames"`} {
		if strings.Contains(text, obsolete) {
			t.Errorf("v4 settings retained obsolete key %s", obsolete)
		}
	}
}

func TestSettingsV4ReadsOldDarkNamesKey(t *testing.T) {
	data, err := marshalSettingsDocument(gsdef)
	if err != nil {
		t.Fatal(err)
	}
	var doc settingsDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	delete(doc.Interface, "dark_mode_names_and_bubbles")
	doc.Interface["dark_bubbles_and_names"] = json.RawMessage("false")
	data, err = json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}

	got, err := unmarshalSettingsDocument(data, gsdef)
	if err != nil {
		t.Fatal(err)
	}
	if got.DarkBubblesAndNames {
		t.Fatal("old dark_bubbles_and_names setting was not preserved")
	}
}

func TestSettingsV3MigratesToV4(t *testing.T) {
	originalSettings := gs
	originalDataDir := dataDirPath
	originalLoaded := settingsLoaded
	originalDirty := settingsDirty
	originalHost := host
	dataDirPath = t.TempDir()
	t.Cleanup(func() {
		gs = originalSettings
		dataDirPath = originalDataDir
		settingsLoaded = originalLoaded
		settingsDirty = originalDirty
		host = originalHost
		setHighQualityResamplingEnabled(gs.HighQualityResampling)
	})

	legacy := []byte(`{
  "Version": 3,
  "LastCharacter": "Migrated Hero",
  "GameScale": 2,
  "SpriteUpscale": 2,
  "SpriteUpscaleFilter": true,
  "SpriteUpscaleMode": 3,
  "BarPlacement": 2,
  "MasterVolume": 0.4,
  "AltNetMode": false,
  "AltNetDelay": 44
}`)
	path := filepath.Join(dataDirPath, settingsFile)
	if err := os.WriteFile(path, legacy, 0o644); err != nil {
		t.Fatal(err)
	}

	settingsDirty = false
	if !loadSettings() {
		t.Fatal("v3 settings were not migrated")
	}
	if gs.Version != SETTINGS_VERSION || gs.LastCharacter != "Migrated Hero" || gs.GameScale != 2 || gs.SpriteUpscaleMode != artworkUpscaleSmooth {
		t.Fatalf("migrated settings = version:%d character:%q scale:%v mode:%d", gs.Version, gs.LastCharacter, gs.GameScale, gs.SpriteUpscaleMode)
	}
	if gs.BarPlacement != BarPlacementLowerRight || gs.MasterVolume != 0.4 || gs.AltNetMode || gs.AltNetDelay != 44 {
		t.Fatalf("legacy values were not preserved: %+v", gs)
	}
	if !settingsDirty {
		t.Error("migration did not mark settings for a v4 save")
	}

	saveSettings()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	if _, ok := root["version"]; !ok {
		t.Error("migrated output has no lowercase v4 version")
	}
	if _, ok := root["Version"]; ok {
		t.Error("migrated output retained the legacy Version key")
	}
}
