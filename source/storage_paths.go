package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type storagePathKind int

const (
	storagePathAssets storagePathKind = iota
	storagePathLogs
	storagePathMacros
	storagePathScripts
)

var storagePathKinds = []storagePathKind{
	storagePathAssets,
	storagePathLogs,
	storagePathMacros,
	storagePathScripts,
}

type storagePathValues struct {
	assets  string
	logs    string
	macros  string
	scripts string
}

var (
	activeStoragePaths    storagePathValues
	storagePathsActivated bool
)

func storagePathName(kind storagePathKind) string {
	switch kind {
	case storagePathAssets:
		return "Assets & Audio"
	case storagePathLogs:
		return "Logs"
	case storagePathMacros:
		return "Legacy Macros"
	case storagePathScripts:
		return "Go Scripts"
	default:
		return "Files"
	}
}

func defaultStoragePath(kind storagePathKind) string {
	switch kind {
	case storagePathMacros:
		return filepath.Join(dataDirPath, legacyMacrosDirName)
	case storagePathScripts:
		return filepath.Join(dataDirPath, "Scripts")
	}
	return dataDirPath
}

func configuredStoragePath(value settings, kind storagePathKind) string {
	var configured string
	switch kind {
	case storagePathAssets:
		configured = value.AssetsPath
	case storagePathLogs:
		configured = value.LogsPath
	case storagePathMacros:
		configured = value.MacrosPath
	case storagePathScripts:
		configured = value.ScriptsPath
	}
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return defaultStoragePath(kind)
	}
	return filepath.Clean(configured)
}

func storagePathsFromSettings(value settings) storagePathValues {
	return storagePathValues{
		assets:  configuredStoragePath(value, storagePathAssets),
		logs:    configuredStoragePath(value, storagePathLogs),
		macros:  configuredStoragePath(value, storagePathMacros),
		scripts: configuredStoragePath(value, storagePathScripts),
	}
}

func activateStoragePaths() {
	activeStoragePaths = storagePathsFromSettings(gs)
	storagePathsActivated = true
}

func activeStoragePath(kind storagePathKind) string {
	if !storagePathsActivated {
		return configuredStoragePath(gs, kind)
	}
	switch kind {
	case storagePathAssets:
		return activeStoragePaths.assets
	case storagePathLogs:
		return activeStoragePaths.logs
	case storagePathMacros:
		return activeStoragePaths.macros
	case storagePathScripts:
		return activeStoragePaths.scripts
	default:
		return dataDirPath
	}
}

func assetsDirPath() string {
	return activeStoragePath(storagePathAssets)
}

func assetFilePath(name string) string {
	return filepath.Join(assetsDirPath(), name)
}

func soundFontsDirPath() string {
	return assetsDirPath()
}

func soundFontPath() string {
	return filepath.Join(soundFontsDirPath(), soundFontFile)
}

func ttsDataDirPath() string {
	return assetsDirPath()
}

func piperDirPath() string {
	return filepath.Join(ttsDataDirPath(), "piper")
}

func logsDirPath() string {
	return activeStoragePath(storagePathLogs)
}

func textLogsDirPath() string {
	return filepath.Join(logsDirPath(), "Text Logs")
}

func macrosDirPath() string {
	return activeStoragePath(storagePathMacros)
}

func scriptsDirPath() string {
	return activeStoragePath(storagePathScripts)
}

func normalizeStoragePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(absolute), nil
}

func verifyStorageDirectory(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", path)
	}
	if _, err := os.ReadDir(path); err != nil {
		return fmt.Errorf("read directory: %w", err)
	}

	probe, err := os.CreateTemp(path, ".gothoom-path-test-*.tmp")
	if err != nil {
		return fmt.Errorf("create test file: %w", err)
	}
	probePath := probe.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(probePath)
		}
	}()
	want := []byte("goThoom path read/write test\n")
	if _, err := probe.Write(want); err != nil {
		_ = probe.Close()
		return fmt.Errorf("write test file: %w", err)
	}
	if err := probe.Sync(); err != nil {
		_ = probe.Close()
		return fmt.Errorf("sync test file: %w", err)
	}
	if err := probe.Close(); err != nil {
		return fmt.Errorf("close test file: %w", err)
	}
	got, err := os.ReadFile(probePath)
	if err != nil {
		return fmt.Errorf("read test file: %w", err)
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("test file contents did not match after reading")
	}
	if err := os.Remove(probePath); err != nil {
		return fmt.Errorf("remove test file: %w", err)
	}
	cleanup = false
	return nil
}

