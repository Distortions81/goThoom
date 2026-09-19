package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	scriptapi "gt2"
)

func bardConfirmCommand(t *testing.T, session *Session, ticket CommandTicket, want string) {
	t.Helper()
	if got := bardQueueTexts(session); !reflect.DeepEqual(got, []string{want}) {
		t.Fatalf("commands = %v, want %s", got, want)
	}
	session.commands.mu.Lock()
	ticket.state.status.State = scriptapi.CommandSent
	session.commands.mu.Unlock()
	session.commands.clear()
}

func bardCaseUpdate(t *testing.T, op *bardCaseOperation) {
	t.Helper()
	if err := op.update(time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestBardCaseRetrievesBeforePerformance(t *testing.T) {
	for _, full := range []bool{false, true} {
		t.Run(fmt.Sprint(full), func(t *testing.T) {
			s := bardConnectedSession(t)
			s.inventory.add(10, -1, "Instrument Case", false)
			if full {
				s.inventory.add(11, -1, "Lucky Lyra", true)
				s.inventory.add(12, -1, "Starbuck Harp", false)
				for n := 3; n < inventoryMaxSlots; n++ {
					s.inventory.add(20, -1, "Pebble", false)
				}
			}
			p, err := startBardPerformance(s, "cde", 17)
			if err != nil {
				t.Fatal(err)
			}
			op := p.preparation
			if op == nil {
				t.Fatal("no case preparation")
			}
			s.inventory.equip(10, -1, true)
			bardCaseUpdate(t, op)
			if op.stage != "equip case" {
				t.Fatal("advanced before command write")
			}
			bardConfirmCommand(t, s, op.pending, "/equip 10")
			bardCaseUpdate(t, op)
			if full {
				bardConfirmCommand(t, s, op.pending, "/useitem instrument case /add Starbuck Harp")
				bardCaseUpdate(t, op)
				if op.stage != "store" {
					t.Fatal("retrieved before storage confirmation")
				}
				s.inventory.remove(12, -1)
				bardCaseUpdate(t, op)
			}
			bardConfirmCommand(t, s, op.pending, "/useitem instrument case /remove Pine Flute")
			bardCaseUpdate(t, op)
			if op.stage != "retrieve" || p.submitted {
				t.Fatal("advanced without the instrument")
			}
			s.inventory.add(321, -1, "Pine Flute", false)
			bardCaseUpdate(t, op)
			bardConfirmCommand(t, s, op.pending, "/unequip 10")
			s.inventory.equip(10, -1, false)
			if err := p.update(time.Now()); err != nil {
				t.Fatal(err)
			}
			if p.preparation != nil || p.instrument.ID != 321 || p.submitted {
				t.Fatal("did not begin equipping retrieved flute")
			}
			bardConfirmCommand(t, s, p.tickets[0], "/equip 321")
			if err := p.update(time.Now()); err != nil || p.submitted {
				t.Fatal("played before equip confirmation", err)
			}
			s.inventory.equip(321, -1, true)
			if err := p.update(time.Now()); err != nil || !p.submitted {
				t.Fatal("did not perform", err)
			}
		})
	}
}

func TestBardCaseStoresAllCopiesAndRestoresLeftHand(t *testing.T) {
	s := bardConnectedSession(t)
	s.inventory.add(10, -1, "Instrument Case", false)
	s.inventory.add(50, -1, "Shield", true)
	s.inventory.mu.Lock()
	s.inventory.items[1].Slot = "left-hand"
	s.inventory.mu.Unlock()
	s.inventory.add(321, -1, "Pine Flute", false)
	s.inventory.add(321, -1, "Pine Flute", false)
	s.inventory.add(222, 0, "Lucky Lyra", true)
	s.inventory.rename(222, 0, "My Lyra")
	op, err := startBardCaseOperation(s, -1)
	if err != nil {
		t.Fatal(err)
	}
	bardConfirmCommand(t, s, op.pending, "/equip 10")
	s.inventory.equip(50, -1, false)
	s.inventory.equip(10, -1, true)
	bardCaseUpdate(t, op)
	for _, item := range []struct {
		id   uint16
		idx  int
		name string
	}{{222, 0, "Lucky Lyra"}, {321, -1, "Pine Flute"}, {321, -1, "Pine Flute"}} {
		bardConfirmCommand(t, s, op.pending, "/useitem instrument case /add "+item.name)
		s.inventory.remove(item.id, item.idx)
		bardCaseUpdate(t, op)
	}
	bardConfirmCommand(t, s, op.pending, "/equip 50")
	bardCaseUpdate(t, op)
	if op.done {
		t.Fatal("did not wait for restored equipment")
	}
	s.inventory.equip(10, -1, false)
	s.inventory.equip(50, -1, true)
	bardCaseUpdate(t, op)
	if !op.done || len(s.inventory.snapshot()) != 2 {
		t.Fatal("storage incomplete")
	}
}

func TestBardCaseFailuresDoNotSendMusic(t *testing.T) {
	for _, failure := range []string{"full without instruments", "store timeout", "retrieve timeout", "reconnect", "case removed", "cancel"} {
		t.Run(failure, func(t *testing.T) {
			s := bardConnectedSession(t)
			s.inventory.add(10, -1, "Instrument Case", true)
			if failure == "full without instruments" || failure == "store timeout" {
				for n := 1; n < inventoryMaxSlots; n++ {
					s.inventory.add(20, -1, "Pebble", false)
				}
				if failure == "store timeout" {
					s.inventory.remove(20, -1)
					s.inventory.add(222, -1, "Lucky Lyra", false)
				}
			}
			p, err := startBardPerformance(s, "cde", 17)
			if failure == "full without instruments" {
				if err == nil || !s.commands.idle() {
					t.Fatal("full pack without instruments accepted", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			op := p.preparation
			bardConfirmCommand(t, s, op.pending, "/equip 10")
			bardCaseUpdate(t, op)
			now := time.Now()
			switch failure {
			case "store timeout", "retrieve timeout":
				now = now.Add(11 * time.Second)
			case "reconnect":
				s.transport.mu.Lock()
				s.transport.generation++
				s.transport.mu.Unlock()
			case "case removed":
				s.inventory.remove(10, -1)
			case "cancel":
				op.cancel()
			}
			if err := p.update(now); err == nil {
				t.Fatal("operation did not fail")
			}
			got := strings.Join(bardQueueTexts(s), "|")
			if strings.Contains(got, "cde") || strings.Contains(got, "/remove") || strings.Contains(got, "/add") {
				t.Fatal("failure left transfer or music queued", got)
			}
			if failure == "reconnect" && got != "" {
				t.Fatal("sent to new connection", got)
			}
		})
	}
}

func TestBardCasePanelAvailabilityAndCancellation(t *testing.T) {
	p, s := bardReadyPanel(t)
	s.inventory.remove(321, -1)
	s.inventory.add(10, -1, "Instrument Case", false)
	p.refreshSelection()
	if p.playButton.Disabled || !strings.Contains(p.instrument.Options[17], "try case") {
		t.Fatal("case instrument unavailable")
	}
	p.play()
	clickMacroEditorButton(t, p.playConfirm, "Play in Game")
	if p.performance == nil || p.performance.preparation == nil {
		t.Fatal("confirmation did not retrieve")
	}
	clickMacroEditorButton(t, p.win, "Stop Playing")
	s.commands.clear()
	s.inventory.add(321, -1, "Pine Flute", false)
	p.storeInstruments()
	if p.storage == nil || p.stopButton.Disabled || !p.playButton.Disabled {
		t.Fatal("missing storage state")
	}
	s.commands.enqueue("/pose sit")
	p.win.Close()
	if got := bardQueueTexts(s); !reflect.DeepEqual(got, []string{"/pose sit"}) {
		t.Fatal("close canceled wrong commands", got)
	}
}
