package main

import (
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"gothoom/eui"
	scriptapi "gt2"
)

func bardFixture(t *testing.T) {
	t.Helper()
	macroEditorFixture(t)
	old, oldPanels := bardWindow, bardPanels
	bardWindow = nil
	bardPanels = make(map[*Session]*bardPanel)
	t.Cleanup(func() {
		if bardWindow != nil {
			bardWindow.win.Close()
		}
		bardWindow, bardPanels = old, oldPanels
	})
}
func TestBardLibraryUnicodeEditorAndPreference(t *testing.T) {
	bardFixture(t)
	value := "<Café — 月>\n@120 (cdefgab/c)2\n"
	tune, err := createBardTune("Café", value)
	if err != nil {
		t.Fatal(err)
	}
	if err = saveBardInstrument(tune, 17); err != nil {
		t.Fatal(err)
	}
	value = "<@instrument: Pine Flute>\n" + value
	tunes, err := listBardTunes()
	if err != nil || len(tunes) != 1 || tunes[0].Instrument != 17 {
		t.Fatalf("library: %+v %v", tunes, err)
	}
	if _, err = createBardTune("CAFÉ", "different"); err == nil {
		t.Fatal("overwrote existing tune")
	}
	for _, name := range []string{"", "../outside", "NUL", "CON.txt", "a/b"} {
		if _, err = createBardTune(name, ""); err == nil {
			t.Fatalf("accepted filename %q", name)
		}
	}
	showBardWindow()
	p := bardWindow
	p.selected = tune.Path
	p.refreshSelection()
	p.editTune()
	path, _ := filepath.Abs(tune.Path)
	ed := sourceEditors[path]
	if ed == nil || ed.doc.macRoman || ed.input.Text != value || ed.options.highlight == nil {
		t.Fatal("missing Unicode editor/highlighting")
	}
	p.instrument.Selected = 2
	p.instrument.Handler.Handle(eui.UIEvent{Type: eui.EventDropdownSelected})
	value = strings.Replace(value, "<@instrument: Pine Flute>", "<@instrument: Starbuck Harp>", 1)
	if ed.input.Text != value || ed.dirty() {
		t.Fatal("instrument change did not update the clean editor")
	}
	editMacroForTest(ed, value+"C4\n")
	if !ed.save(false) {
		t.Fatal(ed.message)
	}
	saved, err := readBardTune(tune.Path)
	if err != nil || saved != value+"C4\n" {
		t.Fatalf("save: %q %v", saved, err)
	}
	if !p.playButton.Disabled || p.previewButton.Disabled {
		t.Fatal("offline actions wrong")
	}
	if len(highlightBardTune(value, eui.DefaultSyntaxColors(eui.ColorBlack))) == 0 {
		t.Fatal("no highlighting")
	}
	bad := filepath.Join(bardTunesDir(), "bad.txt")
	if err := os.WriteFile(bad, []byte{0x8e}, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readBardTune(bad); err == nil {
		t.Fatal("accepted MacRoman tune")
	}
}
func TestBardDeleteTuneConfirmation(t *testing.T) {
	for _, withPreference := range []bool{false, true} {
		t.Run(map[bool]string{false: "no preference", true: "saved preference"}[withPreference], func(t *testing.T) {
			bardFixture(t)
			tune, err := createBardTune("Café", "cdef")
			if err != nil {
				t.Fatal(err)
			}
			if withPreference {
				if err := os.WriteFile(tune.Path+".json", []byte(`{"instrument":17}`), 0644); err != nil {
					t.Fatal(err)
				}
			}
			other, err := createBardTune("Keep me", "gab")
			if err != nil {
				t.Fatal(err)
			}
			showBardWindow()
			p := bardWindow
			p.selected = tune.Path
			p.refreshSelection()
			p.editTune()
			path, _ := filepath.Abs(tune.Path)
			ed := sourceEditors[path]
			editMacroForTest(ed, "cdefgab")
			popup := p.confirmDeleteTune(tune)
			if text, err := readBardTune(tune.Path); err != nil || text != "cdef" {
				t.Fatalf("tune changed before confirmation: %q, %v", text, err)
			}
			clickMacroEditorButton(t, popup, "Cancel")
			if _, err := os.Stat(tune.Path); err != nil || !ed.win.IsOpen() || !ed.dirty() {
				t.Fatal("cancel changed the tune or draft", err)
			}
			popup = p.confirmDeleteTune(tune)
			// The prompt must keep targeting its own tune if selection changes.
			p.selected = other.Path
			clickMacroEditorButton(t, popup, "Delete")
			for _, path := range []string{tune.Path, tune.Path + ".json"} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("deleted file remains: %s: %v", path, err)
				}
			}
			if sourceEditors[path] != nil || ed.win.IsOpen() {
				t.Fatal("deleted tune editor remains open")
			}
			if len(p.tunes) != 1 || p.tunes[0].Path != other.Path || p.selected != other.Path || p.statusProblem {
				t.Fatal("deletion did not preserve the other tune and selection")
			}
			popup = p.confirmDeleteTune(other)
			clickMacroEditorButton(t, popup, "Delete")
			if len(p.tunes) != 0 || p.selected != "" || !p.edit.Disabled || !p.previewButton.Disabled {
				t.Fatal("deleting the selected tune did not clear selection")
			}
		})
	}
}

