package main

import (
	"log"
	"path/filepath"
	"sort"
	"sync"
)

var (
	assetDumpOnce     sync.Once
	assetDumpComplete = make(chan struct{})
)

func assetDumpMode() bool {
	return imgDump || sndDump || dumpPreloadAssets
}

// exportAssets runs from Game.Update, after Ebiten has initialized its graphics
// context. This makes the dump flags useful as one-shot archive exporters
// rather than depending on which assets happen to be used during a session.
func exportAssets() {
	if dumpPreloadAssets {
		exportPreloadImages()
		exportPreloadSounds()
		log.Print("Preload asset export complete.")
		close(assetDumpComplete)
		return
	}
	if imgDump {
		exportImages()
	}
	if sndDump {
		exportSounds()
	}
	log.Print("Asset export complete.")
	close(assetDumpComplete)
}

func exportPreloadImages() {
	if clImages == nil {
		log.Print("Preload image export skipped: CL_Images is not available.")
		return
	}
	log.Printf("Exporting %d preloaded images to %s...", len(startupArtworkPreloadIDs), assetDumpImageDir())
	for _, id := range startupArtworkPreloadIDs {
		if sheet := clImages.Get(uint32(id), nil, false); sheet != nil {
			dumpImageSheet(id, sheet)
		}
	}
}

func exportPreloadSounds() {
	if currentCLSoundsArchive() == nil {
		log.Print("Preload sound export skipped: CL_Sounds is not available.")
		return
	}
	log.Printf("Exporting %d preloaded sounds to %s...", len(startupSoundPreloadIDs), assetDumpSoundDir())
	for _, id := range startupSoundPreloadIDs {
		loadSound(id)
	}
}

func assetDumpImageDir() string {
	if dumpPreloadAssets {
		return filepath.Join("dump", "preload", "img")
	}
	return filepath.Join("dump", "img")
}

func assetDumpSoundDir() string {
	if dumpPreloadAssets {
		return filepath.Join("dump", "preload", "snd")
	}
	return filepath.Join("dump", "snd")
}

func exportImages() {
	if clImages == nil {
		log.Print("Image export skipped: CL_Images is not available.")
		return
	}
	ids := clImages.IDs()
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	log.Printf("Exporting images: 0/%d (0%%) to dump/img...", len(ids))
	for n, id := range ids {
		if id > 0xffff {
			log.Printf("Skipping image %d: ID exceeds the client range.", id)
		} else if sheet := clImages.Get(id, nil, false); sheet != nil {
			dumpImageSheet(uint16(id), sheet)
		}
		if exportProgressDue(n+1, len(ids)) {
			log.Printf("Exporting images: %d/%d (%d%%)", n+1, len(ids), exportPercent(n+1, len(ids)))
		}
	}
	log.Printf("Image export complete: %d images.", len(ids))
}

func exportSounds() {
	sounds := currentCLSoundsArchive()
	if sounds == nil {
		log.Print("Sound export skipped: CL_Sounds is not available.")
		return
	}
	ids := sounds.IDs()
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	log.Printf("Exporting sounds: 0/%d (0%%) to dump/snd...", len(ids))
	for n, id := range ids {
		if id > 0xffff {
			log.Printf("Skipping sound %d: ID exceeds the client range.", id)
		} else {
			loadSound(uint16(id))
		}
		if exportProgressDue(n+1, len(ids)) {
			log.Printf("Exporting sounds: %d/%d (%d%%)", n+1, len(ids), exportPercent(n+1, len(ids)))
		}
	}
	log.Printf("Sound export complete: %d sounds.", len(ids))
}

func exportProgressDue(done, total int) bool {
	return done == total || done%100 == 0
}

func exportPercent(done, total int) int {
	if total == 0 {
		return 100
	}
	return done * 100 / total
}
