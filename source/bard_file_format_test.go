package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBardNativeAndLegacyExtensions(t *testing.T) {
	bardFixture(t)
	for _, name := range []string{"Native", "Explicit.GTTUNE", "Legacy.TUNE", "Plain.TXT"} {
		tune, err := createBardTune(name, "cde")
		if err != nil {
			t.Fatal(err)
		}
		want := name
		if name == "Native" {
			want += ".gttune"
		}
		if filepath.Base(tune.Path) != want {
			t.Fatalf("created %s, want %s", tune.Path, want)
		}
	}
	tunes, err := listBardTunes()
	if err != nil || len(tunes) != 4 {
		t.Fatalf("mixed library: %d, %v", len(tunes), err)
	}
	if _, err := createBardTune("NATIVE.GTTUNE", "different"); err == nil {
		t.Fatal("overwrote case-insensitive native filename")
	}
	for _, tune := range tunes {
		if tune.Err != nil {
			t.Fatal(tune.Err)
		}
		if err := saveBardInstrument(tune, 17); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(tune.Path); err != nil {
			t.Fatal("instrument edit renamed a legacy file", err)
		}
	}
	if lenMustBardTunes(t) != 4 {
		t.Fatal("editing duplicated files")
	}
}

func lenMustBardTunes(t *testing.T) int {
	t.Helper()
	tunes, err := listBardTunes()
	if err != nil {
		t.Fatal(err)
	}
	return len(tunes)
}

func TestBardImportNativeCopyPreservesLegacySourceAndInstrument(t *testing.T) {
	for _, ext := range []string{".tune", ".TXT", ".gttune"} {
		t.Run(ext, func(t *testing.T) {
			bardFixture(t)
			source := filepath.Join(t.TempDir(), "Café"+ext)
			original := "<A comment>\r\n@90 c4 e4 g8\r\n"
			if err := os.WriteFile(source, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			sidecar := []byte(`{"instrument":17}`)
			if err := os.WriteFile(source+".json", sidecar, 0600); err != nil {
				t.Fatal(err)
			}
			tune, err := importBardTune(source)
			if err != nil {
				t.Fatal(err)
			}
			if filepath.Base(tune.Path) != "Café.gttune" {
				t.Fatal(tune.Path)
			}
			value, err := readBardTune(tune.Path)
			if err != nil || !strings.Contains(value, "<A comment>\n@90 c4 e4 g8\n") {
				t.Fatalf("lost music/comment: %q %v", value, err)
			}
			score, err := parseBardScore(value, defaultInstrument)
			if err != nil || score.Parts[0].Instrument != 17 {
				t.Fatalf("lost sidecar instrument: %+v %v", score, err)
			}
			for path, want := range map[string]string{source: original, source + ".json": string(sidecar)} {
				got, err := os.ReadFile(path)
				if err != nil || string(got) != want {
					t.Fatal("modified source", path, err)
				}
			}
			if _, err := importBardTune(source); err == nil {
				t.Fatal("overwrote imported song")
			}
			if lenMustBardTunes(t) != 1 {
				t.Fatal("failed import left an extra file")
			}
		})
	}
}

func TestBardImportArrangementResolvesEveryInstrument(t *testing.T) {
	bardFixture(t)
	source := filepath.Join(t.TempDir(), "Duet.tune")
	value := "<@title: Two voices>\n<@composer: Someone>\n<@tags: duet>\n<@part: Melody>\nc4 d4\n<@part: Bass>\n<@instrument: Gutbucket Bass>\nc8\n"
	if err := os.WriteFile(source, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source+".json", []byte(`{"instrument":17}`), 0600); err != nil {
		t.Fatal(err)
	}
	want, err := parseBardScore(value, 17)
	if err != nil {
		t.Fatal(err)
	}
	tune, err := importBardTune(source)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := readBardTune(tune.Path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseBardScore(saved, defaultInstrument)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != want.Title || got.Composer != want.Composer || !reflect.DeepEqual(got.Tags, want.Tags) || len(got.Parts) != 2 {
		t.Fatal("lost arrangement metadata")
	}
	for i, part := range got.Parts {
		if part.Name != want.Parts[i].Name || part.Instrument != want.Parts[i].Instrument || part.Text != want.Parts[i].Text {
			t.Fatalf("changed voice: %+v / %+v", part, want.Parts[i])
		}
	}
}

func TestBardImportRejectsUnsupportedContentBeforeWriting(t *testing.T) {
	bardFixture(t)
	for _, value := range []string{"/use /tempo 120 cde", "<@project: something>\ncde", "cde ?", string([]byte{0x8e})} {
		path := filepath.Join(t.TempDir(), "Unknown.gttune")
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := importBardTune(path); err == nil {
			t.Fatalf("accepted unsupported content %q", value)
		}
	}
	if lenMustBardTunes(t) != 0 {
		t.Fatal("unsupported import created a file")
	}
}

func TestBardBundledNativeUpgradeHonorsLegacyHistory(t *testing.T) {
	for _, scenario := range []string{"legacy edit", "legacy deletion", "native copy", "native case variant"} {
		t.Run(scenario, func(t *testing.T) {
			old := dataDirPath
			dataDirPath = t.TempDir()
			t.Cleanup(func() { dataDirPath = old })
			if err := os.MkdirAll(bardTunesDir(), 0755); err != nil {
				t.Fatal(err)
			}
			name := "Three Lanterns.gttune"
			if strings.HasPrefix(scenario, "legacy") {
				name = "Three Lanterns.tune"
				if err := os.WriteFile(filepath.Join(bardTunesDir(), ".included-tunes.json"), []byte(`{"Three Lanterns.tune":true}`), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "native case variant" {
				name = "three lanterns.GTTUNE"
			}
			if scenario != "legacy deletion" {
				if err := os.WriteFile(filepath.Join(bardTunesDir(), name), []byte("<My edit>cde"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := installBundledBardTunes(); err != nil {
				t.Fatal(err)
			}
			if err := installBundledBardTunes(); err != nil {
				t.Fatal(err)
			}
			tunes, err := listBardTunes()
			want := 1
			if scenario == "legacy deletion" {
				want = 0
			}
			if err != nil || len(tunes) != want {
				t.Fatalf("duplicated/restored song: %d %v", len(tunes), err)
			}
			if want == 1 {
				got, err := os.ReadFile(filepath.Join(bardTunesDir(), name))
				if err != nil || string(got) != "<My edit>cde" {
					t.Fatal("altered existing copy", err)
				}
			}
		})
	}
}

func TestBardImportKeepsGlobalDefaultAcrossParts(t *testing.T) {
	bardFixture(t)
	path := filepath.Join(t.TempDir(), "Inherited.tune")
	value := "<@instrument: Pine Flute>\n<@part: First>\nc4\n<@part: Second>\nd4\n<@part: Draft>"
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".json", []byte(`{"instrument":2}`), 0600); err != nil {
		t.Fatal(err)
	}
	tune, err := importBardTune(path)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := readBardTune(tune.Path)
	if err != nil {
		t.Fatal(err)
	}
	score, err := parseBardScore(saved, defaultInstrument)
	if err != nil || len(score.Parts) != 3 {
		t.Fatalf("invalid converted arrangement: %+v %v", score, err)
	}
	for _, part := range score.Parts {
		if part.Instrument != 17 {
			t.Fatalf("part %s lost its inherited instrument", part.Name)
		}
	}
	if score.Parts[2].Text != "" {
		t.Fatal("empty draft part gained music")
	}
}
