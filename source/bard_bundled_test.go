package main

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBardBundledTrio(t *testing.T) {
	data, err := bundledBardTunes.ReadFile("data/Tunes/Three Lanterns.tune")
	if err != nil {
		t.Fatal(err)
	}
	score, err := parseBardScore(string(data), 0)
	if err != nil {
		t.Fatal(err)
	}
	music, err := validateBardScore(score)
	if err != nil || len(music) != 3 {
		t.Fatalf("trio: %v", err)
	}
	starts := []time.Duration{4 * time.Second, 2 * time.Second, 0}
	instruments := []int{17, 2, 21}
	var ending time.Duration
	for i, part := range score.Parts {
		if part.Instrument != instruments[i] || music[i].notes[0].Start != starts[i] {
			t.Fatalf("part %d has wrong instrument or entrance", i)
		}
		var end time.Duration
		for _, note := range music[i].notes {
			end = max(end, note.Start+note.Duration)
		}
		if i == 0 {
			ending = end
		}
		// Classic chord releases round to milliseconds; melody releases retain
		// the 1/600-second clock. Their musical endpoints still agree.
		if end-ending > time.Millisecond || ending-end > time.Millisecond || end < 47*time.Second || end > 48*time.Second {
			t.Fatalf("part %d ends at %v, ensemble at %v", i, end, ending)
		}
		// The final marker proves the complete timeline, including rests, is 24 bars.
		withMarker, err := validateBardTune(part.Text+"\n=c1", part.Instrument)
		if err != nil || withMarker[len(withMarker)-1].Start != 48*time.Second {
			t.Fatalf("part %d is not 24 bars: %v", i, err)
		}
		commands, err := bardEnsembleCommands(part.Text, []string{"Blue", "Pixy"})
		if err != nil || len(commands) > 5 {
			t.Fatalf("part %d cannot perform: %v", i, err)
		}
		messages, err := bardPartMessages(score.Title, part)
		if err != nil {
			t.Fatalf("part %d cannot share: %v", i, err)
		}
		var incoming bardIncomingPart
		for _, message := range messages {
			meta, id, index, count, chunk, err := parseBardPartMessage(message)
			if err != nil {
				t.Fatal(err)
			}
			if incoming.chunks == nil {
				incoming = bardIncomingPart{part: meta, id: id, chunks: make([]string, count)}
			}
			incoming.chunks[index] = chunk
		}
		received, err := decodeBardSharedPart(&incoming)
		if err != nil {
			t.Fatal(err)
		}
		notes, err := validateBardTune(received.Notes, received.Instrument)
		if err != nil || !reflect.DeepEqual(notes, music[i].notes) {
			t.Fatalf("sharing changed part %d: %v", i, err)
		}
		t.Logf("%s: %d notes, %d game commands, %d sharing messages", part.Name, len(music[i].notes), len(commands), len(messages))
	}
}

func TestRenderBardBundledTrio(t *testing.T) {
	output := os.Getenv("GOTHOOM_RENDER_BARD_TRIO")
	if output == "" {
		t.Skip("set GOTHOOM_RENDER_BARD_TRIO to a WAV output path")
	}
	root := platformDataDir(runtime.GOOS, runtime.GOARCH, os.Getenv, os.UserHomeDir, os.Executable)
	fontPath := filepath.Join(root, soundFontFile)
	font, err := loadSoundFont(fontPath)
	if err != nil {
		t.Fatal("installed SoundFont:", err)
	}
	setupSynthOnce.Do(func() {})
	synthCacheMu.Lock()
	oldFont, oldSettings := sfntCached, synthSettings
	sfntCached, synthSettings = font, newSynthSettings()
	synthGeneration++
	synthCacheMu.Unlock()
	t.Cleanup(func() {
		synthCacheMu.Lock()
		sfntCached, synthSettings = oldFont, oldSettings
		synthGeneration++
		synthCacheMu.Unlock()
		setupSynthOnce = sync.Once{}
	})
	data, err := bundledBardTunes.ReadFile("data/Tunes/Three Lanterns.tune")
	if err != nil {
		t.Fatal(err)
	}
	score, err := parseBardScore(string(data), 0)
	if err != nil {
		t.Fatal(err)
	}
	parts, err := validateBardScore(score)
	if err != nil {
		t.Fatal(err)
	}
	renderer, err := newMusicGroupRenderer(parts)
	if err != nil {
		t.Fatal(err)
	}
	left, right, err := renderer.render(renderer.remaining())
	if err != nil {
		t.Fatal(err)
	}
	peak := float64(0)
	for i := range left {
		peak = max(peak, math.Abs(float64(left[i])), math.Abs(float64(right[i])))
	}
	if peak == 0 || math.IsNaN(peak) || math.IsInf(peak, 0) {
		t.Fatalf("invalid rendered audio peak: %g", peak)
	}
	if peak > 0.9 {
		for i := range left {
			left[i] *= float32(0.9 / peak)
			right[i] *= float32(0.9 / peak)
		}
	}
	if err := writePCMAsWAV(output, mixPCMChunk(left, right, true)); err != nil {
		t.Fatal(err)
	}
	t.Logf("Rendered %s with %s (peak %.3f)", output, fontPath, peak)
}

func TestBardBundledInstallPreservesUserFiles(t *testing.T) {
	for _, scenario := range []string{"fresh", "edited", "deleted", "existing", "case variant", "bad manifest"} {
		t.Run(scenario, func(t *testing.T) {
			old := dataDirPath
			dataDirPath = t.TempDir()
			t.Cleanup(func() { dataDirPath = old })
			if err := os.MkdirAll(bardTunesDir(), 0755); err != nil {
				t.Fatal(err)
			}
			filename := "Three Lanterns.gttune"
			if scenario == "existing" || scenario == "case variant" {
				filename = "Three Lanterns.tune"
			}
			if scenario == "case variant" {
				filename = strings.ToLower(filename)
			}
			path := filepath.Join(bardTunesDir(), filename)
			if scenario == "existing" || scenario == "case variant" {
				if err := os.WriteFile(path, []byte("<My tune>cde"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "bad manifest" {
				if err := os.WriteFile(filepath.Join(bardTunesDir(), ".included-tunes.json"), []byte("broken"), 0644); err != nil {
					t.Fatal(err)
				}
				if err := installBundledBardTunes(); err == nil {
					t.Fatal("ignored damaged install history")
				}
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("installed despite unreadable history")
				}
				return
			}
			if err := installBundledBardTunes(); err != nil {
				t.Fatal(err)
			}
			if scenario == "edited" {
				if err := os.WriteFile(path, []byte("<My tune>cde"), 0644); err != nil {
					t.Fatal(err)
				}
			} else if scenario == "deleted" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			if err := installBundledBardTunes(); err != nil {
				t.Fatal(err)
			}
			tunes, err := listBardTunes()
			if err != nil || scenario == "deleted" && len(tunes) != 0 || scenario != "deleted" && len(tunes) != 1 {
				t.Fatalf("duplicate or resurrected bundled song: %d, %v", len(tunes), err)
			}
			got, err := os.ReadFile(path)
			switch scenario {
			case "deleted":
				if !os.IsNotExist(err) {
					t.Fatal("restored a deleted tune")
				}
			case "fresh":
				want, readErr := bundledBardTunes.ReadFile("data/Tunes/Three Lanterns.tune")
				if err != nil || readErr != nil || string(got) != string(want) {
					t.Fatal("default tune missing or altered", err, readErr)
				}
			default:
				if err != nil || string(got) != "<My tune>cde" {
					t.Fatal("replaced a user's tune", err)
				}
			}
		})
	}
}
