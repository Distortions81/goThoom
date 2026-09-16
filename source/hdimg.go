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
	"sort"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

var (
	hdPictureSources, hdPicturePacks, hdPicturePackLocation = externalHDPictureCatalog()
	hdPictureCache                                          = make(map[uint16]*ebiten.Image)
)

type hdPictureSource struct {
	files    fs.FS
	label    string
	filePath string
	zipEntry *zip.File
	priority int
}

type hdPicturePack struct {
	key     string
	label   string
	loose   bool
	sources map[uint16]hdPictureSource
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
	sources, _ := indexHDPictureCatalogInFolder(files, folder, folder)
	return sources
}

func indexHDPictureCatalogInFolder(files fs.FS, folder, origin string) (map[uint16]hdPictureSource, []hdPicturePack) {
	sources := make(map[uint16]hdPictureSource)
	if files == nil {
		return sources, nil
	}
	add := func(collection map[uint16]hdPictureSource, id uint16, candidate hdPictureSource) {
		current, exists := collection[id]
		if !exists || candidate.priority < current.priority || candidate.priority == current.priority && candidate.label < current.label {
			collection[id] = candidate
		}
	}
	loose := hdPicturePack{key: origin + ":loose", label: "Loose files", loose: true, sources: make(map[uint16]hdPictureSource)}
	archives := make([]hdPicturePack, 0)
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
			candidate := hdPictureSource{files: files, label: filePath, filePath: filePath, priority: priority}
			add(sources, id, candidate)
			add(loose.sources, id, candidate)
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
		pack := hdPicturePack{
			key:     origin + ":" + filePath,
			label:   hdPictureArchivePackLabel(filePath, folder, origin),
			sources: make(map[uint16]hdPictureSource),
		}
		for _, zipped := range archive.File {
			if zipped.FileInfo().IsDir() {
				continue
			}
			if id, ok := hdPictureIDFromFilename(zipped.Name); ok {
				candidate := hdPictureSource{files: files, label: filePath + "!" + zipped.Name, filePath: filePath, zipEntry: zipped, priority: 2}
				add(sources, id, candidate)
				add(pack.sources, id, candidate)
			}
		}
		if len(pack.sources) > 0 {
			archives = append(archives, pack)
		}
		return nil
	})
	if err != nil {
		log.Printf("HD image folder: %v", err)
	}
	sort.Slice(archives, func(i, j int) bool {
		if archives[i].label == archives[j].label {
			return archives[i].key < archives[j].key
		}
		return archives[i].label < archives[j].label
	})
	packs := make([]hdPicturePack, 0, len(archives)+1)
	if len(loose.sources) > 0 {
		packs = append(packs, loose)
	}
	packs = append(packs, archives...)
	return sources, packs
}

func hdPictureArchivePackLabel(filePath, folder, origin string) string {
	relative := strings.TrimPrefix(filePath, strings.TrimSuffix(folder, "/")+"/")
	return relative + " (" + origin + ")"
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
	if !gs.UseSpritePackFiles || !hdPictureEnabled(id) {
		return false
	}
	return hdPictureCompatibleLocked(id)
}

func hdPictureCompatibleLocked(id uint16) bool {
	if _, ok := hdPictureSources[id]; !ok {
		return false
	}
	return clImages == nil || clImages.NumFrames(uint32(id)) == 1
}

func hdPictureCompatible(id uint16) bool {
	imageCacheLifecycleMu.RLock()
	defer imageCacheLifecycleMu.RUnlock()
	return hdPictureCompatibleLocked(id)
}

func hdPictureSourceLabel(id uint16) string {
	imageCacheLifecycleMu.RLock()
	defer imageCacheLifecycleMu.RUnlock()
	return hdPictureSources[id].label
}

func hdPictureEnabled(id uint16) bool {
	for _, disabledID := range gs.DisabledHDPictures {
		if disabledID == id {
			return false
		}
	}
	return true
}

func setHDPictureEnabled(id uint16, enabled bool) {
	disabled := make([]uint16, 0, len(gs.DisabledHDPictures)+1)
	found := false
	for _, disabledID := range gs.DisabledHDPictures {
		if disabledID == id {
			found = true
			if !enabled {
				disabled = append(disabled, disabledID)
			}
			continue
		}
		disabled = append(disabled, disabledID)
	}
	if !enabled && !found {
		disabled = append(disabled, id)
	}
	sort.Slice(disabled, func(i, j int) bool { return disabled[i] < disabled[j] })
	gs.DisabledHDPictures = disabled
}