func setStoragePathSetting(value *settings, kind storagePathKind, path string) {
	switch kind {
	case storagePathAssets:
		value.AssetsPath = path
	case storagePathLogs:
		value.LogsPath = path
	case storagePathMacros:
		value.MacrosPath = path
	case storagePathScripts:
		value.ScriptsPath = path
	}
}

func storagePathKindForSetting(entry settingsSchemaEntry) (storagePathKind, bool) {
	switch entry.field {
	case "AssetsPath":
		return storagePathAssets, true
	case "LogsPath":
		return storagePathLogs, true
	case "MacrosPath":
		return storagePathMacros, true
	case "ScriptsPath":
		return storagePathScripts, true
	default:
		return 0, false
	}
}

// changeStoragePath validates and optionally populates a directory before it
// commits the setting. The running client keeps its startup paths until the
// next launch so open logs and loaded assets never switch underneath it.
func changeStoragePath(kind storagePathKind, configuredPath string, copyFiles bool) (string, error) {
	configuredPath, err := normalizeStoragePath(configuredPath)
	if err != nil {
		return "", err
	}
	destination := configuredPath
	if destination == "" {
		destination = defaultStoragePath(kind)
	}
	if err := verifyStorageDirectory(destination); err != nil {
		return "", fmt.Errorf("verify %s path %q: %w", storagePathName(kind), destination, err)
	}
	if copyFiles {
		if err := copyStoragePathFiles(kind, activeStoragePath(kind), destination); err != nil {
			return "", fmt.Errorf("copy %s files: %w", storagePathName(kind), err)
		}
	}
	if err := verifyStorageDirectory(destination); err != nil {
		return "", fmt.Errorf("recheck %s path %q: %w", storagePathName(kind), destination, err)
	}

	candidate := settingsForSave()
	setStoragePathSetting(&candidate, kind, configuredPath)
	if err := writeSettingsFile(candidate); err != nil {
		return "", fmt.Errorf("save %s path: %w", storagePathName(kind), err)
	}
	setStoragePathSetting(&gs, kind, configuredPath)
	if globalSettingsBaseReady {
		setStoragePathSetting(&globalSettingsBase, kind, configuredPath)
	}
	settingsDirty = false
	return destination, nil
}

func copyStoragePathFiles(kind storagePathKind, sourceRoot, destinationRoot string) error {
	if sameStoragePath(sourceRoot, destinationRoot) {
		return nil
	}
	type copyRoot struct {
		source      string
		destination string
	}
	var roots []copyRoot
	add := func(relative string) {
		roots = append(roots, copyRoot{
			source:      filepath.Join(sourceRoot, relative),
			destination: filepath.Join(destinationRoot, relative),
		})
	}
	switch kind {
	case storagePathAssets:
		add(CL_ImagesFile)
		add(CL_SoundsFile)
		add(soundFontFile)
		add("piper")
		add(ttsSubstituteFile)
	case storagePathLogs:
		add(diagnosticsDirectoryName)
		add("Text Logs")
	case storagePathMacros:
		roots = append(roots, copyRoot{source: sourceRoot, destination: destinationRoot})
	case storagePathScripts:
		roots = append(roots, copyRoot{source: sourceRoot, destination: destinationRoot})
	default:
		return fmt.Errorf("unknown storage path")
	}

	var plan []storageCopyFile
	for _, root := range roots {
		files, err := planStorageCopy(root.source, root.destination)
		if err != nil {
			return err
		}
		plan = append(plan, files...)
	}
	for _, file := range plan {
		if err := copyStorageFile(file); err != nil {
			return err
		}
	}
	return nil
}

