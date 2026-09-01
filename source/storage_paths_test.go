package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func preserveStoragePathTestState(t *testing.T) {
	t.Helper()
	originalDataDir := dataDirPath
	originalSettings := gs
	originalActivePaths := activeStoragePaths
	originalActivated := storagePathsActivated
	originalGlobalBase := globalSettingsBase
	originalGlobalReady := globalSettingsBaseReady
	originalActiveProfile := activeCharacterProfile
	originalProfiles := characterProfiles
	originalDirty := settingsDirty
	t.Cleanup(func() {
		dataDirPath = originalDataDir
		gs = originalSettings
		activeStoragePaths = originalActivePaths
		storagePathsActivated = originalActivated
		globalSettingsBase = originalGlobalBase
		globalSettingsBaseReady = originalGlobalReady
		activeCharacterProfile = originalActiveProfile
		characterProfiles = originalProfiles
		settingsDirty = originalDirty
	})
}

func TestStoragePathDefaultsAndActivation(t *testing.T) {
	preserveStoragePathTestState(t)
	dataDirPath = t.TempDir()
	gs = gsdef
	gs.AssetsPath = filepath.Join(dataDirPath, "assets")
	gs.MacrosPath = filepath.Join(dataDirPath, "my-macros")
	storagePathsActivated = false

	activateStoragePaths()
	if got := assetsDirPath(); got != gs.AssetsPath {
		t.Fatalf("assets path = %q, want %q", got, gs.AssetsPath)
	}
	if got := soundFontsDirPath(); got != gs.AssetsPath {
		t.Fatalf("soundfonts path = %q, want combined assets path %q", got, gs.AssetsPath)
	}
	if got := macrosDirPath(); got != gs.MacrosPath {
		t.Fatalf("macros path = %q, want %q", got, gs.MacrosPath)
	}
}

