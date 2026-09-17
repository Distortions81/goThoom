package main

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed data/tts_substitute.txt
var defaultTTSSubstitute []byte

var (
	ttsSubs   map[string]string
	ttsSubsMu sync.RWMutex
)

const ttsSubstituteFile = "tts_substitute.txt"

func init() {
	loadTTSSubstitutions()
}

func loadTTSSubstitutions() {
	path := filepath.Join(ttsDataDirPath(), ttsSubstituteFile)
	var b []byte
	if isWASM {
		b = append([]byte(nil), defaultTTSSubstitute...)
	} else {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			_ = os.WriteFile(path, defaultTTSSubstitute, 0o644)
		}
		var err error
		b, err = os.ReadFile(path)
		if err != nil {
			logError("read tts_substitute: %v", err)
			return
		}
	}
	m, err := parseTTSSubstitutions(string(b), false)
	if err != nil {
		logError("read tts_substitute: %v", err)
		return
	}
	ttsSubsMu.Lock()
	ttsSubs = m
	ttsSubsMu.Unlock()
}

func substituteTTS(text string) string {
	ttsSubsMu.RLock()
	for from, to := range ttsSubs {
		text = strings.ReplaceAll(text, from, to)
	}
	ttsSubsMu.RUnlock()
	return text
}
