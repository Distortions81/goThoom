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
	scriptPlayers  []scriptPlayerChangeHandler
	localCommands  map[string]sessionScriptCommand
	localHotkeys   map[string]map[string]sessionScriptHotkey
	localToolbars  map[string][]*scriptToolbarRegistration
	toolbarNext    map[string]int
	stateWaiters   map[string][]*scriptStateWaiter
	changeSnapshot scriptChangeSnapshot
	latestServer   scriptapi.ServerMessage
	serverSequence uint64
	hasServer      bool
	sendMu         sync.Mutex
	sendHistory    map[string][]time.Time

	locationMu sync.RWMutex
	location   string
}

type sessionScriptCommand struct {
	owner   string
	handler scriptCommandHandler
}

type sessionScriptHotkey struct {
	owner    string
	original string
	handler  func(InputEvent) bool
}

func (s *Session) registerSessionScriptHotkey(owner, combo string, handler func(InputEvent), queue *scriptEventQueue) scriptRegistrationHandle {
	if s == nil || s.automation == nil || handler == nil || queue == nil || scriptIsDisabled(owner) {
		return scriptRegistrationHandle{}
	}
	combo = strings.TrimSpace(combo)
	original := combo
	combo = scriptControlValue(owner, "binding", original)
	if combo == "" {
		return scriptRegistrationHandle{}
	}
	// User-created hotkeys are app-wide and retain priority. Script bindings
	// from another session do not conflict because only the selected session's
	// runtime receives direct input.
	hotkeysMu.RLock()
	for _, existing := range hotkeys {
		if existing.Script == "" && sameCombo(existing.Combo, combo) {
			hotkeysMu.RUnlock()
			reportScriptBindingConflict(combo, "global hotkeys")
			return scriptRegistrationHandle{}
		}
	}
	hotkeysMu.RUnlock()

	s.automation.scriptMu.Lock()
	for existingOwner, bindings := range s.automation.localHotkeys {
		for existingCombo := range bindings {
			if sameCombo(existingCombo, combo) {
				s.automation.scriptMu.Unlock()
				reportScriptBindingConflict(combo, existingOwner)
				return scriptRegistrationHandle{}
			}
		}
	}
	if s.automation.localHotkeys == nil {
		s.automation.localHotkeys = make(map[string]map[string]sessionScriptHotkey)
	}
	bindings := s.automation.localHotkeys[owner]
	if bindings == nil {
		bindings = make(map[string]sessionScriptHotkey)
		s.automation.localHotkeys[owner] = bindings
	}
	bindings[combo] = sessionScriptHotkey{
		owner: owner, original: original,
		handler: func(event InputEvent) bool {
			if !queueScriptCallbackWaitOn(queue, owner, "Input", func() { handler(event) }) {
				return true
			}
			return event.Continues()
		},
	}
	s.automation.scriptMu.Unlock()

	var registration scriptRegistrationHandle
	registration = registerSessionScriptResource(queue, func() {
		s.automation.scriptMu.Lock()
		if bindings := s.automation.localHotkeys[owner]; bindings != nil {
			delete(bindings, combo)
			if len(bindings) == 0 {
				delete(s.automation.localHotkeys, owner)
			}
		}
		s.automation.scriptMu.Unlock()
	})
	scriptLogEvent(owner, "Registered binding", combo)
	return registration
}

func (s *Session) sessionScriptHotkey(combo string) (sessionScriptHotkey, bool, bool) {
	if s == nil || s.automation == nil {
		return sessionScriptHotkey{}, false, false
	}
	s.automation.scriptMu.RLock()
	defer s.automation.scriptMu.RUnlock()
	for _, bindings := range s.automation.localHotkeys {
		for registered, hotkey := range bindings {
			if sameCombo(registered, combo) {
				scriptHotkeyMu.RLock()
				enabled, known := scriptHotkeyEnabled[hotkey.owner][hotkey.original]
				scriptHotkeyMu.RUnlock()
				return hotkey, true, !known || enabled
			}
		}
	}
	return sessionScriptHotkey{}, false, false
}

func (s *Session) registerSessionScriptCommand(owner, name string, handler scriptCommandHandler, queue *scriptEventQueue) scriptRegistrationHandle {
	if s == nil || s.automation == nil || handler == nil || queue == nil || scriptIsDisabled(owner) {
		return scriptRegistrationHandle{}
	}
	original := normalizeScriptCommand(name)
	key := scriptControlValue(owner, "command", original)
	if key == "" {
		return scriptRegistrationHandle{}
	}
	s.automation.scriptMu.Lock()
	if s.automation.localCommands == nil {
		s.automation.localCommands = make(map[string]sessionScriptCommand)
	}
	if _, exists := s.automation.localCommands[key]; exists {
		s.automation.scriptMu.Unlock()
		s.publishConsole(fmt.Sprintf("[script] command conflict: /%s already registered", key), messageTextTypeSystem)
		return scriptRegistrationHandle{}
	}
	entry := sessionScriptCommand{owner: owner, handler: func(args string) {
		queueScriptCallbackOn(queue, owner, "Command", func() { handler(args) })
	}}
	s.automation.localCommands[key] = entry
	s.automation.scriptMu.Unlock()

	var registration scriptRegistrationHandle
	registration = registerSessionScriptResource(queue, func() {
		s.automation.scriptMu.Lock()
		if current, ok := s.automation.localCommands[key]; ok && current.owner == owner {
			delete(s.automation.localCommands, key)
		}
		s.automation.scriptMu.Unlock()
	})
	scriptLogEvent(owner, "Registered command", "/"+key)
	return registration
}

func (s *Session) sessionScriptCommand(name string) (sessionScriptCommand, bool) {
	if s == nil || s.automation == nil {
		return sessionScriptCommand{}, false
	}
	s.automation.scriptMu.RLock()
	command, ok := s.automation.localCommands[name]
	s.automation.scriptMu.RUnlock()
	return command, ok
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
		scriptQueues:  newScriptQueueRegistry(),
		scriptTimers:  newScriptTimerRegistry(),
		scripts:       make(map[string]*sessionScriptInstance),
		localCommands: make(map[string]sessionScriptCommand),
		localHotkeys:  make(map[string]map[string]sessionScriptHotkey),
		localToolbars: make(map[string][]*scriptToolbarRegistration),
		toolbarNext:   make(map[string]int),
		sendHistory:   make(map[string][]time.Time),
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
	s.input.markMacroMoved()
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
	case "@selplayer.name":
		return s.selectedPlayerSnapshot(), true
	case "@selplayer.simple_name":
		return legacyMacroSimplePlayerName(s.selectedPlayerSnapshot()), true
	case "@my.shares_in":
		return s.legacyMacroShares(false), true
	case "@my.shares_out":
		return s.legacyMacroShares(true), true
	case "@my.selected_item":
		if item, ok := scriptSelectedItemForSession(s); ok {
			return item.Name, true
		}
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
	clear(s.localCommands)
	clear(s.localHotkeys)
	clear(s.localToolbars)
	clear(s.toolbarNext)
	clear(s.stateWaiters)
	s.changeSnapshot = scriptChangeSnapshot{}
	s.latestServer = scriptapi.ServerMessage{}
	s.serverSequence = 0
	s.hasServer = false
	s.scriptMu.Unlock()
}
