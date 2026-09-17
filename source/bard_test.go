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
	old := bardWindow
	bardWindow = nil
	t.Cleanup(func() {
		if bardWindow != nil {
			bardWindow.win.Close()
		}
		bardWindow = old
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
	if ed.input.Text != value || ed.dirty() {
		t.Fatal("instrument change modified draft")
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
	selectedAppSession().inventory.add(321, -1, "Pine Flute", false)
	showBardWindow()
	p := bardWindow
	p.selected = tune.Path
	p.refreshSelection()
	p.refreshList()
	p.editTune()
	path, _ := filepath.Abs(tune.Path)
	ed := sourceEditors[path]
	ed.win.Open = false
	game := &notesEditorRenderGame{dir: dir, scenes: []notesEditorScene{{"bard", p.win, 640, 520, p.refreshList}, {"bard-narrow", p.win, 400, 520, p.refreshList}, {"tune-editor", ed.win, 780, 540, ed.layout}}}
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
