package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const (
	maxScriptAssetSize   = 16 << 20
	maxScriptPackageSize = 64 << 20
)

type scriptAssetSource struct {
	scriptsDir string
	container  string
	zipped     bool
}

type scriptPackageData struct {
	containerPath string
	sourcePath    string
	fallbackName  string
	source        []byte
	assets        *scriptAssetSource
	fingerprint   [sha256.Size]byte
	modTime       int64
	size          int64
	err           error
}

func validScriptRelativePath(name string) bool {
	return fs.ValidPath(name) && name != "." && !strings.Contains(name, `\`)
}

func scriptRootFileExists(dir, name string) bool {
	if !validScriptRelativePath(filepath.ToSlash(name)) {
		return false
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return false
	}
	defer root.Close()
	_, err = root.Stat(name)
	return err == nil
}

func scriptPackageFingerprint(container string, content [sha256.Size]byte) [sha256.Size]byte {
	hash := sha256.New()
	hash.Write([]byte(container))
	hash.Write([]byte{0})
	hash.Write(content[:])
	var result [sha256.Size]byte
	copy(result[:], hash.Sum(nil))
	return result
}

func (source *scriptAssetSource) read(name string) ([]byte, error) {
	if source == nil {
		return nil, fmt.Errorf("this loose script has no asset directory")
	}
	if !validScriptRelativePath(name) {
		return nil, fmt.Errorf("invalid asset path %q", name)
	}
	root, err := os.OpenRoot(source.scriptsDir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	if source.zipped {
		file, err := root.Open(source.container)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			return nil, err
		}
		archive, err := zip.NewReader(file, info.Size())
		if err != nil {
			return nil, err
		}
		return readScriptAssetFS(archive, name)
	}
	packageRoot, err := root.OpenRoot(source.container)
	if err != nil {
		return nil, err
	}
	defer packageRoot.Close()
	file, err := packageRoot.Open(filepath.FromSlash(name))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readScriptAssetFile(file, name)
}

func readScriptAssetFS(source fs.FS, name string) ([]byte, error) {
	file, err := source.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readScriptAssetFile(file, name)
}

func readScriptAssetFile(file fs.File, name string) ([]byte, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("asset %q is not a regular file", name)
	}
	if info.Size() > maxScriptAssetSize {
		return nil, fmt.Errorf("asset %q exceeds %d bytes", name, maxScriptAssetSize)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxScriptAssetSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxScriptAssetSize {
		return nil, fmt.Errorf("asset %q exceeds %d bytes", name, maxScriptAssetSize)
	}
	return data, nil
}

func discoverScriptPackages(dir string) []scriptPackageData {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil
	}
	defer root.Close()
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return nil
	}
	packages := make([]scriptPackageData, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || name == "gt2" {
			continue
		}
		switch {
		case entry.Type().IsRegular() && isUserScriptFile(name):
			if script, err := loadLooseScriptPackage(root, dir, name); err == nil {
				packages = append(packages, script)
			}
		case entry.IsDir():
			if script, ok := loadDirectoryScriptPackage(root, dir, name); ok {
				packages = append(packages, script)
			}
		case entry.Type().IsRegular() && strings.EqualFold(filepath.Ext(name), ".zip"):
			if script, ok := loadZipScriptPackage(root, dir, name); ok {
				packages = append(packages, script)
			}
		}
	}
	return packages
}

func loadLooseScriptPackage(root *os.Root, dir, name string) (scriptPackageData, error) {
	data, err := root.ReadFile(name)
	if err != nil {
		return scriptPackageData{}, err
	}
	info, err := root.Stat(name)
	if err != nil {
		return scriptPackageData{}, err
	}
	return scriptPackageData{
		containerPath: filepath.Join(dir, name),
		sourcePath:    filepath.Join(dir, name),
		fallbackName:  strings.TrimSuffix(name, filepath.Ext(name)),
		source:        data,
		fingerprint:   sha256.Sum256(data),
		modTime:       info.ModTime().UnixNano(),
		size:          info.Size(),
	}, nil
}

func loadDirectoryScriptPackage(root *os.Root, dir, name string) (scriptPackageData, bool) {
	packageRoot, err := root.OpenRoot(name)
	if err != nil {
		return scriptPackageData{}, false
	}
	defer packageRoot.Close()
	files, fingerprint, modTime, size, err := inspectScriptFS(packageRoot.FS())
	if len(files) == 0 {
		return scriptPackageData{}, false
	}
	result := scriptPackageData{
		containerPath: filepath.Join(dir, name),
		fallbackName:  name,
		fingerprint:   scriptPackageFingerprint(name, fingerprint),
		modTime:       modTime,
		size:          size,
		assets:        &scriptAssetSource{scriptsDir: dir, container: name},
		err:           err,
	}
	if err == nil {
		result.sourcePath = filepath.Join(result.containerPath, filepath.FromSlash(files[0]))
		result.source, result.err = packageRoot.ReadFile(filepath.FromSlash(files[0]))
	}
	return result, true
}

func loadZipScriptPackage(root *os.Root, dir, name string) (scriptPackageData, bool) {
	file, err := root.Open(name)
	if err != nil {
		return scriptPackageData{}, false
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return scriptPackageData{}, false
	}
	archive, err := zip.NewReader(file, info.Size())
	if err != nil {
		return scriptPackageData{
			containerPath: filepath.Join(dir, name), fallbackName: strings.TrimSuffix(name, filepath.Ext(name)),
			modTime: info.ModTime().UnixNano(), size: info.Size(), err: fmt.Errorf("invalid script ZIP: %w", err),
		}, true
	}
	for _, entry := range archive.File {
		entryName := strings.TrimSuffix(entry.Name, "/")
		if entryName == "" || !validScriptRelativePath(entryName) {
			return scriptPackageData{
				containerPath: filepath.Join(dir, name), fallbackName: strings.TrimSuffix(name, filepath.Ext(name)),
				modTime: info.ModTime().UnixNano(), size: info.Size(), err: fmt.Errorf("unsafe script package path %q", entry.Name),
			}, true
		}
	}
	files, fingerprint, _, _, inspectErr := inspectScriptFS(archive)
	if len(files) == 0 && inspectErr == nil {
		return scriptPackageData{}, false
	}
	result := scriptPackageData{
		containerPath: filepath.Join(dir, name),
		fallbackName:  strings.TrimSuffix(name, filepath.Ext(name)),
		fingerprint:   scriptPackageFingerprint(name, fingerprint),
		modTime:       info.ModTime().UnixNano(),
		size:          info.Size(),
		assets:        &scriptAssetSource{scriptsDir: dir, container: name, zipped: true},
		err:           inspectErr,
	}
	if inspectErr == nil {
		result.sourcePath = result.containerPath + string(filepath.Separator) + filepath.FromSlash(files[0])
		result.source, result.err = fs.ReadFile(archive, files[0])
	}
	return result, true
}

func inspectScriptFS(source fs.FS) (goFiles []string, fingerprint [sha256.Size]byte, modTime, totalSize int64, err error) {
	type fileData struct {
		name string
		data []byte
		info fs.FileInfo
	}
	var files []fileData
	err = fs.WalkDir(source, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name == "." {
			return nil
		}
		cleanName := strings.TrimSuffix(name, "/")
		if !validScriptRelativePath(cleanName) {
			return fmt.Errorf("unsafe script package path %q", name)
		}
		if entry.IsDir() {
			return nil
		}
		info, statErr := entry.Info()
		if statErr != nil {
			return statErr
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("script package entry %q is not a regular file", name)
		}
		if info.Size() > maxScriptAssetSize || totalSize+info.Size() > maxScriptPackageSize {
			return fmt.Errorf("script package entry %q is too large", name)
		}
		totalSize += info.Size()
		data, readErr := fs.ReadFile(source, name)
		if readErr != nil {
			return readErr
		}
		files = append(files, fileData{name: name, data: data, info: info})
		if strings.EqualFold(path.Ext(name), ".go") {
			goFiles = append(goFiles, name)
		}
		return nil
	})
	if err != nil {
		return goFiles, fingerprint, 0, 0, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	hash := sha256.New()
	var number [8]byte
	for _, file := range files {
		hash.Write([]byte(file.name))
		hash.Write([]byte{0})
		binary.LittleEndian.PutUint64(number[:], uint64(file.info.ModTime().UnixNano()))
		hash.Write(number[:])
		hash.Write(file.data)
		if changed := file.info.ModTime().UnixNano(); changed > modTime {
			modTime = changed
		}
	}
	copy(fingerprint[:], hash.Sum(nil))
	if len(goFiles) != 1 {
		return goFiles, fingerprint, modTime, totalSize, fmt.Errorf("script package must contain exactly one Go file; found %d", len(goFiles))
	}
	if strings.Contains(goFiles[0], "/") {
		return goFiles, fingerprint, modTime, totalSize, fmt.Errorf("script Go file must be at the package root")
	}
	return goFiles, fingerprint, modTime, totalSize, nil
}
