package main

import (
	"crypto/sha256"
	"fmt"
	"log"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/traefik/yaegi/interp"
	scriptapi "gt2"
)

// sessionScriptInstance is one interpreter that belongs to exactly one
// connection. The shared Scripts window displays the selected runtime.
type sessionScriptInstance struct {
	owner       string
	prepared    *preparedScript
	queue       *scriptEventQueue
	fingerprint [sha256.Size]byte
}

func (s *Session) startSessionScript(owner string, src []byte, restricted interp.Exports, assets *scriptAssetSource) error {
	return s.startSessionScriptVersion(owner, src, restricted, assets, sha256.Sum256(src))
}

func (s *Session) startSessionScriptVersion(owner string, src []byte, restricted interp.Exports, assets *scriptAssetSource, fingerprint [sha256.Size]byte) error {
	if s == nil || s.automation == nil {
		return fmt.Errorf("script session is unavailable")
	}
	prepared, err := prepareScriptSourceWithAssetsForSession(s, owner, src, restricted, assets)
	if err != nil {
		return err
	}
	s.stopSessionScript(owner, "reloaded")
	queue := startSessionScriptEventQueue(s, owner, prepared.interpreter)
	if queue == nil {
		disposePreparedScript(prepared)
		return fmt.Errorf("could not start script queue")
	}
	queue.diagnostics = prepared.diagnostics
	prepared.candidate.activate(queue)
	s.automation.scriptMu.Lock()
	s.automation.scripts[owner] = &sessionScriptInstance{owner: owner, prepared: prepared, queue: queue, fingerprint: fingerprint}
	delete(s.automation.scriptErrors, owner)
	s.automation.scriptMu.Unlock()
	s.dispatchSessionScriptLifecycleForOwner(LifecycleEvent{Type: lifecycleLogin, Character: s.characterName()}, owner, true)
	return nil
}

func (s *Session) stopSessionScript(owner, reason string) {
	if s == nil || s.automation == nil {
		return
	}
	s.automation.scriptMu.Lock()
	instance := s.automation.scripts[owner]
	delete(s.automation.scripts, owner)
	delete(s.automation.scriptConfigs, owner)
	s.automation.scriptMu.Unlock()
	if instance == nil {
		return
	}
	if scriptConfigWin != nil && scriptConfigOwner == owner && scriptConfigSession == s.ID() {
		dispatchMainThread(func() {
			if scriptConfigWin != nil && scriptConfigOwner == owner && scriptConfigSession == s.ID() {
				scriptConfigWin.Close()
			}
		})
	}
	s.dispatchSessionScriptLifecycleForOwner(LifecycleEvent{Type: lifecycleStop, Character: s.characterName(), Reason: reason}, owner, true)
	if instance.prepared.terminate != nil {
		instance.prepared.candidate.callTerminate(instance.prepared.terminate)
	}
	s.commands.cancelScriptCommands(owner, nil)
	stopScriptMovementForSession(s, owner)
	stopSessionScriptEventQueue(s, owner)
	releaseScriptRegistrations(instance.queue)
	s.automation.clearScriptSendHistory(owner)
	s.automation.visuals.clearOwner(owner)
	instance.prepared.candidate.discard()
	interruptScriptInterpreter(instance.prepared.interpreter)
}

type enabledSessionScript struct {
	owner string
	info  scriptInfo
}

func enabledSessionScripts(character string) []enabledSessionScript {
	character = strings.TrimSpace(character)
	scriptMu.RLock()
	scripts := make([]enabledSessionScript, 0, len(scriptPackages))
	for owner, info := range scriptPackages {
		if scriptInvalid[owner] || !scriptEnabledFor[owner].enablesFor(character) {
			continue
		}
		scripts = append(scripts, enabledSessionScript{owner: owner, info: info})
	}
	scriptMu.RUnlock()
	sort.Slice(scripts, func(i, j int) bool { return scripts[i].owner < scripts[j].owner })
	return scripts
}