func loadHDPicture(id uint16) *ebiten.Image {
	imageCacheLifecycleMu.RLock()
	defer imageCacheLifecycleMu.RUnlock()
	if !hdPictureAvailableLocked(id) {
		return nil
	}
	return loadHDPictureSourceLocked(id)
}

// loadHDPicturePreview loads a compatible replacement even when that picture
// is unchecked. The gallery remains useful while choosing which replacements
// the live game should use.
func loadHDPicturePreview(id uint16) *ebiten.Image {
	imageCacheLifecycleMu.RLock()
	defer imageCacheLifecycleMu.RUnlock()
	if !hdPictureCompatibleLocked(id) {
		return nil
	}
	return loadHDPictureSourceLocked(id)
}

func loadHDPictureSourceLocked(id uint16) *ebiten.Image {
	imageMu.Lock()
	if img, cached := hdPictureCache[id]; cached {
		imageMu.Unlock()
		return img
	}
	imageMu.Unlock()

	source := hdPictureSources[id]
	img, err := decodeHDPictureSource(source)
	if err != nil {
		log.Printf("HD picture %d (%s): %v", id, source.label, err)
		cacheMissingHDPicture(id)
		return nil
	}
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

func decodeHDPictureSource(source hdPictureSource) (*ebiten.Image, error) {
	reader, err := source.open()
	if err != nil {
		return nil, err
	}
	decoded, err := png.Decode(reader)
	reader.Close()
	if err != nil {
		return nil, err
	}
	return ebiten.NewImageFromImage(decoded), nil
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
	sources, packs, location := externalHDPictureCatalog()
	imageCacheLifecycleMu.Lock()
	imageMu.Lock()
	for _, img := range hdPictureCache {
		deallocateImage(img)
	}
	hdPictureCache = make(map[uint16]*ebiten.Image)
	hdPictureSources = sources
	hdPicturePacks = packs
	hdPicturePackLocation = location
	imageMu.Unlock()
	imageCacheLifecycleMu.Unlock()
	toolbarHandsRendered = false
	inventoryDirty = true
	playersDirty = true
	return location, len(sources)
}

func externalHDPictureCatalog() (map[uint16]hdPictureSource, []hdPicturePack, string) {
	gameFiles, gameLocation := gameHDPictureFiles()
	sources, gamePacks := indexHDPictureCatalogInFolder(gameFiles, "data/hdimg", "game folder")
	packs := make([]hdPicturePack, 0, len(gamePacks)+1)
	loose := hdPicturePack{key: "loose", label: "Loose files", loose: true, sources: make(map[uint16]hdPictureSource)}
	mergePacks := func(indexed []hdPicturePack) {
		for _, pack := range indexed {
			if !pack.loose {
				packs = append(packs, pack)
				continue
			}
			for id, source := range pack.sources {
				if _, exists := loose.sources[id]; !exists {
					loose.sources[id] = source
				}
			}
		}
	}
	mergePacks(gamePacks)
	locations := make([]string, 0, 2)
	if gameFiles != nil {
		locations = append(locations, gameLocation)
	}

	userFolder := filepath.Join(dataDirPath, "hdimg")
	if info, err := os.Stat(userFolder); err == nil && info.IsDir() {
		userSources, userPacks := indexHDPictureCatalogInFolder(os.DirFS(dataDirPath), "hdimg", "user folder")
		for id, source := range userSources {
			if _, gamePackHasID := sources[id]; !gamePackHasID {
				sources[id] = source
			}
		}
		mergePacks(userPacks)
		locations = append(locations, userFolder)
	}
	sortFrom := 0
	if len(loose.sources) > 0 {
		packs = append([]hdPicturePack{loose}, packs...)
		sortFrom = 1
	}
	if len(packs)-sortFrom > 1 {
		sort.Slice(packs[sortFrom:], func(i, j int) bool {
			return packs[sortFrom+i].label < packs[sortFrom+j].label
		})
	}
	if len(locations) == 0 {
		return sources, packs, "data/hdimg or user-data hdimg (not found)"
	}
	return sources, packs, strings.Join(locations, "; ")
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
			root, err := filepath.Abs(candidate.root)
			if err != nil {
				continue
			}
			return os.DirFS(root), candidate.folder
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
