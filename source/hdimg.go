package main

import (
	"archive/zip"
	"bytes"
	"image/png"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

var (
	hdPictureSources, hdPicturePackLocation = externalHDPictureSources()
	hdPictureCache                          = make(map[uint16]*ebiten.Image)
)

type hdPictureSource struct {
	files    fs.FS
	label    string
	filePath string
	zipEntry *zip.File
	priority int
}

func (source hdPictureSource) open() (io.ReadCloser, error) {
	if source.zipEntry != nil {
		return source.zipEntry.Open()
	}
	return source.files.Open(source.filePath)
}

func indexHDPictureSources(files fs.FS) map[uint16]hdPictureSource {
	return indexHDPictureSourcesInFolder(files, "data/hdimg")
}

func indexHDPictureSourcesInFolder(files fs.FS, folder string) map[uint16]hdPictureSource {
	sources := make(map[uint16]hdPictureSource)
	if files == nil {
		return sources
	}
	add := func(id uint16, candidate hdPictureSource) {
		current, exists := sources[id]
		if !exists || candidate.priority < current.priority || candidate.priority == current.priority && candidate.label < current.label {
			sources[id] = candidate
		}
	}
	err := fs.WalkDir(files, folder, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if id, ok := hdPictureIDFromFilename(filePath); ok {
			priority := 1
			if path.Dir(filePath) == folder {
				priority = 0
			}
			add(id, hdPictureSource{files: files, label: filePath, filePath: filePath, priority: priority})
			return nil
		}
		if !strings.EqualFold(path.Ext(filePath), ".zip") {
			return nil
		}
		data, err := fs.ReadFile(files, filePath)
		if err != nil {
			log.Printf("HD image bundle %s: %v", filePath, err)
			return nil
		}
		archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			log.Printf("HD image bundle %s: %v", filePath, err)
			return nil
		}
		for _, zipped := range archive.File {
			if zipped.FileInfo().IsDir() {
				continue
			}
			if id, ok := hdPictureIDFromFilename(zipped.Name); ok {
				add(id, hdPictureSource{files: files, label: filePath + "!" + zipped.Name, filePath: filePath, zipEntry: zipped, priority: 2})
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("HD image folder: %v", err)
	}
	return sources
}

func hdPictureIDFromFilename(name string) (uint16, bool) {
	base := path.Base(name)
	if !strings.EqualFold(path.Ext(base), ".png") {
		return 0, false
	}
	id, err := strconv.ParseUint(base[:len(base)-4], 10, 16)
	return uint16(id), err == nil
}

func hdPictureAvailable(id uint16) bool {
	imageCacheLifecycleMu.RLock()
	defer imageCacheLifecycleMu.RUnlock()
	return hdPictureAvailableLocked(id)
}

func hdPictureAvailableLocked(id uint16) bool {
	if !gs.UseSpritePackFiles {
		return false
	}
	if _, ok := hdPictureSources[id]; !ok {
		return false
	}
	return clImages == nil || clImages.NumFrames(uint32(id)) == 1
}

func loadHDPicture(id uint16) *ebiten.Image {
	imageCacheLifecycleMu.RLock()
	defer imageCacheLifecycleMu.RUnlock()
	if !hdPictureAvailableLocked(id) {
		return nil
	}
	imageMu.Lock()
	if img, cached := hdPictureCache[id]; cached {
		imageMu.Unlock()
		return img
	}
	imageMu.Unlock()

	source := hdPictureSources[id]
	reader, err := source.open()
	if err != nil {
		log.Printf("HD picture %d (%s): %v", id, source.label, err)
		cacheMissingHDPicture(id)
		return nil
	}
	decoded, err := png.Decode(reader)
	reader.Close()
	if err != nil {
		log.Printf("HD picture %d (%s): %v", id, source.label, err)
		cacheMissingHDPicture(id)
		return nil
	}
	img := ebiten.NewImageFromImage(decoded)
	imageMu.Lock()
	if cached, exists := hdPictureCache[id]; exists {
		imageMu.Unlock()
		deallocateImage(img)
		return cached
	}
	hdPictureCache[id] = img
	imageMu.Unlock()
	return img
}

func cacheMissingHDPicture(id uint16) {
	imageMu.Lock()
	if _, cached := hdPictureCache[id]; !cached {
		hdPictureCache[id] = nil
	}
	imageMu.Unlock()
}

func isHDPictureImage(id uint16, img *ebiten.Image) bool {
	if img == nil {
		return false
	}
	imageCacheLifecycleMu.RLock()
	defer imageCacheLifecycleMu.RUnlock()
	if !hdPictureAvailableLocked(id) {
		return false
	}
	imageMu.Lock()
	hd := hdPictureCache[id]
	imageMu.Unlock()
	return hd == img
}

// reloadHDPictures rescans external sprite packs in the game and user-data
// directories. The game pack wins when both provide the same sprite ID.
func reloadHDPictures() (string, int) {
	sources, location := externalHDPictureSources()
	imageCacheLifecycleMu.Lock()
	imageMu.Lock()
	for _, img := range hdPictureCache {
		deallocateImage(img)
	}
	hdPictureCache = make(map[uint16]*ebiten.Image)
	hdPictureSources = sources
	hdPicturePackLocation = location
	imageMu.Unlock()
	imageCacheLifecycleMu.Unlock()
	toolbarHandsRendered = false
	inventoryDirty = true
	playersDirty = true
	return location, len(sources)
}

func externalHDPictureSources() (map[uint16]hdPictureSource, string) {
	gameFiles, gameLocation := gameHDPictureFiles()
	sources := indexHDPictureSources(gameFiles)
	locations := make([]string, 0, 2)
	if gameFiles != nil {
		locations = append(locations, gameLocation)
	}

	userFolder := filepath.Join(dataDirPath, "hdimg")
	if info, err := os.Stat(userFolder); err == nil && info.IsDir() {
		for id, source := range indexHDPictureSourcesInFolder(os.DirFS(dataDirPath), "hdimg") {
			if _, gamePackHasID := sources[id]; !gamePackHasID {
				sources[id] = source
			}
		}
		locations = append(locations, userFolder)
	}
	if len(locations) == 0 {
		return sources, "data/hdimg or user-data hdimg (not found)"
	}
	return sources, strings.Join(locations, "; ")
}

func gameHDPictureFiles() (fs.FS, string) {
	// The working directory is the application directory in a normal launch.
	// In a go run session it is a temporary build directory, so fall back to
	// the checked-out source tree for development.
	for _, candidate := range []struct {
		root, folder string
	}{
		{root: ".", folder: "data/hdimg"},
		{root: "source", folder: "source/data/hdimg"},
	} {
		if info, err := os.Stat(candidate.folder); err == nil && info.IsDir() {
			return os.DirFS(candidate.root), candidate.folder
		}
	}
	if _, sourceFile, _, ok := runtime.Caller(0); ok {
		sourceDir := filepath.Dir(sourceFile)
		folder := filepath.Join(sourceDir, "data", "hdimg")
		if info, err := os.Stat(folder); err == nil && info.IsDir() {
			return os.DirFS(sourceDir), folder
		}
	}
	return nil, "data/hdimg (not found)"
}

func hdPictureDrawScale(id uint16, img *ebiten.Image) (float64, float64) {
	if !isHDPictureImage(id, img) || clImages == nil {
		return 1, 1
	}
	w, h := clImages.Size(uint32(id))
	return pictureSourceScale(w, h, img.Bounds().Dx(), img.Bounds().Dy())
}

func pictureSourceScale(nativeWidth, nativeHeight, sourceWidth, sourceHeight int) (float64, float64) {
	if nativeWidth <= 0 || nativeHeight <= 0 || sourceWidth <= 0 || sourceHeight <= 0 {
		return 1, 1
	}
	return float64(nativeWidth) / float64(sourceWidth), float64(nativeHeight) / float64(sourceHeight)
}
