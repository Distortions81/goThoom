package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

// TestMain initializes a single global audio context for all tests.
func TestMain(m *testing.M) {
	// Only create the context once per process. Ebiten panics on duplicates.
	if audioContext == nil {
		audioContext = audio.NewContext(sampleRate)
	}
	// Point dataDirPath at the repo's ./data so tests can find assets like soundfont.sf2
	if wd, err := os.Getwd(); err == nil {
		dataDirPath = filepath.Join(wd, "data")
	}
	code := m.Run()
	os.Exit(code)
}