// syncEnabledSessionScripts makes one session match the saved global and
// per-character script selections.
func (s *Session) syncEnabledSessionScripts(character string) []error {
	if s == nil || s.automation == nil {
		return nil
	}
	expectedList := enabledSessionScripts(character)
	expected := make(map[string]scriptInfo, len(expectedList))
	for _, script := range expectedList {
		expected[script.owner] = script.info
	}

	s.automation.scriptMu.RLock()
	running := make(map[string][sha256.Size]byte, len(s.automation.scripts))
	for owner, instance := range s.automation.scripts {
		if instance != nil {
			running[owner] = instance.fingerprint
		}
	}
	s.automation.scriptMu.RUnlock()

	var stopped []string
	for owner := range running {
		if _, ok := expected[owner]; !ok {
			stopped = append(stopped, owner)
		}
	}
	sort.Strings(stopped)
	for _, owner := range stopped {
		s.stopSessionScript(owner, "disabled for this character")
	}

	var errs []error
	for _, script := range expectedList {
		if fingerprint, ok := running[script.owner]; ok && fingerprint == script.info.fingerprint {
			continue
		}
		if err := s.startSessionScriptVersion(script.owner, script.info.src, restrictedStdlib(), script.info.assets, script.info.fingerprint); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", script.owner, err))
		}
	}
	return errs
}

func reportSessionScriptSyncErrors(session *Session, errs []error) {
	for _, err := range errs {
		owner := strings.TrimSpace(strings.SplitN(err.Error(), ":", 2)[0])
		if session != nil && session.automation != nil && owner != "" {
			session.automation.scriptMu.Lock()
			session.automation.scriptErrors[owner] = err.Error()
			session.automation.scriptMu.Unlock()
		}
		message := fmt.Sprintf("[script] Session %d: %v", session.ID(), err)
		log.Print(message)
		session.publishClientConsole(message, messageTextTypeSystem)
	}
}

func (s *Session) scriptRuntimeSnapshot(owner string) (running bool, errorText string) {
	if s == nil {
		scriptMu.RLock()
		disabled, known := scriptDisabled[owner]
		errorText = scriptErrors[owner]
		scriptMu.RUnlock()
		return known && !disabled, errorText
	}
	if s.automation == nil {
		return false, ""
	}
	s.automation.scriptMu.RLock()
	_, running = s.automation.scripts[owner]
	errorText = s.automation.scriptErrors[owner]
	s.automation.scriptMu.RUnlock()
	return running, errorText
}

func (s *Session) scriptConfigEntriesSnapshot(owner string) []scriptConfigEntry {
	if s == nil {
		scriptConfigMu.RLock()
		entries := append([]scriptConfigEntry(nil), scriptConfigEntries[owner]...)
		scriptConfigMu.RUnlock()
		return entries
	}
	if s.automation == nil {
		return nil
	}
	s.automation.scriptMu.RLock()
	entries := append([]scriptConfigEntry(nil), s.automation.scriptConfigs[owner]...)
	s.automation.scriptMu.RUnlock()
	return entries
}

func scriptManagerSession() *Session {
	if appSessions == nil {
		return primarySession
	}
	return selectedAppSession()
}

func scriptManagerCharacter(session *Session) string {
	if session == nil {
		return effectiveCharacterName()
	}
	if character := strings.TrimSpace(session.characterName()); character != "" {
		return character
	}
	if session.login != nil {
		return strings.TrimSpace(session.login.requestSnapshot().character)
	}
	return ""
}

func scriptRuntimeStatusForSession(session *Session, owner string, scope scriptScope, invalid bool, packageError string, reloadFailed bool) string {
	if invalid {
		return scriptStatusLabel(true, true, packageError, reloadFailed)
	}
	running, runtimeError := session.scriptRuntimeSnapshot(owner)
	if runtimeError != "" {
		if running {
			return "Reload Failed (old version still running)"
		}
		return "Stopped After Error"
	}
	if running {
		return "Running"
	}
	character := scriptManagerCharacter(session)
	if !scope.enablesFor(character) {
		if character == "" {
			return "Waiting for player selection"
		}
		return "Enabled for another player"
	}
	if session == nil || !session.transport.connected() {
		return "Waiting for login"
	}
	return "Stopped"
}

