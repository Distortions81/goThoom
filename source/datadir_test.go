package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPlatformDataDir(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home", "player")
	localAppData := filepath.Join(root, "users", "player", "AppData", "Local")
	xdgData := filepath.Join(root, "home", "player", "xdg-data")
	executable := filepath.Join(root, "portable", "goThoom", "goThoom")
	tests := []struct {
		name string
		goos string
		env  map[string]string
		home string
		want string
	}{
		{name: "windows local app data", goos: "windows", env: map[string]string{"LOCALAPPDATA": localAppData}, home: home, want: filepath.Join(localAppData, "goThoom")},
		{name: "windows home fallback", goos: "windows", home: home, want: filepath.Join(home, "AppData", "Local", "goThoom")},
		{name: "windows ignores relative local app data", goos: "windows", env: map[string]string{"LOCALAPPDATA": "relative"}, home: home, want: filepath.Join(home, "AppData", "Local", "goThoom")},
		{name: "linux xdg data", goos: "linux", env: map[string]string{"XDG_DATA_HOME": xdgData}, home: home, want: filepath.Join(xdgData, "goThoom")},
		{name: "linux default", goos: "linux", home: home, want: filepath.Join(home, ".local", "share", "goThoom")},
		{name: "linux ignores relative xdg", goos: "linux", env: map[string]string{"XDG_DATA_HOME": "relative"}, home: home, want: filepath.Join(home, ".local", "share", "goThoom")},
		{name: "macOS existing container location", goos: "darwin", home: home, want: filepath.Join(home, "Library", "Containers", "com.goThoom.client")},
		{name: "macOS sandbox home", goos: "darwin", home: filepath.Join(home, "Library", "Containers", "com.goThoom.client", "Data"), want: filepath.Join(home, "Library", "Containers", "com.goThoom.client")},
		{name: "other platform portable fallback", goos: "freebsd", home: home, want: filepath.Join(filepath.Dir(executable), "data")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			getenv := func(name string) string { return test.env[name] }
			userHome := func() (string, error) {
				if test.home == "" {
					return "", errors.New("no home")
				}
				return test.home, nil
			}
			exe := func() (string, error) { return executable, nil }
			if got := platformDataDir(test.goos, "amd64", getenv, userHome, exe); got != test.want {
				t.Fatalf("platformDataDir() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestMigratePortableDataCopiesMissingFilesOnce(t *testing.T) {
	legacyRoot := t.TempDir()
	destination := t.TempDir()
	writeTestFile(t, filepath.Join(legacyRoot, "data", "settings.json"), "legacy settings")
	writeTestFile(t, filepath.Join(legacyRoot, "data", "Scripts", "custom.go"), "package main")
	writeTestFile(t, filepath.Join(legacyRoot, "data", "background.png"), "background")
	writeTestFile(t, filepath.Join(legacyRoot, "themes", "palettes", "Mine.json"), "theme")
	writeTestFile(t, filepath.Join(legacyRoot, "Text Logs", "Hero", "session.txt"), "chat")
	writeTestFile(t, filepath.Join(legacyRoot, "logs", "error.log"), "error")
	writeTestFile(t, filepath.Join(destination, "settings.json"), "new settings")

	migrated, err := migratePortableData(legacyRoot, destination)
	if err != nil {
		t.Fatalf("migratePortableData: %v", err)
	}
	if !migrated {
		t.Fatal("migratePortableData did not report a migration")
	}
	assertTestFile(t, filepath.Join(destination, "settings.json"), "new settings")
	assertTestFile(t, filepath.Join(destination, "Scripts", "custom.go"), "package main")
	assertTestFile(t, filepath.Join(destination, "background.png"), "background")
	assertTestFile(t, filepath.Join(destination, "themes", "palettes", "Mine.json"), "theme")
	assertTestFile(t, filepath.Join(destination, "Text Logs", "Hero", "session.txt"), "chat")
	assertTestFile(t, filepath.Join(destination, "logs", "error.log"), "error")
	assertTestFile(t, filepath.Join(legacyRoot, "data", "Scripts", "custom.go"), "package main")
	if _, err := os.Stat(filepath.Join(destination, legacyMigrationMarker)); err != nil {
		t.Fatalf("migration marker: %v", err)
	}

	writeTestFile(t, filepath.Join(legacyRoot, "data", "added-later.txt"), "old")
	migrated, err = migratePortableData(legacyRoot, destination)
	if err != nil {
		t.Fatalf("second migratePortableData: %v", err)
	}
	if migrated {
		t.Fatal("second migratePortableData unexpectedly migrated again")
	}
	if _, err := os.Stat(filepath.Join(destination, "added-later.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("second migration copied a new legacy file: %v", err)
	}
}

func TestMigratePortableDataIgnoresPackagedGuideOnly(t *testing.T) {
	legacyRoot := t.TempDir()
	destination := t.TempDir()
	writeTestFile(t, filepath.Join(legacyRoot, "data", "Macros", "Library", "README.md"), "guide")

	migrated, err := migratePortableData(legacyRoot, destination)
	if err != nil {
		t.Fatalf("migratePortableData: %v", err)
	}
	if migrated {
		t.Fatal("packaged guide triggered migration")
	}
	if _, err := os.Stat(filepath.Join(destination, legacyMigrationMarker)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("packaged guide created migration marker: %v", err)
	}
}

func TestMigratePortableDataWhenApplicationIsInDestination(t *testing.T) {
	destination := t.TempDir()
	writeTestFile(t, filepath.Join(destination, "data", "settings.json"), "legacy settings")

	migrated, err := migratePortableData(destination, destination)
	if err != nil {
		t.Fatalf("migratePortableData: %v", err)
	}
	if !migrated {
		t.Fatal("migratePortableData did not report a migration")
	}
	assertTestFile(t, filepath.Join(destination, "settings.json"), "legacy settings")
	assertTestFile(t, filepath.Join(destination, "data", "settings.json"), "legacy settings")
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertTestFile(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("%s = %q, want %q", path, data, want)
	}
}
