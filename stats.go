package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type assetStats struct {
	Images map[uint16]int `json:"images"`
	Sounds map[uint16]int `json:"sounds"`
}

const statsFile = "stats.json"
const dataDirPath = "data"

var (
	stats      assetStats
	statsMu    sync.Mutex
	statsDirty bool
)

func loadStats() {
	stats.Images = make(map[uint16]int)
	stats.Sounds = make(map[uint16]int)

	path := filepath.Join(dataDirPath, statsFile)
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &stats); err != nil {
			log.Printf("load stats: %v", err)
		}
		if stats.Images == nil {
			stats.Images = make(map[uint16]int)
		}
		if stats.Sounds == nil {
			stats.Sounds = make(map[uint16]int)
		}
	}

	go func() {
		ticker := time.NewTicker(time.Minute)
		for range ticker.C {
			saveStats()
		}
	}()
}

func saveStats() {
	statsMu.Lock()
	if !statsDirty {
		statsMu.Unlock()
		return
	}
	statsDirty = false
	data, err := json.MarshalIndent(stats, "", "  ")
	statsMu.Unlock()
	if err != nil {
		log.Printf("save stats: %v", err)
		return
	}
	path := filepath.Join(dataDirPath, statsFile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("save stats: %v", err)
	}
}

func statImageLoaded(id uint16) {
	if !gs.recordAssetStats {
		return
	}
	statsMu.Lock()
	if stats.Images == nil {
		stats.Images = make(map[uint16]int)
	}
	stats.Images[id]++
	statsDirty = true
	statsMu.Unlock()
}

func statSoundLoaded(id uint16) {
	if !gs.recordAssetStats {
		return
	}
	statsMu.Lock()
	if stats.Sounds == nil {
		stats.Sounds = make(map[uint16]int)
	}
	stats.Sounds[id]++
	statsDirty = true
	statsMu.Unlock()
}
