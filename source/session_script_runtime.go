package main

import (
	"fmt"

	"github.com/traefik/yaegi/interp"
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