func reloadScriptForSession(session *Session, owner string) {
	if session == nil {
		return
	}
	info, err := refreshScriptPackage(owner)
	if err != nil {
		session.automation.scriptMu.Lock()
		session.automation.scriptErrors[owner] = err.Error()
		session.automation.scriptMu.Unlock()
		session.publishClientConsole("[script] reload error: "+err.Error(), messageTextTypeSystem)
		refreshscriptsWindow()
		refreshscriptDetails()
		return
	}
	running, _ := session.scriptRuntimeSnapshot(owner)
	if !running {
		deleteSessionScriptError(session, owner)
		refreshscriptsWindow()
		refreshscriptDetails()
		return
	}
	if err := session.startSessionScriptVersion(owner, info.src, restrictedStdlib(), info.assets, info.fingerprint); err != nil {
		session.automation.scriptMu.Lock()
		session.automation.scriptErrors[owner] = err.Error()
		session.automation.scriptMu.Unlock()
		session.publishClientConsole("[script] reload error: "+err.Error(), messageTextTypeSystem)
	}
	refreshscriptsWindow()
	refreshscriptDetails()
}

func deleteSessionScriptError(session *Session, owner string) {
	if session == nil || session.automation == nil {
		return
	}
	session.automation.scriptMu.Lock()
	delete(session.automation.scriptErrors, owner)
	session.automation.scriptMu.Unlock()
}

func init() {
	previous := queueSelectedSessionUIUpdate
	queueSelectedSessionUIUpdate = func() {
		previous()
		dispatchMainThread(func() {
			refreshscriptsWindow()
			refreshscriptDetails()
			if scriptConfigWin != nil && scriptConfigSession != selectedAppSession().ID() {
				scriptConfigWin.Close()
			}
		})
	}
}

func syncConnectedSessionScripts() {
	if appSessions == nil {
		return
	}
	for _, session := range appSessions.snapshot() {
		if session == nil || !session.transport.connected() {
			continue
		}
		reportSessionScriptSyncErrors(session, session.syncEnabledSessionScripts(session.characterName()))
	}
}

func restartConnectedSessionScript(owner, reason string) {
	if appSessions == nil {
		return
	}
	for _, session := range appSessions.snapshot() {
		if session == nil || !session.transport.connected() {
			continue
		}
		session.stopSessionScript(owner, reason)
		reportSessionScriptSyncErrors(session, session.syncEnabledSessionScripts(session.characterName()))
	}
}

func (s *sessionAutomationState) stopSessionScripts(reason string) {
	if s == nil {
		return
	}
	s.scriptMu.RLock()
	owners := make([]string, 0, len(s.scripts))
	for owner := range s.scripts {
		owners = append(owners, owner)
	}
	s.scriptMu.RUnlock()
	// The owning Session is recorded on every queue, so this state method can
	// be used from Session.resetConnectionModels without an implicit primary
	// session reference.
	for _, owner := range owners {
		s.scriptMu.RLock()
		instance := s.scripts[owner]
		s.scriptMu.RUnlock()
		if instance != nil && instance.queue != nil && instance.queue.session != nil {
			instance.queue.session.stopSessionScript(owner, reason)
		}
	}
	s.scriptQueues.stopAll()
}

func registerSessionScriptResource(queue *scriptEventQueue, cleanup func()) scriptRegistrationHandle {
	return registerScriptResourceOn(queue, cleanup)
}

func (s *Session) registerSessionScriptChat(owner string, filter ChatFilter, fn func(ChatEvent)) scriptRegistrationHandle {
	queue := currentSessionScriptEventQueue(s, owner)
	if fn == nil || queue == nil {
		return scriptRegistrationHandle{}
	}
	var registration scriptRegistrationHandle
	registration = registerSessionScriptResource(queue, func() {
		s.automation.scriptMu.Lock()
		for i := len(s.automation.scriptChats) - 1; i >= 0; i-- {
			if s.automation.scriptChats[i].registration == registration {
				s.automation.scriptChats = append(s.automation.scriptChats[:i], s.automation.scriptChats[i+1:]...)
			}
		}
		s.automation.scriptMu.Unlock()
	})
	if !registration.valid() {
		return registration
	}
	s.automation.scriptMu.Lock()
	s.automation.scriptChats = append(s.automation.scriptChats, structuredChatHandler{owner: owner, filter: filter, fn: fn, queue: queue, registration: registration})
	s.automation.scriptMu.Unlock()
	return registration
}

