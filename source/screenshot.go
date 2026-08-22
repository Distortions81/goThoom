package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func takeScreenshot() {
	if isWASM {
		// Screenshots not supported in WASM; ignore.
		return
	}
	var shot image.Image
	if gameImage != nil {
		rect := worldViewRect
		if !rect.Empty() {
			rect = rect.Intersect(gameImage.Bounds())
			if !rect.Empty() {
				shot = gameImage.SubImage(rect)
			}
		}
	}
	if shot == nil && worldRT != nil {
		rect := worldRTUsedRect
		if !rect.Empty() {
			rect = rect.Intersect(worldRT.Bounds())
			if !rect.Empty() {
				shot = worldRT.SubImage(rect)
			}
		}
	}
	if shot == nil {
		return
	}

	dir := filepath.Join(dataDirPath, "Screenshots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logError("screenshot: create %v: %v", dir, err)
		return
	}
	ts := time.Now().Format("2006-01-02-15-04-05")
	buf := "clanlord-"
	if clmov != "" {
		clname := path.Base(clmov)
		clname = strings.ToLower(clname)
		clname = strings.TrimSuffix(clname, ".clmov")
		cllen := len(clname)
		if cllen > 16 {
			clname = clname[:16]
		}
		buf = clname
	}
	if gs.LastCharacter != "" {
		buf = gs.LastCharacter
	}

	fn := filepath.Join(dir, fmt.Sprintf("%v__%s.png", buf, ts))
	f, err := os.Create(fn)
	if err != nil {
		logError("screenshot: create %v: %v", fn, err)
		return
	}
	defer f.Close()
	if err := png.Encode(f, shot); err != nil {
		logError("screenshot: encode %v: %v", fn, err)
		return
	}
	consoleMessage(fmt.Sprintf("snapshot taken: %s", filepath.Base(fn)))
}
