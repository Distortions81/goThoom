package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gothoom/eui"
)

func bardReadyPanel(t *testing.T) (*bardPanel, *Session) {
	t.Helper()
	bardFixture(t)
	session := bardConnectedSession(t)
	appSessions = newSessionManager(session)
	session.setCharacterName("Flutist")
	session.inventory.add(321, -1, "Pine Flute", false)
	tune, err := createBardTune("duet", bardDuetFixture)
	if err != nil {
		t.Fatal(err)
	}
	showBardWindow()
	bardWindow.selected = tune.Path
	bardWindow.refreshSelection()
	bardWindow.refreshList()
	return bardWindow, session
}

func TestBardPlayConfirmationAndCancel(t *testing.T) {
	p, session := bardReadyPanel(t)
	p.preview = &bardPreview{who: -999}
	preview := p.preview
	p.partners.Text = "Blue"
	p.play()
	popup := p.playConfirm
	if popup == nil || popup.DefaultButton == nil || popup.DefaultButton.Text != "Cancel" {
		t.Fatal("missing confirmation with Cancel as the default action")
	}
	if !session.commands.idle() || p.performance != nil || preview.stopped.Load() {
		t.Fatal("requesting confirmation changed playback or queued a game command")
	}
	var message strings.Builder
	for _, item := range popup.Contents[0].Contents {
		message.WriteString(item.Text)
	}
	for _, want := range []string{"Moonlight", "Flutist", "Melody", "Pine Flute", "Blue"} {
		if !strings.Contains(message.String(), want) {
			t.Fatalf("confirmation omitted %q", want)
		}
	}
	clickMacroEditorButton(t, popup, "Cancel")
	if p.playConfirm != nil || !session.commands.idle() || preview.stopped.Load() {
		t.Fatal("cancel did not leave playback alone")
	}
	p.play()
	clickMacroEditorButton(t, p.playConfirm, "Play in Game")
	if p.performance == nil || !preview.stopped.Load() || len(bardQueueTexts(session)) != 1 {
		t.Fatal("confirmed playback did not stop preview and queue equipping")
	}
}

func TestBardConfirmationRejectsChangesSincePrompt(t *testing.T) {
	for _, change := range []string{"song", "part", "file", "connection", "character", "closed"} {
		t.Run(change, func(t *testing.T) {
			p, session := bardReadyPanel(t)
			p.play()
			popup := p.playConfirm
			if popup == nil {
				t.Fatal("no confirmation")
			}
			switch change {
			case "song":
				p.selected = "different.tune"
			case "part":
				p.selectedPart = "Accompaniment"
			case "file":
				if err := os.WriteFile(p.selected, []byte("cdef"), 0644); err != nil {
					t.Fatal(err)
				}
			case "connection":
				session.transport.mu.Lock()
				session.transport.generation++
				session.transport.mu.Unlock()
			case "character":
				session.setCharacterName("Someone else")
			case "closed":
				p.win.Close()
				if p.playConfirm != nil || popup.IsOpen() {
					t.Fatal("closing Bard left a playback confirmation open")
				}
			}
			// Even an already queued click on a stale popup must not perform.
			clickMacroEditorButton(t, popup, "Play in Game")
			if !session.commands.idle() || p.performance != nil || !p.statusProblem {
				t.Fatal("a stale confirmation performed or failed without explanation")
			}
		})
	}
}

