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
	path := filepath.Join(dataDirPath, ttsSubstituteFile)
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
	m := make(map[string]string)
	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.Index(line, "="); idx >= 0 {
			from := strings.TrimSpace(line[:idx])
			to := strings.TrimSpace(line[idx+1:])
			if from != "" {
				m[from] = to
			}
		}
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
