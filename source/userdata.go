package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const userDataDirectoryName = "goThoom"

// dataDirPath is the root for all persistent application data.
var dataDirPath = platformDataDir(
	runtime.GOOS,
	runtime.GOARCH,
	os.Getenv,
	os.UserHomeDir,
	os.Executable,
)

func platformDataDir(goos, goarch string, getenv func(string) string, userHomeDir, executable func() (string, error)) string {
	if goos == "js" || goarch == "wasm" {
		return "data"
	}
	home, _ := userHomeDir()
	switch goos {
	case "windows":
		if local := strings.TrimSpace(getenv("LOCALAPPDATA")); filepath.IsAbs(local) {
			return filepath.Join(local, userDataDirectoryName)
		}
		if filepath.IsAbs(home) {
			return filepath.Join(home, "AppData", "Local", userDataDirectoryName)
		}
	case "linux":
		if xdg := strings.TrimSpace(getenv("XDG_DATA_HOME")); filepath.IsAbs(xdg) {
			return filepath.Join(xdg, userDataDirectoryName)
		}
		if filepath.IsAbs(home) {
			return filepath.Join(home, ".local", "share", userDataDirectoryName)
		}
	case "darwin":
		if filepath.IsAbs(home) {
			if filepath.Base(home) == "Data" && filepath.Base(filepath.Dir(home)) == "com.goThoom.client" {
				home = filepath.Dir(home)
			} else {
				home = filepath.Join(home, "Library", "Containers", "com.goThoom.client")
			}
			return home
		}
	}
	if exe, err := executable(); err == nil {
		if dir, err := filepath.Abs(filepath.Dir(exe)); err == nil {
			return filepath.Join(dir, "data")
		}
	}
	return "data"
}

// initializeUserData creates the platform data root. Existing application-adjacent
// files are left alone; users can copy selected files into this directory when
// needed.
func initializeUserData() error {
	if isWASM {
		return nil
	}
	if err := os.MkdirAll(dataDirPath, 0o755); err != nil {
		return fmt.Errorf("create user data directory %q: %w", dataDirPath, err)
	}
	return nil
}
