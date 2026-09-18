package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gothoom/eui"
	scriptapi "gt2"
)

const bardDuetFixture = `;@title: Moonlight — 月
;@composer: Café
;@tags: quiet, duet, QUIET
; These words and /use commands are never sent.

;@part: Melody
;@instrument: Pine Flute
@90 c4e4g8 ; A gentle entrance <unclosed prose is fine here.

;@part: Accompaniment
;@instrument: Lucky Lyra
@90 c8g8
`

func TestBardScoreMetadataAndSimultaneousParts(t *testing.T) {
	score, err := parseBardScore(bardDuetFixture, 2)
	if err != nil {
		t.Fatal(err)
	}
	if score.Title != "Moonlight — 月" || score.Composer != "Café" || !reflect.DeepEqual(score.Tags, []string{"quiet", "duet"}) {
		t.Fatalf("metadata: %+v", score)
	}
	if len(score.Parts) != 2 || score.Parts[0].Name != "Melody" || score.Parts[0].Instrument != 17 || score.Parts[1].Instrument != 0 {
		t.Fatalf("parts: %+v", score.Parts)
	}
	parts, err := validateBardScore(score)
	if err != nil {
		t.Fatal(err)
	}
	for i, part := range parts {
		if part.program != instruments[score.Parts[i].Instrument].program || part.notes[0].Start != 0 {
			t.Fatalf("part %d does not start with its own instrument at time zero", i)
		}
		commands, err := bardEnsembleCommands(score.Parts[i].Text, []string{"Blue"})
		if err != nil || len(commands) != 1 || strings.ContainsAny(commands[0], ";<>\n") || strings.Contains(commands[0], "Moonlight") {
			t.Fatalf("metadata/comment leaked into commands: %q %v", commands, err)
		}
	}
	if _, err := bardTuneCommands(bardDuetFixture); err == nil {
		t.Fatal("silently combined the performers' parts")
	}
	// A trio adds an independent timeline; leading rests delay only that part.
	trio, err := parseBardScore(bardDuetFixture+";@part: Bass\n;@instrument: Gutbucket Bass\n@90 p4c8\n", 0)
	if err != nil {
		t.Fatal(err)
	}
	music, err := validateBardScore(trio)
	if err != nil || len(music) != 3 || music[0].notes[0].Start != 0 || music[2].notes[0].Start <= 0 {
		t.Fatalf("trio timelines: %+v %v", music, err)
	}
}

func TestBardScoreLegacyAndCommentCompatibility(t *testing.T) {
	if _, err := validateBardTune("<"+strings.Repeat("x", bardMaxFileSize-3)+">c", 0); err != nil {
		t.Fatal("rejected a tune at the file-size limit", err)
	}
	for _, value := range []string{
		"<old ; comments\n;@part: not metadata\n>c<; keep sharp>#4d",
		"; arbitrary text > < /use\nc#4d ; ignored\n",
		";@instrument: Pine Flute\nc#4d",
	} {
		got, err := validateBardTune(value, 17)
		want, wantErr := validateBardTune("c#4d", 17)
		if err != nil || wantErr != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("comments changed music: %q: %v", value, err)
		}
	}
	score, err := parseBardScore(";@instrument: Starbuck Harp\n;@part: One\ncde\n;@part: Two\ngab", 17)
	if err != nil || score.Parts[0].Instrument != 2 || score.Parts[1].Instrument != 2 {
		t.Fatal("song instrument was not inherited by both parts", err)
	}
	for _, value := range []string{
		";@part:\ncde", ";@part: A\ncde\n;@part: a\ngab",
		"cde\n;@part: A\ngab", ";@instrument: kazoo\ncde",
		";@part: A\n;@title: late\ncde", ";@title missing colon\ncde",
		";@title: A\n;@title: B\ncde", ";@instrumnt: Pine Flute\ncde",
		";@instrument: Pine Flute\n;@instrument: Bone Flute\ncde",
		";@part: Melody\n<unclosed", "c>de",
	} {
		if _, err := parseBardScore(value, 0); err == nil {
			t.Fatalf("accepted invalid score %q", value)
		}
	}
	bad, err := parseBardScore(bardDuetFixture+";@part: Broken\n;@instrument: Pine Flute\n[ceg]C", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := validateBardScore(bad); err == nil || !strings.Contains(err.Error(), "Broken") {
		t.Fatal("validation did not identify the failing part", err)
	}
}