type storageCopyFile struct {
	source      string
	destination string
	mode        fs.FileMode
}

func planStorageCopy(source, destination string) ([]storageCopyFile, error) {
	info, err := os.Lstat(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("symbolic link %q cannot be copied", source)
	}
	if !info.IsDir() {
		return planStorageFile(source, destination, info)
	}
	if storagePathInside(destination, source) {
		return nil, fmt.Errorf("destination %q is inside source %q", destination, source)
	}
	if destinationInfo, err := os.Stat(destination); err == nil && !destinationInfo.IsDir() {
		return nil, fmt.Errorf("destination %q is not a directory", destination)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	var plan []storageCopyFile
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == source {
			return nil
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic link %q cannot be copied", path)
		}
		if info.IsDir() {
			if targetInfo, err := os.Stat(target); err == nil && !targetInfo.IsDir() {
				return fmt.Errorf("destination %q is not a directory", target)
			} else if err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported file type %q", path)
		}
		files, err := planStorageFile(path, target, info)
		if err != nil {
			return err
		}
		plan = append(plan, files...)
		return nil
	})
	return plan, err
}

func planStorageFile(source, destination string, info fs.FileInfo) ([]storageCopyFile, error) {
	sourceHash, err := storageFileHash(source)
	if err != nil {
		return nil, err
	}
	destinationInfo, err := os.Stat(destination)
	if err == nil {
		if !destinationInfo.Mode().IsRegular() {
			return nil, fmt.Errorf("destination %q is not a regular file", destination)
		}
		destinationHash, err := storageFileHash(destination)
		if err != nil {
			return nil, err
		}
		if sourceHash != destinationHash {
			return nil, fmt.Errorf("destination file %q already exists with different contents", destination)
		}
		return nil, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return []storageCopyFile{{
		source:      source,
		destination: destination,
		mode:        info.Mode().Perm(),
	}}, nil
}

func copyStorageFile(file storageCopyFile) error {
	if err := os.MkdirAll(filepath.Dir(file.destination), 0o755); err != nil {
		return err
	}
	source, err := os.Open(file.source)
	if err != nil {
		return err
	}
	defer source.Close()
	destination, err := os.OpenFile(file.destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, file.mode)
	if err != nil {
		return err
	}
	removeDestination := true
	defer func() {
		_ = destination.Close()
		if removeDestination {
			_ = os.Remove(file.destination)
		}
	}()
	copied := sha256.New()
	if _, err := io.Copy(io.MultiWriter(destination, copied), source); err != nil {
		return err
	}
	if err := destination.Sync(); err != nil {
		return err
	}
	if err := destination.Close(); err != nil {
		return err
	}
	destinationHash, err := storageFileHash(file.destination)
	if err != nil {
		return err
	}
	var copiedHash [sha256.Size]byte
	copy(copiedHash[:], copied.Sum(nil))
	if destinationHash != copiedHash {
		return fmt.Errorf("checksum verification failed for %q", file.destination)
	}
	removeDestination = false
	return nil
}

func storageFileHash(path string) ([sha256.Size]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return [sha256.Size]byte{}, err
	}
	var result [sha256.Size]byte
	copy(result[:], hash.Sum(nil))
	return result, nil
}

func sameStoragePath(first, second string) bool {
	firstInfo, firstErr := os.Stat(first)
	secondInfo, secondErr := os.Stat(second)
	if firstErr == nil && secondErr == nil && os.SameFile(firstInfo, secondInfo) {
		return true
	}
	firstAbs, firstErr := filepath.Abs(first)
	secondAbs, secondErr := filepath.Abs(second)
	return firstErr == nil && secondErr == nil && filepath.Clean(firstAbs) == filepath.Clean(secondAbs)
}

func storagePathInside(path, directory string) bool {
	path, pathErr := filepath.Abs(path)
	directory, directoryErr := filepath.Abs(directory)
	if pathErr != nil || directoryErr != nil {
		return false
	}
	relative, err := filepath.Rel(directory, path)
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