func TestBardDeleteTuneFailure(t *testing.T) {
	bardFixture(t)
	tune, err := createBardTune("Keep me", "cdef")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tune.Path+".json", []byte(`{"instrument":17}`), 0644); err != nil {
		t.Fatal(err)
	}
	showBardWindow()
	p := bardWindow
	p.selected = tune.Path
	p.editTune()
	path, _ := filepath.Abs(tune.Path)
	ed := sourceEditors[path]
	// A nonempty directory at the file path makes deletion fail on all platforms.
	if err := os.Remove(tune.Path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(tune.Path, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tune.Path, "keep"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	clickMacroEditorButton(t, p.confirmDeleteTune(tune), "Delete")
	if p.status.Invisible || !ed.win.IsOpen() || p.selected != tune.Path {
		t.Fatal("failed deletion did not report the error and preserve the editor")
	}
	if bardSavedInstrument(tune.Path) != 17 {
		t.Fatal("failed deletion removed the instrument preference")
	}
}

func TestBardValidationAndMultipartPreservesMusic(t *testing.T) {
	for _, value := range []string{"@120 (c#4d.e_f2|1g|2a)2", "[ceg]4C4D4", "<月> c<comment>#4d", "[ceg]$p8[ceg]$C"} {
		if _, err := validateBardTune(value, 7); err != nil {
			t.Fatalf("valid %q: %v", value, err)
		}
	}
	for _, value := range []string{"", "<unfinished", "(cde", "[ceg", "c\n/use /give all", "C\x00", "|1c", "((((((((c)9)9)9)9)9)9)9)9", "((" + strings.Repeat(" ", 20000) + ")9)9c"} {
		if _, err := validateBardTune(value, 0); err == nil {
			t.Fatalf("accepted %q", value[:min(len(value), 80)])
		}
	}
	if _, err := validateBardTune("[ceg]C", 17); err == nil {
		t.Fatal("flute accepted chords")
	}
	tune := "<Unicode title: 雪>\n@130 " + strings.Repeat("(c#4d.e_f2g|1a|2b)2 ", 65)
	want, err := validateBardTune(tune, 2)
	if err != nil {
		t.Fatal(err)
	}
	commands, err := bardTuneCommands(tune)
	if err != nil || len(commands) < 2 {
		t.Fatalf("split: %v %v", commands, err)
	}
	var pieces []string
	for i, cmd := range commands {
		if len(cmd) > 511 || strings.ContainsAny(cmd, "\r\n") {
			t.Fatalf("invalid command %q", cmd)
		}
		prefix := "/use /part "
		if i == len(commands)-1 {
			prefix = "/use "
		}
		if !strings.HasPrefix(cmd, prefix) {
			t.Fatal(cmd)
		}
		pieces = append(pieces, strings.TrimPrefix(cmd, prefix))
	}
	got, parseErr := parseClassicTune(strings.Join(pieces, " "), instruments[2], 120, 100)
	if parseErr != nil || !reflect.DeepEqual(want, got) {
		t.Fatal("splitting changed notes or timing", parseErr)
	}
	if _, err := bardTuneCommands(strings.Repeat("c", 2501)); err == nil {
		t.Fatal("accepted six parts")
	}
	if _, err := bardTuneCommands("c" + strings.Repeat("#", 510)); err == nil {
		t.Fatal("accepted oversized token")
	}
}
func bardConnectedSession(t *testing.T) *Session {
	t.Helper()
	s := mustNewSession(primarySessionID)
	client, server := net.Pipe()
	t.Cleanup(func() { s.transport.disconnect(); server.Close() })
	s.transport.attach(client, client)
	return s
}
func bardQueueTexts(s *Session) []string {
	s.commands.mu.Lock()
	defer s.commands.mu.Unlock()
	var out []string
	if s.commands.pending != "" {
		out = append(out, s.commands.pending)
	}
	for _, c := range s.commands.queue {
		out = append(out, c.text)
	}
	return out
}
func TestBardPerformanceEquipmentStopAndReconnect(t *testing.T) {
	s := bardConnectedSession(t)
	s.inventory.add(321, -1, "Pine Flute", false)
	if _, ok := bardOwnedInstrument(s, 17); !ok {
		t.Fatal("missing flute")
	}
	if bardInstrumentIndex("Mammoth Violène") != 20 {
		t.Fatal("accented instrument not recognized")
	}
	p, err := startBardPerformance(s, "cdef", 17)
	if err != nil {
		t.Fatal(err)
	}
	if got := bardQueueTexts(s); !reflect.DeepEqual(got, []string{"/equip 321"}) {
		t.Fatal(got)
	}
	if err = p.update(time.Now()); err != nil || p.submitted {
		t.Fatal("played without equipment confirmation", err)
	}
	s.inventory.equip(321, -1, true)
	if err = p.update(time.Now()); err != nil || p.submitted {
		t.Fatal("played before equip command write", err)
	}
	s.commands.mu.Lock()
	p.tickets[0].state.status.State = scriptapi.CommandSent
	s.commands.mu.Unlock()
	s.commands.clear() // Simulate the equip acknowledgement.
	if err = p.update(time.Now()); err != nil || !p.submitted {
		t.Fatal("did not play after confirmation", err)
	}
	s.commands.enqueue("/pose sit")
	p.stop()
	got := strings.Join(bardQueueTexts(s), "|")
	if strings.Contains(got, "cdef") || !strings.Contains(got, "/pose sit") || !strings.HasSuffix(got, "/use /stop") {
		t.Fatal("stop affected wrong commands", got)
	}
	s.commands.clear()
	p, err = startBardPerformance(s, "cdef", 17)
	if err != nil {
		t.Fatal(err)
	}
	s.transport.mu.Lock()
	s.transport.generation++
	s.transport.mu.Unlock()
	p.stop()
	if got := bardQueueTexts(s); len(got) != 0 {
		t.Fatal("old performance sent commands to new connection", got)
	}
}
func TestBardPerformanceRejectsBeforeSending(t *testing.T) {
	s := bardConnectedSession(t)
	s.inventory.add(321, -1, "Pine Flute", false)
	for _, value := range []string{"c?", strings.Repeat("c", 2501)} {
		if _, err := startBardPerformance(s, value, 17); err == nil {
			t.Fatal("accepted invalid tune")
		}
		if len(bardQueueTexts(s)) != 0 {
			t.Fatal("queued before validation")
		}
	}
	if _, err := startBardPerformance(s, "cde", 2); err == nil {
		t.Fatal("performed without instrument")
	}
	p, err := startBardPerformance(s, "cde", 17)
	if err != nil {
		t.Fatal(err)
	}
	if err = p.update(time.Now().Add(11 * time.Second)); err == nil {
		t.Fatal("no equip timeout")
	}
	if strings.Contains(strings.Join(bardQueueTexts(s), " "), "cde") {
		t.Fatal("timeout sent tune")
	}
}
func TestBardPreviewRejectsWithoutServerCommands(t *testing.T) {
	bardFixture(t)
	gsBefore := gs
	t.Cleanup(func() { gs = gsBefore })
	gs.Music = false
	if _, err := startBardPreview("cde", 17, nil); err == nil {
		t.Fatal("preview ignored muted music")
	}
	if !selectedAppSession().commands.idle() {
		t.Fatal("preview sent server commands")
	}
}
func TestRenderBardTool(t *testing.T) {
	dir := os.Getenv("GOTHOOM_RENDER_BARD")
	if dir == "" {
		t.Skip("set GOTHOOM_RENDER_BARD to a capture directory")
	}
	bardFixture(t)
	if err := eui.LoadTheme("AccentDark"); err != nil {
		t.Fatal(err)
	}
	tune, err := createBardTune("A walk through Puddleby", "<A walk through Puddleby>\n@120 (cdef gab/c)2\n")
	if err != nil {
		t.Fatal(err)
	}
	createBardTune("Moonlight at the docks", "@90 c4e4g8")
	// Use a display title covered by the bundled UI font; the score tests keep
	// the original Unicode fixture to exercise metadata preservation.
	renderDuet := strings.Replace(bardDuetFixture, "Moonlight — 月", "Moonlight duet", 1)
	duet, err := createBardTune("Moonlight duet", renderDuet)
	if err != nil {
		t.Fatal(err)
	}
	session := bardConnectedSession(t)
	appSessions = newSessionManager(session)
	session.setCharacterName("Flutist")
	session.inventory.add(321, -1, "Pine Flute", false)
	session.players.players["Blue"] = &Player{Name: "Blue"}
	showBardWindow()
	p := bardWindow
	p.selected = tune.Path
	p.refreshSelection()
	p.refreshList()
	p.selected = duet.Path
	p.selectedPart = ""
	p.refreshSelection()
	p.editTune()
	path, _ := filepath.Abs(duet.Path)
	ed := sourceEditors[path]
	ed.check()
	p.partners.Text = "Blue"
	p.play()
	confirmation := p.playConfirm
	if confirmation == nil {
		t.Fatalf("no confirmation: %s", p.status.Text)
	}
	confirmation.Open = false
	bardVisiblePlayersFixture(t, session)
	p.showPartnerPicker()
	partnerPicker := p.partnerPicker
	partnerPicker.Open = false
	p.showEnsembleWindow()
	ensembleWindow := p.sharing.win
	ensembleWindow.Open = false
	for _, editor := range sourceEditors {
		editor.win.Open = false
	}
	selectTune := func(path string) func() {
		return func() {
			p.moreActions.Invisible, p.moreButton.Text = true, "More…"
			p.selected, p.selectedPart = path, ""
			p.refreshSelection()
			p.refreshList()
			p.partners.Text = "Bl"
			eui.Focus(p.partners)
		}
	}
	game := &notesEditorRenderGame{dir: dir, scenes: []notesEditorScene{
		{"bard", p.win, 640, 520, selectTune(tune.Path)},
		{"bard-narrow", p.win, 400, 520, selectTune(tune.Path)},
		{"bard-ensemble", p.win, 640, 520, selectTune(duet.Path)},
		{"bard-ensemble-narrow", p.win, 400, 520, selectTune(duet.Path)},
		{"bard-more-narrow", p.win, 400, 520, func() {
			selectTune(duet.Path)()
			clickMacroEditorButton(t, p.win, "More…")
		}},
		{"bard-confirm", confirmation, 0, 0, nil},
		{"bard-partners", partnerPicker, 420, 360, partnerPicker.OnResize},
		{"bard-duet-trio", ensembleWindow, 520, 500, ensembleWindow.OnResize},
		{"bard-duet-trio-narrow", ensembleWindow, 360, 500, ensembleWindow.OnResize},
		{"tune-editor", ed.win, 780, 540, ed.layout},
	}}
	if os.Getenv("GOTHOOM_RENDER_ENSEMBLE_ONLY") != "" {
		var scenes []notesEditorScene
		for _, scene := range game.scenes {
			if strings.HasPrefix(scene.name, "bard-duet-trio") {
				scenes = append(scenes, scene)
			}
		}
		game.scenes = scenes
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}