func TestBardEnsembleCommandSplitting(t *testing.T) {
	value := "@90 " + strings.Repeat("c#4d.e_f2g ", 130)
	commands, err := bardEnsembleCommands(value, []string{"Blue", "Example d'Exile"})
	if err != nil || len(commands) < 2 {
		t.Fatalf("split: %v %v", commands, err)
	}
	var music []string
	for i, command := range commands {
		prefix := "/use /part /with Blue /with ExampledExile "
		if i == len(commands)-1 {
			prefix = "/use /with Blue /with ExampledExile "
		}
		if !strings.HasPrefix(command, prefix) || len(command) > 511 {
			t.Fatalf("invalid ensemble command: %q", command)
		}
		music = append(music, strings.TrimPrefix(command, prefix))
	}
	got, err := validateBardTune(strings.Join(music, " "), 17)
	want, wantErr := validateBardTune(value, 17)
	if err != nil || wantErr != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("ensemble command splitting changed the music", err, wantErr)
	}
	for _, names := range [][]string{{"A", "B", "C"}, {"Blue", "blue"}, {""}, {"Blue\n/use /stop"}, {"/stop"}} {
		if _, err := bardEnsembleCommands("cde", names); err == nil {
			t.Fatalf("accepted invalid partners: %q", names)
		}
	}
	if _, err := bardEnsembleCommands(strings.Repeat("c", 2499), []string{"Blue", "Pixy"}); err == nil {
		t.Fatal("did not account for /with overhead in the five-segment limit")
	}
}