func TestChangeStoragePathCopiesAndVerifiesAssetsBeforeCommit(t *testing.T) {
	preserveStoragePathTestState(t)
	dataDirPath = t.TempDir()
	gs = gsdef
	globalSettingsBaseReady = false
	activeCharacterProfile = ""
	storagePathsActivated = false
	for name, contents := range map[string]string{
		CL_ImagesFile:     "image archive",
		CL_SoundsFile:     "sound archive",
		soundFontFile:     "soundfont",
		ttsSubstituteFile: "foo=bar",
		filepath.Join("piper", "voices", "voice.onnx"): "voice",
	} {
		path := filepath.Join(dataDirPath, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	activateStoragePaths()
	destination := filepath.Join(t.TempDir(), "alternate-assets")

	got, err := changeStoragePath(storagePathAssets, destination, true)
	if err != nil {
		t.Fatal(err)
	}
	if got != destination || gs.AssetsPath != destination {
		t.Fatalf("saved assets path = %q, returned %q", gs.AssetsPath, got)
	}
	for name, want := range map[string]string{
		CL_ImagesFile:     "image archive",
		CL_SoundsFile:     "sound archive",
		soundFontFile:     "soundfont",
		ttsSubstituteFile: "foo=bar",
		filepath.Join("piper", "voices", "voice.onnx"): "voice",
	} {
		data, err := os.ReadFile(filepath.Join(destination, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Errorf("copied %s = %q, want %q", name, data, want)
		}
	}

	data, err := os.ReadFile(filepath.Join(dataDirPath, settingsFile))
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := unmarshalSettingsDocument(data, gsdef)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.AssetsPath != destination {
		t.Fatalf("persisted assets path = %q, want %q", loaded.AssetsPath, destination)
	}
	// Runtime paths remain stable until restart.
	if got := assetsDirPath(); got != dataDirPath {
		t.Fatalf("active assets path changed before restart: %q", got)
	}
}

func TestChangeStoragePathFailureLeavesSettingAndDestinationUntouched(t *testing.T) {
	preserveStoragePathTestState(t)
	dataDirPath = t.TempDir()
	gs = gsdef
	storagePathsActivated = false
	if err := os.WriteFile(filepath.Join(dataDirPath, CL_ImagesFile), []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	activateStoragePaths()
	destination := t.TempDir()
	conflict := filepath.Join(destination, CL_ImagesFile)
	if err := os.WriteFile(conflict, []byte("keep destination"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := changeStoragePath(storagePathAssets, destination, true); err == nil {
		t.Fatal("conflicting copy unexpectedly succeeded")
	}
	if gs.AssetsPath != "" {
		t.Fatalf("assets setting changed after failure: %q", gs.AssetsPath)
	}
	data, err := os.ReadFile(conflict)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "keep destination" {
		t.Fatalf("destination conflict was overwritten: %q", data)
	}
	if _, err := os.Stat(filepath.Join(dataDirPath, settingsFile)); !os.IsNotExist(err) {
		t.Fatalf("settings file written after failed path change: %v", err)
	}
}

func TestChangeStoragePathRejectsNonDirectoryBeforeCommit(t *testing.T) {
	preserveStoragePathTestState(t)
	dataDirPath = t.TempDir()
	gs = gsdef
	storagePathsActivated = false
	activateStoragePaths()
	notDirectory := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notDirectory, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := changeStoragePath(storagePathLogs, notDirectory, false); err == nil {
		t.Fatal("non-directory path unexpectedly succeeded")
	}
	if gs.LogsPath != "" {
		t.Fatalf("logs setting changed after failed verification: %q", gs.LogsPath)
	}
}

func TestSettingCommandValidatesFilePathsBeforeUpdating(t *testing.T) {
	preserveStoragePathTestState(t)
	dataDirPath = t.TempDir()
	gs = gsdef
	globalSettingsBaseReady = false
	activeCharacterProfile = ""
	storagePathsActivated = false
	activateStoragePaths()
	entry, err := findSettingEntry("file_paths.go_scripts")
	if err != nil {
		t.Fatal(err)
	}
	notDirectory := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notDirectory, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := setSettingFromText(entry, notDirectory); err == nil {
		t.Fatal("setting command accepted a non-directory path")
	}
	if gs.ScriptsPath != "" {
		t.Fatalf("scripts setting changed after failed command: %q", gs.ScriptsPath)
	}

	destination := filepath.Join(t.TempDir(), "scripts")
	if _, err := setSettingFromText(entry, destination); err != nil {
		t.Fatal(err)
	}
	if gs.ScriptsPath != destination {
		t.Fatalf("scripts setting = %q, want %q", gs.ScriptsPath, destination)
	}
}

func TestCopyStoragePathFilesUsesCategoryContents(t *testing.T) {
	preserveStoragePathTestState(t)
	for _, test := range []struct {
		name  string
		kind  storagePathKind
		files map[string]string
	}{
		{name: "logs", kind: storagePathLogs, files: map[string]string{filepath.Join(diagnosticsDirectoryName, diagnosticsLogName): "diagnostic", filepath.Join("Text Logs", "Gaia", "session.txt"): "chat"}},
		{name: "macros", kind: storagePathMacros, files: map[string]string{filepath.Join("Library", "custom.mac"): "macro"}},
		{name: "scripts", kind: storagePathScripts, files: map[string]string{filepath.Join("My Script", "main.go"): "package main"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := t.TempDir()
			destination := t.TempDir()
			for name, contents := range test.files {
				path := filepath.Join(source, name)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := copyStoragePathFiles(test.kind, source, destination); err != nil {
				t.Fatal(err)
			}
			for name, want := range test.files {
				got, err := os.ReadFile(filepath.Join(destination, name))
				if err != nil {
					t.Fatal(err)
				}
				if string(got) != want {
					t.Errorf("copied %s = %q, want %q", name, got, want)
				}
			}
		})
	}
}

func TestFilePathSettingsJSONRoundTrip(t *testing.T) {
	want := gsdef
	want.AssetsPath = "/mnt/assets"
	want.LogsPath = "/mnt/logs"
	want.MacrosPath = "/mnt/macros"
	want.ScriptsPath = "/mnt/scripts"
	data, err := marshalSettingsDocument(want)
	if err != nil {
		t.Fatal(err)
	}
	var document settingsDocument
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.FilePaths) != len(storagePathKinds) {
		t.Fatalf("file_paths contains %d entries, want %d", len(document.FilePaths), len(storagePathKinds))
	}
	got, err := unmarshalSettingsDocument(data, gsdef)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(storagePathsFromSettings(got), storagePathsFromSettings(want)) {
		t.Fatalf("file path settings did not round trip: got %+v, want %+v", storagePathsFromSettings(got), storagePathsFromSettings(want))
	}
}
