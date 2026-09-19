package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gothoom/eui"
)

func TestBardEditorCheckIncludesCurrentPartners(t *testing.T) {
	p, _ := bardReadyPanel(t)
	value := "<@instrument: Pine Flute>\n" + strings.Repeat("c", 2450)
	if err := os.WriteFile(p.selected, []byte(value), 0644); err != nil {
		t.Fatal(err)
	}
	p.reload()
	p.editTune()
	path, _ := filepath.Abs(p.selected)
	ed := sourceEditors[path]
	if !ed.check() {
		t.Fatal("solo song should fit five segments", ed.message)
	}
	p.partners.Text = strings.Repeat("A", 32) + "," + strings.Repeat("B", 32)
	if ed.check() || !strings.Contains(ed.message, "with partners") || !strings.Contains(ed.message, "five") {
		t.Fatal("Check missed partner-name overhead", ed.message)
	}
	p.partners.Text = ""
	if !ed.check() {
		t.Fatal("Check kept stale partners", ed.message)
	}
}

func TestBardPartDurationIncludesRestsLoopsAndTempoChanges(t *testing.T) {
	for _, test := range []struct {
		notes string
		want  time.Duration
	}{
		{"c4p4 @60 c4p4", 3 * time.Second},
		{"@120 (c4p4)2 p8", 3 * time.Second},
		{"[ceg]8p8 p8", 2 * time.Second},
		{"@60(c4@+60p4)2", 7 * time.Second / 3},
	} {
		duration, err := bardPartDuration(bardPart{Instrument: 2, Text: test.notes})
		if err != nil || duration != test.want {
			t.Errorf("%q: got %v, want %v: %v", test.notes, duration, test.want, err)
		}
	}
	data, err := bundledBardTunes.ReadFile("data/Tunes/Three Lanterns.tune")
	if err != nil {
		t.Fatal(err)
	}
	score, err := parseBardScore(string(data), 0)
	if err != nil {
		t.Fatal(err)
	}
	summary, warnings := bardTimingSummary(score)
	if len(warnings) != 0 || strings.Count(summary, "48.00 seconds") != 3 {
		t.Fatalf("bundled trio has incorrect timing: %s %v", summary, warnings)
	}
}

func TestBardEnsembleWarningsDoNotBlockIntentionalAssignments(t *testing.T) {
	p, _ := bardReadyPanel(t)
	if err := os.WriteFile(p.selected, []byte(bardDuetFixture+"p8\n"), 0644); err != nil {
		t.Fatal(err)
	}
	p.reload()
	p.partners.Text = "Blue"
	p.showEnsembleWindow()
	p.sharing.assignments[0].Selected = 1
	p.sharing.assignments[0].Handler.Emit(eui.UIEvent{Type: eui.EventDropdownSelected})
	text := strings.Join(strings.Fields(p.sharing.analysis.Text), " ")
	if !strings.Contains(text, "assigned to both") || !strings.Contains(text, "seconds apart") || !strings.Contains(text, "120 BPM") || p.sharing.send.Disabled {
		t.Fatalf("missing nonblocking warnings: %s", text)
	}
	p.sharing.assignments[0].Selected = 2
	p.sharing.assignments[0].Handler.Emit(eui.UIEvent{Type: eui.EventDropdownSelected})
	if strings.Contains(p.sharing.analysis.Text, "assigned to both") {
		t.Fatal("assignment warning did not update")
	}
	p.editTune()
	path, _ := filepath.Abs(p.selected)
	ed := sourceEditors[path]
	if !ed.check() || !strings.Contains(ed.message, "warning") {
		t.Fatalf("different durations should warn without failing Check: %s", ed.message)
	}
}
