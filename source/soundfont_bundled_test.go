package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Distortions81/go-meltysynth/meltysynth"
)

func TestBundledSoundFontIsDefault(t *testing.T) {
	preserveStoragePathTestState(t)
	gs = gsdef
	if soundFontFile != "qt.sf2" || gsdef.SoundFontFile != soundFontFile {
		t.Fatalf("default SoundFont = %q, configured default = %q; want qt.sf2", soundFontFile, gsdef.SoundFontFile)
	}
	if alternateSoundFontFile != "soundfont.sf2" {
		t.Fatalf("previous SoundFont filename = %q, want soundfont.sf2", alternateSoundFontFile)
	}
	for _, name := range []string{soundFontFile, "", "../outside.sf2", "invalid.txt"} {
		gs.SoundFontFile = name
		if got := configuredSoundFontFile(); got != soundFontFile {
			t.Errorf("configured SoundFont for %q = %q, want %q", name, got, soundFontFile)
		}
	}
	gs.SoundFontFile = alternateSoundFontFile
	if got := configuredSoundFontFile(); got != alternateSoundFontFile {
		t.Fatalf("saved previous SoundFont selection = %q, want %q", got, alternateSoundFontFile)
	}
}

func TestBundledSoundFontWithoutAssetsDirectory(t *testing.T) {
	preserveStoragePathTestState(t)
	gs = gsdef
	gs.AssetsPath = filepath.Join(t.TempDir(), "missing-assets")
	activeStoragePaths.assets = gs.AssetsPath
	storagePathsActivated = true
	fonts, err := listSoundFonts()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{soundFontFile}; !reflect.DeepEqual(fonts, want) {
		t.Fatalf("available SoundFonts = %v, want %v", fonts, want)
	}
	if _, err := loadMusicSoundFont(soundFontFile); err != nil {
		t.Fatalf("load bundled SoundFont without an assets directory: %v", err)
	}
	if _, err := os.Stat(gs.AssetsPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("loading the bundled SoundFont created the missing assets directory: %v", err)
	}
}

func TestBundledSoundFontListsPreviousAndCustomOptions(t *testing.T) {
	preserveStoragePathTestState(t)
	gs = gsdef
	gs.AssetsPath = t.TempDir()
	activeStoragePaths.assets = gs.AssetsPath
	storagePathsActivated = true
	for _, name := range []string{soundFontFile, alternateSoundFontFile, "custom.sf2"} {
		if err := os.WriteFile(filepath.Join(gs.AssetsPath, name), bundledSoundFont, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(gs.AssetsPath, "broken.sf2"), []byte("invalid SF2"), 0o644); err != nil {
		t.Fatal(err)
	}
	fonts, err := listSoundFonts()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{soundFontFile, "custom.sf2", alternateSoundFontFile}
	if !reflect.DeepEqual(fonts, want) {
		t.Fatalf("available SoundFonts = %v, want builtin first, both alternatives, and no duplicate: %v", fonts, want)
	}
	for _, name := range []string{alternateSoundFontFile, "custom.sf2"} {
		if _, err := loadMusicSoundFont(name); err != nil {
			t.Errorf("load selectable SoundFont %q: %v", name, err)
		}
	}
	if _, err := loadMusicSoundFont("broken.sf2"); err == nil {
		t.Fatal("invalid custom SoundFont unexpectedly loaded")
	}
	if _, err := loadMusicSoundFont("missing.sf2"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing custom SoundFont error = %v, want file-not-found", err)
	}
}

func TestBundledSoundFontSettingsDefaultsAndSavedChoices(t *testing.T) {
	for _, test := range []struct {
		name, audio, want string
	}{
		{name: "missing field", audio: `{}`, want: soundFontFile},
		{name: "previous selection", audio: `{"music_soundfont":"soundfont.sf2"}`, want: alternateSoundFontFile},
		{name: "custom selection", audio: `{"music_soundfont":"custom.sf2"}`, want: "custom.sf2"},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := []byte(fmt.Sprintf(`{"version":%d,"audio":%s}`, SETTINGS_VERSION, test.audio))
			settings, err := unmarshalSettingsDocument(data, gsdef)
			if err != nil {
				t.Fatal(err)
			}
			if settings.SoundFontFile != test.want {
				t.Fatalf("saved/default SoundFont = %q, want %q", settings.SoundFontFile, test.want)
			}
		})
	}
}

func TestBundledSoundFontRendersEveryBardProgram(t *testing.T) {
	font, err := loadMusicSoundFont(soundFontFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(font.Presets) == 0 || len(font.WaveData) == 0 {
		t.Fatal("bundled SoundFont has no presets or sample data")
	}
	checked := make(map[int]bool)
	for _, instrument := range instruments {
		program := instrument.program
		if checked[program] {
			continue
		}
		checked[program] = true
		t.Run(fmt.Sprintf("program_%d", program), func(t *testing.T) {
			if !soundFontHasProgram(font, program) {
				t.Fatalf("bundled SoundFont lacks Bank 0 preset %d", program)
			}
			keys := []int32{48, 60, 72}
			if instrument.allowedNoteClass != 0 {
				// Orga Drum permits G and B, unlike the calibration C notes.
				keys = []int32{43, 47, 55, 59, 67, 71}
			}
			for _, key := range keys {
				synth, err := meltysynth.NewSynthesizer(font, newSynthSettings())
				if err != nil {
					t.Fatal(err)
				}
				synth.ProcessMidiMessage(0, 0xC0, int32(program), 0)
				synth.NoteOn(0, key, 100)
				var left, right [block]float32
				peak := 0.0
				for frame := 0; frame < sampleRate; frame += block {
					if frame == (sampleRate*3/4)/block*block {
						synth.NoteOff(0, key)
					}
					synth.Render(left[:], right[:])
					for i := range left {
						for _, value := range [2]float32{left[i], right[i]} {
							v := float64(value)
							if math.IsNaN(v) || math.IsInf(v, 0) {
								t.Fatalf("key %d produced nonfinite audio at frame %d", key, frame+i)
							}
							peak = max(peak, math.Abs(v))
						}
					}
				}
				if peak <= 1e-8 {
					t.Errorf("key %d produced silent audio", key)
				}
			}
		})
	}
}
