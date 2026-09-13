package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	scriptapi "gt2"
)

// sessionAutomationState contains mutable automation data that belongs to one
// connection. The primary-session macro globals remain a UI compatibility
// adapter while shared macro-management windows are converted in a later UI
// slice.
type sessionAutomationState struct {
	legacyMu       sync.RWMutex
	legacySources  []legacyMacroSource
	legacyProgram  legacyMacroProgram
	legacyRuntime  *legacyMacroRuntime
	scriptQueues   *scriptQueueRegistry
	scriptTimers   *scriptTimerRegistry
	scriptMu       sync.RWMutex
	scripts        map[string]*sessionScriptInstance
	scriptChats    []structuredChatHandler
	scriptServers  []serverMessageHandler
	scriptEvents   []scriptLifecycleHandler
	scriptChanges  []scriptChangeHandler
	changeSnapshot scriptChangeSnapshot
	latestServer   scriptapi.ServerMessage
	serverSequence uint64
	hasServer      bool
	sendMu         sync.Mutex
	sendHistory    map[string][]time.Time

	locationMu sync.RWMutex
	location   string
}

func (s *Session) setScriptLocation(location string) {
	if s == nil || s.automation == nil {
		return
	}
	s.automation.locationMu.Lock()
	s.automation.location = strings.TrimSpace(location)
	s.automation.locationMu.Unlock()
}

func (s *Session) scriptLocationSnapshot() string {
	if s == nil || s.automation == nil {
		return ""
	}
	s.automation.locationMu.RLock()
	location := s.automation.location
	s.automation.locationMu.RUnlock()
	return location
}

func newSessionAutomationState() *sessionAutomationState {
	return &sessionAutomationState{
		scriptQueues: newScriptQueueRegistry(),
		scriptTimers: newScriptTimerRegistry(),
		scripts:      make(map[string]*sessionScriptInstance),
		sendHistory:  make(map[string][]time.Time),
	}
}

func (s *sessionAutomationState) recordScriptSend(owner string, now time.Time) int {
	if s == nil {
		return 0
	}
	cutoff := now.Add(-5 * time.Second)
	s.sendMu.Lock()
	times := s.sendHistory[owner]
	n := 0
	for _, sent := range times {
		if sent.After(cutoff) {
			times[n] = sent
			n++
		}
	}
	times = append(times[:n], now)
	s.sendHistory[owner] = times
	count := len(times)
	s.sendMu.Unlock()
	return count
}

func (s *sessionAutomationState) clearScriptSendHistory(owner string) {
	if s == nil {
		return
	}
	s.sendMu.Lock()
	delete(s.sendHistory, owner)
	s.sendMu.Unlock()
}

func (s *Session) loadLegacyMacrosForCharacter(character string) error {
	if s == nil || s.automation == nil {
		return nil
	}
	program, err := loadLegacyMacroProgramForCharacter(character)
	if err != nil {
		return err
	}
	runtime := newSessionLegacyMacroRuntime(s, program)
	runtime.startFunctionIfDefined("@login")
	s.automation.legacyMu.Lock()
	previous := s.automation.legacyRuntime
	s.automation.legacySources = append([]legacyMacroSource(nil), program.Files...)
	s.automation.legacyProgram = program
	s.automation.legacyRuntime = runtime
	s.automation.legacyMu.Unlock()
	if previous != nil {
		previous.cancelAll()
	}
	if diagnostics := runtime.diagnosticsSnapshot(); len(diagnostics) > 0 {
		return fmt.Errorf("legacy macro setup failed: %s", diagnostics[0])
	}
	return nil
}

func newSessionLegacyMacroRuntime(session *Session, program legacyMacroProgram) *legacyMacroRuntime {
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
		SendText: func(text string) { session.enqueueLegacyMacroCommand(text) },
		Message:  func(text string) { session.publishConsole(text, messageTextTypeSystem) },
		Move:     func(move legacyMacroMove) { session.queueLegacyMacroMove(move) },
		ResolveState: func(name string) (string, bool) {
			return session.legacyMacroStateValue(name)
		},
	})
	runtime.allowContinuous = gs.LegacyMacroContinuous
	return runtime
}

func (s *Session) enqueueLegacyMacroCommand(text string) {
	if s == nil || text == "" {
		return
	}
	// Local commands belong to the selected application UI. A background
	// session must never route one through the primary client state, so macro
	// output is always delivered through its own server-command stream.
	s.commands.enqueue(text)
}

