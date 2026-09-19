package main

import (
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	scriptapi "gt2"
)

func bardAdditionalSession(t *testing.T, name string) *Session {
	t.Helper()
	session, ok := appSessions.addSession()
	if !ok {
		t.Fatal("could not add session")
	}
	client, server := net.Pipe()
	session.transport.attach(client, client)
	session.setCharacterName(name)
	t.Cleanup(func() { session.transport.disconnect(); server.Close() })
	return session
}

func TestBardThreeSessionPlaybackAndSelection(t *testing.T) {
	first, one := bardReadyPanel(t)
	two := bardAdditionalSession(t, "Lyrist")
	three := bardAdditionalSession(t, "Bassist")
	two.inventory.add(222, -1, "Lucky Lyra", false)
	three.inventory.add(333, -1, "Gutbucket Bass", false)
	trio, err := createBardTune("trio", bardDuetFixture+"<@part: Bass>\n<@instrument: Gutbucket Bass>\n@90 cde\n")
	if err != nil {
		t.Fatal(err)
	}
	sessions := []*Session{one, two, three}
	parts := []string{"Melody", "Accompaniment", "Bass"}
	partners := []string{"Lyrist, Bassist", "Flutist, Bassist", "Flutist, Lyrist"}
	var panels []*bardPanel
	for i, session := range sessions {
		appSessions.selectSession(session.ID())
		updateBardWindow()
		p := bardWindow
		panels = append(panels, p)
		p.reload()
		p.selected, p.selectedPart, p.partners.Text = trio.Path, parts[i], partners[i]
		p.refreshSelection()
		p.play()
		if p.playConfirm == nil {
			t.Fatal("missing confirmation", p.statusMessage)
		}
		clickMacroEditorButton(t, p.playConfirm, "Play in Game")
		if p.performance == nil || p.performance.session != session {
			t.Fatal("wrong playback session", p.statusMessage)
		}
	}
	if panels[0] != first {
		t.Fatal("lost first session's panel")
	}
	// All three are still preparing, including the two background sessions.
	for i, p := range panels {
		perf := p.performance
		if perf == nil {
			t.Fatalf("switching stopped performer %d", i)
		}
		s := sessions[i]
		s.inventory.equip(perf.instrument.ID, -1, true)
		s.commands.mu.Lock()
		perf.tickets[0].state.status.State = scriptapi.CommandSent
		s.commands.mu.Unlock()
		s.commands.clear()
	}
	updateBardWindow()
	for i, p := range panels {
		if !p.performance.submitted {
			t.Fatalf("background performer %d did not advance", i)
		}
		commands := strings.Join(bardQueueTexts(sessions[i]), "\n")
		with, err := bardWithOptions(strings.Split(partners[i], ","))
		if err != nil || !strings.Contains(commands, "/use "+with) {
			t.Fatalf("wrong session's partners: %q", commands)
		}
	}
	appSessions.selectSession(one.ID())
	updateBardWindow()
	if bardWindow != first || first.selected != trio.Path || first.selectedPart != "Melody" || first.partners.Text != partners[0] || !strings.Contains(first.win.Title, "Flutist") {
		t.Fatal("session selection was not restored")
	}
	for i, p := range panels {
		if p.win.IsOpen() != (i == 0) {
			t.Fatal("more than the selected Bard panel is visible")
		}
	}
	otherCommands := bardQueueTexts(two)
	clickMacroEditorButton(t, first.win, "Stop Playing")
	if first.performance != nil || panels[1].performance == nil || panels[2].performance == nil || !reflect.DeepEqual(bardQueueTexts(two), otherCommands) {
		t.Fatal("Stop Playing affected another session")
	}
	first.win.Close()
	if bardWindow != nil || len(bardPanels) != 0 || panels[1].performance != nil || panels[2].performance != nil {
		t.Fatal("closing Bard left background performances running")
	}
}

func TestBardBackgroundSharingIsSessionScoped(t *testing.T) {
	first, one, messages := bardShareFixture(t)
	first.sharing.assignments[0].Selected = 1
	clickMacroEditorButton(t, first.sharing.win, "Send Parts")
	if len(first.sharing.tickets) == 0 {
		t.Fatal(first.statusMessage)
	}
	queued := bardQueueTexts(one)
	two := bardAdditionalSession(t, "Second")
	updateBardWindow()
	second := bardWindow
	second.selected, second.selectedPart = first.selected, "Melody"
	second.refreshSelection()
	second.partners.Text = "Stranger"
	second.showEnsembleWindow()
	if second.sharing.receive.Checked || !first.sharing.receive.Checked || !reflect.DeepEqual(queued, bardQueueTexts(one)) {
		t.Fatal("switching changed another session's sending or receive consent")
	}
	before := second.selected
	// Exercise actual server dispatch, including the background-session route.
	for _, message := range messages {
		if _, _, err := parseSessionDrawState(one, buildDrawData("Blue", kBubbleThought, "Blue to you: "+message), false); err != nil {
			t.Fatal(err)
		}
	}
	drainMainThreadDispatcher()
	if first.selected == before || second.selected != before || bardWindow != second || first.win.IsOpen() || !two.commands.idle() {
		t.Fatal("background transfer changed or played the visible session")
	}
	appSessions.selectSession(one.ID())
	updateBardWindow()
	first.showEnsembleWindow()
	if bardWindow != first || first.selectedPart != "Solo" || first.partners.Text != "Blue, Pixy" || !first.sharing.receive.Checked {
		t.Fatal("background received song or receive settings were not retained")
	}
}

func TestBardBackgroundDisconnectAndSlotReuse(t *testing.T) {
	first, one := bardReadyPanel(t)
	first.play()
	clickMacroEditorButton(t, first.playConfirm, "Play in Game")
	oldPerformance := first.performance
	two := bardAdditionalSession(t, "Second")
	updateBardWindow()
	second := bardWindow
	one.transport.mu.Lock()
	one.transport.generation++
	one.transport.mu.Unlock()
	updateBardWindow()
	if first.performance != nil || !first.statusProblem || bardWindow != second || !two.commands.idle() {
		t.Fatal("background reconnect did not stop only the stale performance")
	}
	if oldPerformance.tickets[0].Status().State == scriptapi.CommandQueued {
		t.Fatal("stale ticket survived reconnect")
	}
	// Removing a tab discards its panel; reusing the slot must start fresh.
	if !appSessions.closeSession(one.ID()) {
		t.Fatal("could not close first tab")
	}
	updateBardWindow()
	if first.registered() {
		t.Fatal("removed session retained its Bard state")
	}
	replacement, ok := appSessions.addSession()
	if !ok {
		t.Fatal("could not reuse tab")
	}
	updateBardWindow()
	if bardWindow.session != replacement || bardWindow == first || bardWindow.partners.Text != "" || bardWindow.performance != nil {
		t.Fatal("reused tab inherited another connection's Bard state")
	}
}

func TestBardBackgroundInstrumentStorageContinues(t *testing.T) {
	first, one := bardReadyPanel(t)
	one.inventory.add(10, -1, "Instrument Case", false)
	first.storeInstruments()
	op := first.storage
	if op == nil {
		t.Fatal(first.statusMessage)
	}
	bardAdditionalSession(t, "Second")
	updateBardWindow()
	if first.storage != op || bardWindow.storage != nil {
		t.Fatal("switching lost or moved instrument storage")
	}
	op.deadline = time.Now().Add(-time.Second)
	updateBardWindow()
	if first.storage != nil || !first.statusProblem || bardWindow.statusProblem {
		t.Fatal("storage error was not scoped to its session")
	}
}
