package main

import (
	"strings"
	"testing"
	"time"

	scriptapi "gt2"
)

func bardWatchingPerformance(t *testing.T) (*bardPanel, *Session) {
	t.Helper()
	p, s := bardReadyPanel(t)
	oldSettings, oldBlock, oldCapture := gs, blockMusic, movieMusicIndexCapture
	gs.Mute, blockMusic, movieMusicIndexCapture = true, false, nil
	t.Cleanup(func() { gs, blockMusic, movieMusicIndexCapture = oldSettings, oldBlock, oldCapture })
	performance, err := startBardEnsemblePerformance(s, "c4p4", 17, []string{"Blue"})
	if err != nil {
		t.Fatal(err)
	}
	p.performance = performance
	s.inventory.equip(321, -1, true)
	s.commands.mu.Lock()
	performance.tickets[0].state.status.State = scriptapi.CommandSent
	s.commands.mu.Unlock()
	s.commands.clear()
	if err := performance.update(time.Now()); err != nil {
		t.Fatal(err)
	}
	if s.music.bardWatch != performance.watch || !performance.submitted {
		t.Fatal("submitting music did not start observation")
	}
	s.commands.mu.Lock()
	for _, ticket := range performance.tickets {
		ticket.state.status.State = scriptapi.CommandSent
	}
	s.commands.mu.Unlock()
	s.commands.clear()
	return p, s
}

func TestBardEnsembleStatusFollowsObservedGroupWhileMuted(t *testing.T) {
	p, s := bardWatchingPerformance(t)
	parse := func(command string) {
		t.Helper()
		if !parseSessionMusicCommand(s, command, nil) {
			t.Fatal("command not recognized", command)
		}
		if err := p.performance.update(time.Now()); err != nil {
			t.Fatal(err)
		}
		p.refreshStatus()
	}
	parse("/music/W42/S") // Initial stop clears old music, not this new performance.
	parse("/music/S")
	parse("/music/W99/P/I17/Nc4p4")
	if !p.performance.startedAt.IsZero() {
		t.Fatal("nearby music started our performance")
	}
	parse("/music/E/W42/P/I17/M/H7/Nc4")
	parse("/music/W42/P/I17/H7/Np4")
	if !p.performance.finishAt.IsZero() || !strings.Contains(p.status.Text, "Waiting") {
		t.Fatal("own final segment started an incomplete ensemble", p.status.Text)
	}
	parse("/music/W7/P/I2/H42/Nc4p4")
	if p.performance.startedAt.IsZero() || p.performance.finishAt.Sub(p.performance.startedAt) != time.Second || !strings.Contains(p.status.Text, "Playing with") {
		t.Fatal("completed group did not start or lost its trailing rest", p.status.Text)
	}
	if len(s.music.active) != 0 {
		t.Fatal("muted observation enabled audio tracks")
	}
	s.music.mu.Lock()
	p.performance.watch.started = time.Now().Add(-2 * time.Second)
	s.music.mu.Unlock()
	p.updateSession()
	if p.performance != nil || !strings.Contains(p.statusMessage, "finished") || s.music.bardWatch != nil {
		t.Fatal("observed performance did not finish", p.statusMessage)
	}
}

func TestBardEnsembleObservationIsScopedAndRecognizesStop(t *testing.T) {
	p, s := bardWatchingPerformance(t)
	other := bardAdditionalSession(t, "Other")
	parseSessionMusicCommand(other, "/music/E/W42/P/I17/Nc4p4", nil)
	parseSessionMusicCommand(s, "/music/W42/P/I17/N/E4p4", nil)
	if p.performance.watch.ownSeen {
		t.Fatal("another session or a high-octave E was mistaken for /me")
	}
	parseSessionMusicCommand(s, "/music/E/W42/P/I17/Nd4p4", nil)
	parseSessionMusicCommand(s, "/music/E/W42/P/I17/Nc4p8", nil)
	if !p.performance.watch.started.IsZero() {
		t.Fatal("different music was mistaken for the requested part")
	}
	parseSessionMusicCommand(s, "/music/E/W42/P/I17/Nc4p4", nil)
	parseSessionMusicCommand(s, "/music/W99/S", nil)
	if p.performance.watch.stopped {
		t.Fatal("another bard's stop ended our performance")
	}
	parseSessionMusicCommand(s, "/music/W42/S", nil)
	p.updateSession()
	if p.performance != nil || !strings.Contains(p.statusMessage, "stopped") {
		t.Fatal("server stop was not reflected in the panel", p.statusMessage)
	}
}
