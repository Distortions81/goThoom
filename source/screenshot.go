package main

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/hajimehoshi/ebiten/v2"
)

type snapshotRequest struct {
	name         string
	fullWindow   bool
	hideNameTags bool
	jpeg         bool
	done         func(error)
}

var pendingSnapshot *snapshotRequest

func snapshotHidesNameTags() bool {
	return pendingSnapshot != nil && pendingSnapshot.hideNameTags
}

func defaultSnapshotName() string {
	prefix := "clanlord"
	if clmov != "" {
		prefix = strings.TrimSuffix(strings.ToLower(path.Base(clmov)), ".clmov")
		if len(prefix) > 16 {
			prefix = prefix[:16]
		}
	}
	if gs.LastCharacter != "" {
		prefix = gs.LastCharacter
	}
	if name := strings.TrimSpace(playerName); name != "" {
		prefix = name
	}
	prefix = strings.Map(func(r rune) rune {
		if invalidSnapshotNameRune(r) {
			return '-'
		}
		return r
	}, prefix)
	return prefix + "__" + time.Now().Format("2006-01-02-15-04-05")
}

func invalidSnapshotNameRune(r rune) bool {
	return unicode.IsControl(r) || strings.ContainsRune(`<>:"/\|?*`, r)
}

func snapshotFilename(name string, asJPEG bool) (string, error) {
	name = strings.TrimSpace(name)
	ext := strings.ToLower(filepath.Ext(name))
	if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
		name = name[:len(name)-len(ext)]
	}
	if name == "" || name == "." || name == ".." || strings.HasSuffix(name, ".") {
		return "", fmt.Errorf("Enter a filename without a trailing dot.")
	}
	if len(name) > 180 {
		return "", fmt.Errorf("Use a shorter filename (up to 180 bytes).")
	}
	for _, r := range name {
		if invalidSnapshotNameRune(r) {
			return "", fmt.Errorf("Use a filename without slashes or special characters.")
		}
	}
	// Keep snapshot names portable across the desktop platforms.
	base := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" ||
		(len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9') {
		return "", fmt.Errorf("That filename is reserved; choose another name.")
	}
	if asJPEG {
		return name + ".jpg", nil
	}
	return name + ".png", nil
}

func snapshotDirectory() string { return filepath.Join(dataDirPath, "Screenshots") }

// Capture after the next complete draw, with the options window already closed.
func capturePendingSnapshot(screen *ebiten.Image) {
	request := pendingSnapshot
	if request == nil {
		return
	}
	pendingSnapshot = nil
	var shot image.Image
	if request.fullWindow {
		shot = screen
	} else if gameImage != nil {
		rect := worldViewRect.Intersect(gameImage.Bounds())
		if !rect.Empty() {
			shot = gameImage.SubImage(rect)
		}
	}
	var err error
	if shot == nil {
		err = fmt.Errorf("No game view is available to capture.")
	} else {
		var filename string
		filename, err = saveSnapshotImage(shot, snapshotDirectory(), request.name, request.jpeg)
		if err == nil {
			consoleMessage("snapshot taken: " + filepath.Base(filename))
		}
	}
	if err != nil {
		logError("snapshot: %v", err)
	}
	if request.done != nil {
		request.done(err)
	}
}

func saveSnapshotImage(shot image.Image, dir, name string, asJPEG bool) (string, error) {
	filename, err := snapshotFilename(name, asJPEG)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	ext := filepath.Ext(filename)
	stem := strings.TrimSuffix(filename, ext)
	for n := 1; n <= 10000; n++ {
		candidate := filename
		if n > 1 {
			candidate = fmt.Sprintf("%s (%d)%s", stem, n, ext)
		}
		fullPath := filepath.Join(dir, candidate)
		f, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if asJPEG {
			err = jpeg.Encode(f, shot, &jpeg.Options{Quality: 95})
		} else {
			err = png.Encode(f, shot)
		}
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(fullPath)
			return "", err
		}
		return fullPath, nil
	}
	return "", fmt.Errorf("Too many snapshots with this name; choose another filename.")
}