func (s *Session) registerSessionScriptLifecycle(owner, kind string, fn func(LifecycleEvent)) scriptRegistrationHandle {
	queue := currentSessionScriptEventQueue(s, owner)
	if fn == nil || queue == nil {
		return scriptRegistrationHandle{}
	}
	var registration scriptRegistrationHandle
	registration = registerSessionScriptResource(queue, func() {
		s.automation.scriptMu.Lock()
		for i := len(s.automation.scriptEvents) - 1; i >= 0; i-- {
			if s.automation.scriptEvents[i].registration == registration {
				s.automation.scriptEvents = append(s.automation.scriptEvents[:i], s.automation.scriptEvents[i+1:]...)
			}
		}
		s.automation.scriptMu.Unlock()
	})
	if !registration.valid() {
		return registration
	}
	s.automation.scriptMu.Lock()
	s.automation.scriptEvents = append(s.automation.scriptEvents, scriptLifecycleHandler{owner: owner, kind: kind, fn: fn, queue: queue, registration: registration})
	s.automation.scriptMu.Unlock()
	return registration
}

func (s *Session) registerSessionScriptServerMessage(owner string, filter ServerMessageFilter, fn func(scriptapi.ServerMessage)) scriptRegistrationHandle {
	queue := currentSessionScriptEventQueue(s, owner)
	if fn == nil || queue == nil {
		return scriptRegistrationHandle{}
	}
	var registration scriptRegistrationHandle
	registration = registerSessionScriptResource(queue, func() {
		s.automation.scriptMu.Lock()
		for i := len(s.automation.scriptServers) - 1; i >= 0; i-- {
			if s.automation.scriptServers[i].registration == registration {
				s.automation.scriptServers = append(s.automation.scriptServers[:i], s.automation.scriptServers[i+1:]...)
			}
		}
		s.automation.scriptMu.Unlock()
	})
	if !registration.valid() {
		return registration
	}
	s.automation.scriptMu.Lock()
	s.automation.scriptServers = append(s.automation.scriptServers, serverMessageHandler{
		owner: owner, filter: filter, fn: fn, queue: queue, registration: registration,
	})
	s.automation.scriptMu.Unlock()
	return registration
}

