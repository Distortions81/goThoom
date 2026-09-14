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

func TestInitializeUserDataCreatesRoot(t *testing.T) {
	original := dataDirPath
	dataDirPath = filepath.Join(t.TempDir(), "user-data")
	t.Cleanup(func() { dataDirPath = original })

	if err := initializeUserData(); err != nil {
		t.Fatalf("initializeUserData: %v", err)
	}
	info, err := os.Stat(dataDirPath)
	if err != nil {
		t.Fatalf("stat user data root: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("user data root is not a directory: %s", dataDirPath)
	}
}
