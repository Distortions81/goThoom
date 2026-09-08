package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

// TestMain initializes a single global audio context for all tests.
func TestMain(m *testing.M) {
	// Never let tests emit audio through the host device. Player creation and
	// lifecycle remain exercised; each player is simply held at zero volume.
	muteAudioOutputForTests = true
	// Only create the context once per process. Ebiten panics on duplicates.
	if audioContext == nil {
		audioContext = audio.NewContext(sampleRate)
	}
	// Keep test data rooted in the repo. Audio integration tests separately
	// resolve an installed SoundFont without directing writes into user data.
	if wd, err := os.Getwd(); err == nil {
		dataDirPath = filepath.Join(wd, "data")
	}
	code := m.Run()
	os.Exit(code)
}