func (s *Session) queueLegacyMacroMove(move legacyMacroMove) {
	if s == nil || s.input == nil {
		return
	}
	if move.Direction == legacyMacroMoveStop {
		s.input.enqueue(inputState{})
		return
	}
	dx, dy := 0, 0
	switch move.Direction {
	case legacyMacroMoveEast:
		dx = 1
	case legacyMacroMoveNorthEast:
		dx, dy = 1, -1
	case legacyMacroMoveNorth:
		dy = -1
	case legacyMacroMoveNorthWest:
		dx, dy = -1, -1
	case legacyMacroMoveWest:
		dx = -1
	case legacyMacroMoveSouthWest:
		dx, dy = -1, 1
	case legacyMacroMoveSouth:
		dy = 1
	case legacyMacroMoveSouthEast:
		dx, dy = 1, 1
	default:
		return
	}
	speed := gs.KBWalkSpeed
	if move.Run {
		speed = 1
	}
	s.input.enqueue(inputState{
		mouseX:    int16(float64(dx) * float64(fieldCenterX) * speed),
		mouseY:    int16(float64(dy) * float64(fieldCenterY) * speed),
		mouseDown: true,
	})
}

func (s *Session) legacyMacroStateValue(name string) (string, bool) {
	switch strings.ToLower(name) {
	case "@env.textlog":
		return s.latestEventText(), true
	case "@my.name":
		return s.characterName(), true
	case "@my.simple_name":
		return legacyMacroSimplePlayerName(s.characterName()), true
	case "@selplayer.name", "@selplayer.simple_name":
		// A background session has no shared-panel selection. It gains one when
		// Players.BindSession is introduced.
		return "", true
	case "@my.shares_in":
		return s.legacyMacroShares(false), true
	case "@my.shares_out":
		return s.legacyMacroShares(true), true
	case "@my.selected_item":
		return "Nothing", true
	}
	if slot, ok := legacyMacroItemSlots[strings.ToLower(name)]; ok {
		return s.legacyMacroEquippedItemName(slot), true
	}
	return "", false
}

func (s *Session) latestEventText() string {
	if s == nil || s.events == nil || s.events.log == nil {
		return ""
	}
	events := s.events.log.snapshot()
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Kind == sessionEventChat || events[i].Kind == sessionEventConsole {
			return events[i].Text
		}
	}
	return ""
}

func (s *Session) legacyMacroEquippedItemName(slot int) string {
	if clImages == nil || s == nil || s.inventory == nil {
		return "Nothing"
	}
	for _, item := range s.inventory.snapshot() {
		if item.Equipped && clImages.ItemSlot(uint32(item.ID)) == slot {
			return item.Name
		}
	}
	return "Nothing"
}

func (s *Session) legacyMacroShares(outbound bool) string {
	if s == nil || s.players == nil {
		return ""
	}
	self := s.characterName()
	names := make([]string, 0)
	for _, player := range s.players.snapshot() {
		if strings.EqualFold(player.Name, self) {
			continue
		}
		if (outbound && player.Sharee) || (!outbound && player.Sharing) {
			names = append(names, legacyMacroSimplePlayerName(player.Name))
		}
	}
	sort.Strings(names)
	return strings.Join(names, " ")
}

func (s *Session) advanceLegacyMacros(frame int64) {
	if s == nil || s.automation == nil {
		return
	}
	s.automation.legacyMu.RLock()
	runtime := s.automation.legacyRuntime
	s.automation.legacyMu.RUnlock()
	if runtime != nil {
		runtime.advance(frame)
	}
}

func (s *Session) advanceScriptTick() {
	if s == nil || s.automation == nil || s.automation.scriptTimers == nil {
		return
	}
	s.automation.scriptTimers.advanceTick()
}

func (s *Session) legacyMacroRuntimeSnapshot() *legacyMacroRuntime {
	if s == nil || s.automation == nil {
		return nil
	}
	s.automation.legacyMu.RLock()
	runtime := s.automation.legacyRuntime
	s.automation.legacyMu.RUnlock()
	return runtime
}

func (s *sessionAutomationState) reset() {
	if s == nil {
		return
	}
	s.legacyMu.Lock()
	runtime := s.legacyRuntime
	s.legacySources = nil
	s.legacyProgram = legacyMacroProgram{}
	s.legacyRuntime = nil
	s.legacyMu.Unlock()
	s.locationMu.Lock()
	s.location = ""
	s.locationMu.Unlock()
	if runtime != nil {
		runtime.cancelAll()
	}
	s.stopSessionScripts("session reset")
	s.sendMu.Lock()
	clear(s.sendHistory)
	s.sendMu.Unlock()
	s.scriptMu.Lock()
	s.changeSnapshot = scriptChangeSnapshot{}
	s.latestServer = scriptapi.ServerMessage{}
	s.serverSequence = 0
	s.hasServer = false
	s.scriptMu.Unlock()
}