func TestBardRadioSelectionAndFixedStatus(t *testing.T) {
	p, session := bardReadyPanel(t)
	other, err := createBardTune("Other song", "cde")
	if err != nil {
		t.Fatal(err)
	}
	p.reload()
	if !p.list.Outlined {
		t.Fatal("song list has no frame")
	}
	for _, row := range p.list.Contents {
		radio := row.Contents[0]
		if radio.ItemType != eui.ITEM_RADIO || radio.RadioGroup != "bard-song" {
			t.Fatal("song row is not a radio selection")
		}
		if strings.HasPrefix(radio.Text, "Other song") {
			radio.Handler.Emit(eui.UIEvent{Type: eui.EventRadioSelected, Checked: true})
			break
		}
	}
	selected := 0
	for _, row := range p.list.Contents {
		if row.Contents[0].Checked {
			selected++
		}
	}
	if selected != 1 || p.selected != other.Path || !strings.Contains(p.details.Text, "Selected song: Other song") || !session.commands.idle() {
		t.Fatal("selection did not identify exactly one song without playing it")
	}
	p.layout()
	windowSize, listSize, statusSize := p.win.Size, p.list.Size, p.statusFrame.GetSize()
	p.setError(fmt.Errorf("%s", strings.Repeat("A long error message. ", 100)))
	p.layout()
	if p.status.Invisible || p.win.Size != windowSize || p.list.Size != listSize || p.statusFrame.GetSize() != statusSize || p.statusFrame.Color != eui.ColorDarkRed {
		t.Fatal("an error changed the window/list geometry or failed to show a red status")
	}
	p.setStatus("Check passed.", false)
	p.layout()
	if p.list.Size != listSize || p.statusFrame.GetSize() != statusSize || !strings.Contains(p.status.Text, "Status: OK") {
		t.Fatal("success feedback changed the status bar geometry")
	}
	p.editTune()
	path, _ := filepath.Abs(p.selected)
	ed := sourceEditors[path]
	ed.layout()
	inputSize := ed.input.Size
	if ed.statusFrame == nil || ed.status.Invisible || !ed.check() || ed.input.Size != inputSize {
		t.Fatal("tune editor did not reserve its status bar before checking")
	}
	editMacroForTest(ed, "?")
	if ed.check() || ed.input.Size != inputSize || ed.statusFrame.Color != eui.ColorDarkRed {
		t.Fatal("failed editor check changed text-area geometry or lost the error color")
	}
}

func TestBardPartnerNameCompletion(t *testing.T) {
	session := mustNewSession(primarySessionID)
	session.setCharacterName("Flutist")
	for _, name := range []string{"Blue", "Pixy", "Flutist", "Example d'Exile"} {
		session.players.players[name] = &Player{Name: name}
	}
	session.players.players["Blue NPC"] = &Player{Name: "Blue NPC", IsNPC: true}
	for _, test := range []struct{ input, suffix string }{
		{"Bl", "ue"}, {"Blue, Pi", "xy"}, {"Blue, Example d'", "Exile"},
		{"Blue, Bl", ""}, {"Blu, Bl", ""}, {"Flu", ""}, {"Blue N", ""},
		{"", ""}, {"Blue, ", ""}, {"Blue, Pixy, Ex", ""}, {"Nobody", ""},
	} {
		if got := bardPartnerCompletion(session, test.input); got != test.suffix {
			t.Errorf("completion for %q = %q, want %q", test.input, got, test.suffix)
		}
	}
	if got := bardPartnerCompletion(nil, "Bl"); got != "" {
		t.Fatal("completion leaked another session's players")
	}
}

func TestBardPartSelectionStaysVisibleAndSavesTheSelectedPart(t *testing.T) {
	p, session := bardReadyPanel(t)
	p.showEnsembleWindow()
	if p.part.ParentWindow != p.win || p.part.Invisible || p.sharing.part.ParentWindow != p.sharing.win {
		t.Fatal("opening ensemble setup moved the main part control")
	}
	p.sharing.part.Selected = 1
	p.sharing.part.Handler.Emit(eui.UIEvent{Type: eui.EventDropdownSelected})
	if p.part.Selected != 1 || p.selectedPart != "Accompaniment" || p.instrument.Selected != 0 {
		t.Fatal("ensemble selection did not update the main part and instrument")
	}
	p.instrument.Selected = 2
	p.instrument.Handler.Emit(eui.UIEvent{Type: eui.EventDropdownSelected})
	value, err := readBardTune(p.selected)
	if err != nil {
		t.Fatal(err)
	}
	score, err := parseBardScore(value, 0)
	if err != nil || score.Parts[0].Instrument != 17 || score.Parts[1].Instrument != 2 {
		t.Fatal("instrument picker changed the wrong part", err)
	}
	if !strings.Contains(p.status.Text, "Saved Starbuck Harp for Accompaniment") || !strings.Contains(p.sharing.part.Options[1], "Starbuck Harp") {
		t.Fatal("saved instrument was not reflected in feedback and ensemble selection")
	}
	p.part.Selected = 0
	p.part.Handler.Emit(eui.UIEvent{Type: eui.EventDropdownSelected})
	if p.sharing.part.Selected != 0 || p.instrument.Selected != 17 {
		t.Fatal("main selection did not update ensemble setup")
	}
	p.sharing.win.Close()
	if p.part.ParentWindow != p.win || p.part.Invisible || !session.commands.idle() {
		t.Fatal("closing ensemble setup hid the main part or sent game commands")
	}
}
