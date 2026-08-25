package main

import (
	"log"
	"sort"
	"sync"
)

var (
	assetDumpOnce     sync.Once
	assetDumpComplete = make(chan struct{})
)

func assetDumpMode() bool {
	return imgDump || sndDump
}

// exportAssets runs from Game.Update, after Ebiten has initialized its graphics
// context. This makes the dump flags useful as one-shot archive exporters
// rather than depending on which assets happen to be used during a session.
func exportAssets() {
	if imgDump {
		exportImages()
	}
	if sndDump {
		exportSounds()
	}
	log.Print("Asset export complete.")
	close(assetDumpComplete)
}

func exportImages() {
	if clImages == nil {
		log.Print("Image export skipped: CL_Images is not available.")
		return
	}
	ids := clImages.IDs()
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	log.Printf("Exporting %d images to dump/img...", len(ids))
	for _, id := range ids {
		if id > 0xffff {
			log.Printf("Skipping image %d: ID exceeds the client range.", id)
			continue
		}
		if sheet := clImages.Get(id, nil, false); sheet != nil {
			dumpImageSheet(uint16(id), sheet)
		}
	}
	log.Print("Image export complete.")
}

func exportSounds() {
	if clSounds == nil {
		log.Print("Sound export skipped: CL_Sounds is not available.")
		return
	}
	ids := clSounds.IDs()
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	log.Printf("Exporting %d sounds to dump/snd...", len(ids))
	for _, id := range ids {
		if id > 0xffff {
			log.Printf("Skipping sound %d: ID exceeds the client range.", id)
			continue
		}
		loadSound(uint16(id))
	}
	log.Print("Sound export complete.")
}
