package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	userDataDirectoryName = "goThoom"
	legacyMigrationMarker = ".portable-data-migrated"
)

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

func executableDirectory() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Abs(filepath.Dir(exe))
}

// initializeUserData creates the platform data root and copies an existing
// portable installation into it once. The old files are intentionally left in
// place so migration is recoverable.
func initializeUserData() (bool, error) {
	if isWASM {
		return false, nil
	}
	if err := os.MkdirAll(dataDirPath, 0o755); err != nil {
		return false, fmt.Errorf("create user data directory %q: %w", dataDirPath, err)
	}
	// macOS already stored writable data in its application container. Only
	// Windows and Linux need migration from executable-adjacent directories.
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		return false, nil
	}
	legacyRoot, err := executableDirectory()
	if err != nil {
		return false, fmt.Errorf("find portable data directory: %w", err)
	}
	return migratePortableData(legacyRoot, dataDirPath)
}

func migratePortableData(legacyRoot, destination string) (bool, error) {
	legacyRoot, err := filepath.Abs(legacyRoot)
	if err != nil {
		return false, err
	}
	destination, err = filepath.Abs(destination)
	if err != nil {
		return false, err
	}
	if filepath.Join(legacyRoot, "data") == destination {
		return false, nil
	}
	marker := filepath.Join(destination, legacyMigrationMarker)
	if _, err := os.Stat(marker); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}

	meaningful, err := hasPortableUserData(legacyRoot)
	if err != nil || !meaningful {
		return false, err
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return false, err
	}
	for _, entry := range []struct {
		source      string
		destination string
	}{
		{filepath.Join(legacyRoot, "data"), destination},
		{filepath.Join(legacyRoot, "themes"), filepath.Join(destination, "themes")},
		{filepath.Join(legacyRoot, "Text Logs"), filepath.Join(destination, "Text Logs")},
		{filepath.Join(legacyRoot, "logs"), filepath.Join(destination, "logs")},
	} {
		if err := copyMissingTree(entry.source, entry.destination); err != nil {
			return false, err
		}
	}
	message := fmt.Sprintf("Copied from %s; the original files were left in place.\n", legacyRoot)
	if err := os.WriteFile(marker, []byte(message), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func hasPortableUserData(legacyRoot string) (bool, error) {
	for _, dir := range []string{"data", "themes", "Text Logs", "logs"} {
		root := filepath.Join(legacyRoot, dir)
		found := false
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if errors.Is(err, os.ErrNotExist) {
				return fs.SkipDir
			}
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			relative, err := filepath.Rel(legacyRoot, path)
			if err != nil {
				return err
			}
			if filepath.ToSlash(relative) == "data/Macros/Library/README.md" {
				return nil
			}
			found = true
			return fs.SkipAll
		})
		if err != nil {
			return false, err
		}
		if found {
			return true, nil
		}
	}
	return false, nil
}

func copyMissingTree(source, destination string) error {
	info, err := os.Stat(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("legacy path %q is not a directory", source)
	}
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		return copyFileIfMissing(path, target, info.Mode().Perm())
	})
}

func copyFileIfMissing(source, destination string, mode os.FileMode) error {
	if _, err := os.Stat(destination); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	removePartial := true
	defer func() {
		_ = out.Close()
		if removePartial {
			_ = os.Remove(destination)
		}
	}()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	removePartial = false
	return nil
}
