package main

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/traefik/yaegi/interp"
	scriptapi "gt2"
)

// sessionScriptInstance is one interpreter that belongs to exactly one
// connection. The shared Scripts window continues to manage the primary
// runtime; this type gives background sessions an independent lifecycle.
type sessionScriptInstance struct {
	owner    string
	prepared *preparedScript
	queue    *scriptEventQueue
}

func (s *Session) startSessionScript(owner string, src []byte, restricted interp.Exports, assets *scriptAssetSource) error {
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
	s.automation.scripts[owner] = &sessionScriptInstance{owner: owner, prepared: prepared, queue: queue}
	s.automation.scriptMu.Unlock()
	s.dispatchSessionScriptLifecycle(LifecycleEvent{Type: lifecycleLogin, Character: s.characterName()}, true)
	return nil
}

func (s *Session) stopSessionScript(owner, reason string) {
	if s == nil || s.automation == nil {
		return
	}
	s.automation.scriptMu.Lock()
	instance := s.automation.scripts[owner]
	delete(s.automation.scripts, owner)
	s.automation.scriptMu.Unlock()
	if instance == nil {
		return
	}
	s.dispatchSessionScriptLifecycle(LifecycleEvent{Type: lifecycleStop, Character: s.characterName(), Reason: reason}, true)
	if instance.prepared.terminate != nil {
		instance.prepared.candidate.callTerminate(instance.prepared.terminate)
	}
	s.commands.cancelScriptCommands(owner, nil)
	stopSessionScriptEventQueue(s, owner)
	releaseScriptRegistrations(instance.queue)
	s.automation.clearScriptSendHistory(owner)
	instance.prepared.candidate.discard()
	interruptScriptInterpreter(instance.prepared.interpreter)
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

func (s *Session) dispatchSessionScriptChat(msg string) {
	if s == nil || s.automation == nil {
		return
	}
	event := classifyScriptChat(msg)
	s.automation.scriptMu.RLock()
	handlers := append([]structuredChatHandler(nil), s.automation.scriptChats...)
	s.automation.scriptMu.RUnlock()
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
	if s == nil || s.automation == nil {
		return
	}
	s.automation.scriptMu.RLock()
	handlers := append([]scriptLifecycleHandler(nil), s.automation.scriptEvents...)
	s.automation.scriptMu.RUnlock()
	for _, handler := range handlers {
		if handler.kind != event.Type || handler.fn == nil {
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
	if !previous.initialized {
		return
	}
	dispatchScriptSnapshotChanges(previous, current, s.dispatchSessionScriptChange)
}

func captureSessionScriptChangeSnapshot(session *Session) scriptChangeSnapshot {
	if session == nil {
		return scriptChangeSnapshot{}
	}
	inventory := scriptInventoryForSession(session)
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
		initialized: true, inventory: inventory, equipment: equipment,
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
