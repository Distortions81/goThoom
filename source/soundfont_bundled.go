package main

import (
	"bytes"
	_ "embed"
	"path/filepath"

	meltysynth "github.com/Distortions81/go-meltysynth/meltysynth"
)

const soundFontFile = "qt.sf2"

//go:embed data/audio/qt.sf2
var bundledSoundFont []byte

// The default is available without downloads or a writable Assets & Audio
// folder. Other SoundFonts remain files in the player's chosen audio folder.
func loadMusicSoundFont(name string) (*meltysynth.SoundFont, error) {
	if name == soundFontFile {
		return meltysynth.NewSoundFont(bytes.NewReader(bundledSoundFont))
	}
	return loadSoundFont(filepath.Join(soundFontsDirPath(), name))
}