func TestBardInstrumentMetadataPreservesFilesAndDrafts(t *testing.T) {
	bardFixture(t)
	tune, err := createBardTune("duet", bardDuetFixture)
	if err != nil {
		t.Fatal(err)
	}
	// Preserve the source document's BOM and newline style when editing metadata.
	raw := "\ufeff" + strings.ReplaceAll(bardDuetFixture, "\n", "\r\n")
	if err := os.WriteFile(tune.Path, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	if err := saveBardPartInstrument(tune, 1, 2); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(tune.Path)
	want := strings.Replace(raw, ";@instrument: Lucky Lyra", ";@instrument: Starbuck Harp", 1)
	if err != nil || string(got) != want {
		t.Fatal("instrument change rewrote unrelated file content", err)
	}
	if _, err := os.Stat(tune.Path + ".json"); !os.IsNotExist(err) {
		t.Fatal("created a new sidecar preference", err)
	}
	showBardWindow()
	p := bardWindow
	p.selected = tune.Path
	p.refreshSelection()
	p.editTune()
	path, _ := filepath.Abs(tune.Path)
	ed := sourceEditors[path]
	draft := ed.input.Text + "; unsaved\n"
	editMacroForTest(ed, draft)
	if err := saveBardPartInstrument(tune, 0, 1); err == nil {
		t.Fatal("changed an instrument beneath an unsaved editor")
	}
	if got, err := os.ReadFile(tune.Path); err != nil || string(got) != want || ed.input.Text != draft {
		t.Fatal("instrument change lost the saved file or unsaved draft", err)
	}
	if !ed.save(false) {
		t.Fatal(ed.message)
	}
	clickMacroEditorButton(t, ed.win, "Check")
	if ed.status.Invisible || ed.message != "Check passed." {
		t.Fatal("valid ensemble check was not reported", ed.message)
	}
	editMacroForTest(ed, ed.input.Text+";@part: Bad\n?")
	clickMacroEditorButton(t, ed.win, "Check")
	if !strings.Contains(ed.message, "Part Bad:") {
		t.Fatal("check did not identify the bad part", ed.message)
	}
}

func TestBardLibraryMetadataSearchAndPartSelection(t *testing.T) {
	bardFixture(t)
	duet, err := createBardTune("filename", bardDuetFixture)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := createBardTune("A solo", "cdef")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy.Path+".json", []byte(`{"instrument":17}`), 0644); err != nil {
		t.Fatal(err)
	}
	showBardWindow()
	p := bardWindow
	for _, query := range []string{"moonlight", "café", "quiet", "accompaniment"} {
		p.query = query
		found := p.filteredTunes()
		if len(found) != 1 || found[0].Path != duet.Path {
			t.Fatalf("metadata search %q returned %+v", query, found)
		}
	}
	p.query = ""
	p.sortOrder.Selected = 3
	if got := p.filteredTunes(); len(got) != 2 || got[0].Path != legacy.Path || got[0].Instrument != 17 {
		t.Fatalf("part-count sort/legacy instrument: %+v", got)
	}
	p.tag.Selected = 1
	if got := p.filteredTunes(); len(got) != 1 || got[0].Path != duet.Path {
		t.Fatalf("tag filter: %+v", got)
	}
	p.selected = duet.Path
	p.refreshSelection()
	if p.part.Invisible || p.previewPart.Invisible || p.partners.Invisible || p.previewButton.Text != "Preview All" {
		t.Fatal("ensemble controls are missing")
	}
	p.part.Selected = 1
	p.part.Handler.Handle(eui.UIEvent{Type: eui.EventDropdownSelected})
	if p.selectedPart != "Accompaniment" || p.instrument.Selected != 0 {
		t.Fatal("part selector did not choose its own instrument")
	}
	p.reload()
	if p.selectedPart != "Accompaniment" || p.part.Selected != 1 {
		t.Fatal("refresh lost the selected part")
	}
	s := bardConnectedSession(t)
	appSessions = newSessionManager(s)
	s.inventory.add(222, -1, "Lucky Lyra", false)
	p.refreshSelection()
	p.partners.Text = "Blue"
	p.play()
	if p.playConfirm == nil || p.performance != nil || !s.commands.idle() {
		t.Fatal("playback did not wait for confirmation")
	}
	clickMacroEditorButton(t, p.playConfirm, "Play in Game")
	if p.performance == nil || p.performance.instrument.ID != 222 || !p.performance.ensemble {
		t.Fatal("did not prepare the selected ensemble part", p.status.Text)
	}
	s.inventory.equip(222, -1, true)
	s.commands.mu.Lock()
	p.performance.tickets[0].state.status.State = scriptapi.CommandSent
	s.commands.mu.Unlock()
	s.commands.clear()
	if err := p.performance.update(time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := bardQueueTexts(s); !reflect.DeepEqual(got, []string{"/use /stop", "/use /with Blue @90 c8g8"}) {
		t.Fatalf("played the wrong part: %q", got)
	}
	// Sending our last segment is not evidence that the remote bard has started.
	s.commands.mu.Lock()
	p.performance.tickets[len(p.performance.tickets)-1].state.status.State = scriptapi.CommandSent
	s.commands.mu.Unlock()
	if err := p.performance.update(time.Now()); err != nil || !p.performance.finishAt.IsZero() {
		t.Fatal("guessed ensemble finish before other performers were ready", err)
	}
}

func TestBardInstrumentEditFollowsPartAfterExternalReorder(t *testing.T) {
	bardFixture(t)
	melody := ";@part: Melody\n;@instrument: Pine Flute\ncde\n"
	bass := ";@part: Bass\n;@instrument: Gutbucket Bass\ngab\n"
	tune, err := createBardTune("duet", melody+bass)
	if err != nil {
		t.Fatal(err)
	}
	listed, err := listBardTunes()
	if err != nil || len(listed) != 1 {
		t.Fatal("could not list tune", err)
	}
	if err := os.WriteFile(tune.Path, []byte(bass+melody), 0644); err != nil {
		t.Fatal(err)
	}
	if err := saveBardPartInstrument(listed[0], 0, 1); err != nil {
		t.Fatal(err)
	}
	got, err := readBardTune(tune.Path)
	want := bass + strings.Replace(melody, "Pine Flute", "Bone Flute", 1)
	if err != nil || got != want {
		t.Fatalf("instrument was applied to the wrong part: %q %v", got, err)
	}
}

func TestBardPerformanceUsesEmbeddedInstrument(t *testing.T) {
	s := bardConnectedSession(t)
	s.inventory.add(321, -1, "Pine Flute", false)
	p, err := startBardPerformance(s, ";@instrument: Pine Flute\ncde", 0)
	if err != nil || p.instrument.ID != 321 {
		t.Fatal("performance ignored the file's instrument", err)
	}
}