func (s *Session) registerSessionScriptChange(owner, kind string, fn func(ChangeEvent)) scriptRegistrationHandle {
	queue := currentSessionScriptEventQueue(s, owner)
	if fn == nil || queue == nil {
		return scriptRegistrationHandle{}
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	var registration scriptRegistrationHandle
	registration = registerSessionScriptResource(queue, func() {
		s.automation.scriptMu.Lock()
		for i := len(s.automation.scriptChanges) - 1; i >= 0; i-- {
			if s.automation.scriptChanges[i].registration == registration {
				s.automation.scriptChanges = append(s.automation.scriptChanges[:i], s.automation.scriptChanges[i+1:]...)
			}
		}
		s.automation.scriptMu.Unlock()
	})
	if !registration.valid() {
		return registration
	}
	s.automation.scriptMu.Lock()
	s.automation.scriptChanges = append(s.automation.scriptChanges, scriptChangeHandler{
		owner: owner, kind: kind, fn: fn, queue: queue, registration: registration,
	})
	s.automation.scriptMu.Unlock()
	return registration
}

func (s *Session) registerSessionScriptPlayerChange(owner string, fn func(scriptapi.PlayerChangeEvent)) scriptRegistrationHandle {
	queue := currentSessionScriptEventQueue(s, owner)
	if fn == nil || queue == nil {
		return scriptRegistrationHandle{}
	}
	var registration scriptRegistrationHandle
	registration = registerSessionScriptResource(queue, func() {
		s.automation.scriptMu.Lock()
		for i := len(s.automation.scriptPlayers) - 1; i >= 0; i-- {
			if s.automation.scriptPlayers[i].handle == registration {
				s.automation.scriptPlayers = append(s.automation.scriptPlayers[:i], s.automation.scriptPlayers[i+1:]...)
			}
		}
		s.automation.scriptMu.Unlock()
	})
	if !registration.valid() {
		return registration
	}
	s.automation.scriptMu.Lock()
	s.automation.scriptPlayers = append(s.automation.scriptPlayers, scriptPlayerChangeHandler{
		owner: owner, queue: queue, handle: registration, fn: fn,
	})
	s.automation.scriptMu.Unlock()
	return registration
}

func (s *Session) dispatchSessionScriptChat(msg string) {
	if s == nil || s.automation == nil {
		return
	}
	s.automation.scriptMu.RLock()
	if len(s.automation.scriptChats) == 0 {
		s.automation.scriptMu.RUnlock()
		return
	}
	handlers := append([]structuredChatHandler(nil), s.automation.scriptChats...)
	s.automation.scriptMu.RUnlock()
	event := classifyScriptChatForSession(s, msg)
	for _, handler := range handlers {
		if handler.fn == nil || !scriptChatFilterMatches(handler.filter, event) {
			continue
		}
		fn := handler.fn
		if !queueScriptCallbackOn(handler.queue, handler.owner, "OnChat", func() { fn(event) }) {
			continue
		}
	}
}

func (s *Session) dispatchSessionScriptLifecycle(event LifecycleEvent, wait bool) {
	s.dispatchSessionScriptLifecycleForOwner(event, "", wait)
}

func (s *Session) dispatchSessionScriptLifecycleForOwner(event LifecycleEvent, owner string, wait bool) {
	if s == nil || s.automation == nil {
		return
	}
	s.automation.scriptMu.RLock()
	handlers := append([]scriptLifecycleHandler(nil), s.automation.scriptEvents...)
	s.automation.scriptMu.RUnlock()
	for _, handler := range handlers {
		if handler.kind != event.Type || handler.fn == nil || owner != "" && handler.owner != owner {
			continue
		}
		fn := handler.fn
		if wait {
			queueScriptCallbackWaitOn(handler.queue, handler.owner, "Lifecycle "+event.Type, func() { fn(event) })
		} else {
			queueScriptCallbackOn(handler.queue, handler.owner, "Lifecycle "+event.Type, func() { fn(event) })
		}
	}
}

func (s *Session) dispatchSessionScriptServerMessage(event scriptapi.ServerMessage) {
	if s == nil || s.automation == nil {
		return
	}
	s.automation.scriptMu.Lock()
	s.automation.serverSequence++
	event.Sequence = s.automation.serverSequence
	event.ReceivedAt = time.Now()
	s.automation.latestServer = event
	s.automation.hasServer = true
	handlers := append([]serverMessageHandler(nil), s.automation.scriptServers...)
	s.automation.scriptMu.Unlock()
	for _, handler := range handlers {
		if handler.fn == nil || handler.filter.Type != "" && !strings.EqualFold(handler.filter.Type, event.Type) ||
			handler.filter.Contains != "" && !strings.Contains(strings.ToLower(event.Message), strings.ToLower(handler.filter.Contains)) {
			continue
		}
		fn := handler.fn
		queueScriptCallbackOn(handler.queue, handler.owner, "OnServerMessage", func() { fn(event) })
	}
}

func (s *Session) latestSessionScriptServerMessage() (scriptapi.ServerMessage, bool) {
	if s == nil || s.automation == nil {
		return scriptapi.ServerMessage{}, false
	}
	s.automation.scriptMu.RLock()
	event, ok := s.automation.latestServer, s.automation.hasServer
	s.automation.scriptMu.RUnlock()
	return event, ok
}

func (s *Session) dispatchSessionScriptChange(event ChangeEvent) {
	if s == nil || s.automation == nil {
		return
	}
	s.automation.scriptMu.RLock()
	handlers := append([]scriptChangeHandler(nil), s.automation.scriptChanges...)
	s.automation.scriptMu.RUnlock()
	for _, handler := range handlers {
		if handler.fn == nil || handler.kind != "" && handler.kind != event.Type {
			continue
		}
		fn := handler.fn
		queueScriptCallbackOn(handler.queue, handler.owner, "Change "+event.Type, func() { fn(event) })
	}
}

func (s *Session) pollSessionScriptChangeEvents() {
	if s == nil || s.automation == nil {
		return
	}
	current := captureSessionScriptChangeSnapshot(s)
	s.automation.scriptMu.Lock()
	previous := s.automation.changeSnapshot
	s.automation.changeSnapshot = current
	s.automation.scriptMu.Unlock()
	s.notifySessionScriptStateWaiters()
	if !previous.initialized {
		return
	}
	if previous.playersObserved && current.playersObserved {
		s.dispatchSessionScriptPlayerChanges(previous.players, current.players)
	}
	dispatchScriptSnapshotChanges(previous, current, s.dispatchSessionScriptChange)
}

func (s *Session) dispatchSessionScriptPlayerChanges(previous, current []scriptapi.Player) {
	if s == nil || s.automation == nil {
		return
	}
	s.automation.scriptMu.RLock()
	handlers := append([]scriptPlayerChangeHandler(nil), s.automation.scriptPlayers...)
	s.automation.scriptMu.RUnlock()
	dispatchScriptPlayerChangeEvents(scriptPlayerChangeEvents(previous, current), handlers)
}

func (s *Session) notifySessionScriptStateWaiters() {
	if s == nil || s.automation == nil {
		return
	}
	s.automation.scriptMu.Lock()
	for _, waiters := range s.automation.stateWaiters {
		notifyScriptStateWaiterList(waiters)
	}
	s.automation.scriptMu.Unlock()
}

func captureSessionScriptChangeSnapshot(session *Session) scriptChangeSnapshot {
	if session == nil {
		return scriptChangeSnapshot{}
	}
	inventory := scriptInventoryForSession(session)
	session.automation.scriptMu.RLock()
	playersObserved := len(session.automation.scriptPlayers) > 0
	session.automation.scriptMu.RUnlock()
	var players []scriptapi.Player
	if playersObserved {
		players = scriptPlayersForSession(session)
	}
	selectedPlayer := session.selectedPlayerSnapshot()
	selectedItem, hasSelectedItem := scriptSelectedItemForSession(session)
	equipment := make([]InventoryItem, 0, len(inventory))
	for _, item := range inventory {
		if item.Equipped {
			equipment = append(equipment, item)
		}
	}
	session.draw.mu.Lock()
	health, healthMax := session.draw.current.hp, session.draw.current.hpMax
	spirit, spiritMax := session.draw.current.sp, session.draw.current.spMax
	balanceValue, balanceMaxValue := session.draw.current.balance, session.draw.current.balanceMax
	session.draw.mu.Unlock()
	return scriptChangeSnapshot{
		initialized: true, players: players, playersObserved: playersObserved, inventory: inventory, equipment: equipment,
		selectedPlayer: selectedPlayer, selectedItem: selectedItem, hasSelectedItem: hasSelectedItem,
		health: health, healthMax: healthMax, spirit: spirit, spiritMax: spiritMax,
		balance: balanceValue, balanceMax: balanceMaxValue,
		location: session.scriptLocationSnapshot(), worldGeneration: session.draw.generation.Load(),
	}
}

func dispatchScriptSnapshotChanges(previous, current scriptChangeSnapshot, dispatch func(ChangeEvent)) {
	if dispatch == nil {
		return
	}
	base := ChangeEvent{
		Inventory: append([]InventoryItem(nil), current.inventory...), Equipment: append([]InventoryItem(nil), current.equipment...),
		SelectedPlayer: current.selectedPlayer, SelectedItem: current.selectedItem, HasSelectedItem: current.hasSelectedItem,
		Health: current.health, HealthMax: current.healthMax, Spirit: current.spirit, SpiritMax: current.spiritMax,
		Balance: current.balance, BalanceMax: current.balanceMax, Location: current.location, WorldGeneration: current.worldGeneration,
	}
	if !reflect.DeepEqual(previous.inventory, current.inventory) {
		event := base
		event.Type = ChangeInventory
		dispatch(event)
	}
	if !reflect.DeepEqual(previous.equipment, current.equipment) {
		event := base
		event.Type = ChangeEquipment
		dispatch(event)
	}
	if previous.selectedPlayer != current.selectedPlayer {
		event := base
		event.Type = ChangeSelectedPlayer
		dispatch(event)
	}
	if previous.hasSelectedItem != current.hasSelectedItem || previous.selectedItem != current.selectedItem {
		event := base
		event.Type = ChangeSelectedItem
		dispatch(event)
	}
	if previous.health != current.health || previous.healthMax != current.healthMax || previous.spirit != current.spirit ||
		previous.spiritMax != current.spiritMax || previous.balance != current.balance || previous.balanceMax != current.balanceMax {
		event := base
		event.Type = ChangeVitals
		dispatch(event)
	}
	if previous.worldGeneration != current.worldGeneration {
		event := base
		event.Type = ChangeWorld
		dispatch(event)
	}
	if previous.location != current.location {
		event := base
		event.Type = ChangeLocation
		dispatch(event)
	}
}
