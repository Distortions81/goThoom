package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Real-audio integration tests read an installed font without redirecting test
// data writes into the user's installation. Respect its Assets & Audio folder
// and selected font; a repo-local font remains a usable CI fixture.
func soundFontForTest(t *testing.T) string {
	t.Helper()
	root := platformDataDir(runtime.GOOS, runtime.GOARCH, os.Getenv, os.UserHomeDir, os.Executable)
	candidates := soundFontTestCandidates(root)
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			t.Logf("SoundFont: %s", path)
			return path
		}
	}
	t.Skipf("real-audio integration needs an installed SoundFont; checked %v", candidates)
	return ""
}

func soundFontTestCandidates(root string) []string {
	candidates := []string{soundFontPath()}
	installed := gsdef
	if data, err := os.ReadFile(filepath.Join(root, settingsFile)); err == nil {
		if value, err := unmarshalSettingsDocument(data, gsdef); err == nil {
			installed = value
		}
	}
	assets := root
	if installed.AssetsPath != "" {
		assets = filepath.Clean(installed.AssetsPath)
	}
	name := installed.SoundFontFile
	if name == "" || filepath.Base(name) != name || !strings.EqualFold(filepath.Ext(name), ".sf2") {
		name = soundFontFile
	}
	candidates = append(candidates, filepath.Join(assets, name), filepath.Join(assets, soundFontFile), filepath.Join(root, soundFontFile))
	return candidates
}

func TestSoundFontFixtureRespectsInstalledAssetSettings(t *testing.T) {
	root := t.TempDir()
	value := gsdef
	value.AssetsPath = filepath.Join(root, "custom audio")
	value.SoundFontFile = "Selected.sf2"
	data, err := marshalSettingsDocument(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, settingsFile), data, 0o600); err != nil {
		t.Fatal(err)
	}
	candidates := soundFontTestCandidates(root)
	if candidates[1] != filepath.Join(value.AssetsPath, value.SoundFontFile) || candidates[2] != filepath.Join(value.AssetsPath, soundFontFile) {
		t.Fatalf("installed SoundFont settings ignored: %v", candidates)
	}
	after, err := os.ReadFile(filepath.Join(root, settingsFile))
	if err != nil || string(after) != string(data) {
		t.Fatal("fixture lookup modified installed settings")
	}
}
