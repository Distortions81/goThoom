package main

import (
	"reflect"
	"strconv"
	"strings"
	"time"

	scriptapi "gt2"
)

// Storage binds an opaque handle to the interpreter and character that created it.
type Storage struct {
	owner, character string
	candidate        *scriptCandidate
}

func (s Storage) Active() bool {
	scriptSessionMu.Lock()
	character := normalizeScriptCharacter(scriptSessionCharacter)
	scriptSessionMu.Unlock()
	// The character name remains known during logout and Terminate; the candidate
	// still enforces interpreter lifetime and session generation.
	return s.character != "" && s.character == character &&
		s.candidate.apiCallAllowed(s.owner, "storage") && scriptHasPermission(s.owner, "storage")
}
func normalizeScriptCharacter(name string) string { return strings.ToLower(strings.TrimSpace(name)) }
func (s Storage) key(key string) string {
	return "__gt:character:" + strconv.Quote(s.character) + ":" + key
}
func (s Storage) get(key string) any {
	if !s.Active() {
		return nil
	}
	return s.candidate.getStorage(s.owner, s.key(key))
}
func (s Storage) Store(key string, value any) {
	if s.Active() {
		s.candidate.setStorage(s.owner, s.key(key), value)
	}
}
func (s Storage) LoadString(key, fallback string) string {
	return scriptStoredString(s.get(key), fallback)
}
func (s Storage) LoadBool(key string, fallback bool) bool {
	return scriptStoredBool(s.get(key), fallback)
}
func (s Storage) LoadInteger(key string, fallback int) int {
	return scriptStoredInteger(s.get(key), fallback)
}
func (s Storage) LoadDecimal(key string, fallback float64) float64 {
	return scriptStoredDecimal(s.get(key), fallback)
}
func (s Storage) LoadStrings(key string, fallback []string) []string {
	return scriptStoredStrings(s.get(key), fallback)
}
func (s Storage) LoadJSON(key string, target any) bool { return scriptStoredJSON(s.get(key), target) }
func (s Storage) DeleteStored(key string) {
	if s.Active() {
		s.candidate.deleteStorage(s.owner, s.key(key))
	}
}

func addScriptExtendedExports(m map[string]reflect.Value, owner string, candidate *scriptCandidate) {
	for name, value := range map[string]any{
		"Storage": (*Storage)(nil), "Task": (*Task)(nil), "CommandTicket": (*CommandTicket)(nil),
		"CommandStatus": (*scriptapi.CommandStatus)(nil), "PlayerChangeEvent": (*scriptapi.PlayerChangeEvent)(nil),
		"WindowControl": (*scriptapi.WindowControl)(nil), "WindowControlEvent": (*scriptapi.WindowControlEvent)(nil),
		"WindowRow":     (*scriptapi.WindowRow)(nil),
		"CommandQueued": scriptapi.CommandQueued, "CommandSent": scriptapi.CommandSent,
		"CommandCancelled": scriptapi.CommandCancelled, "CommandRejected": scriptapi.CommandRejected,
		"PlayerDiscovered": scriptapi.PlayerDiscovered, "PlayerRemoved": scriptapi.PlayerRemoved,
		"PlayerLogin": scriptapi.PlayerLogin, "PlayerLogout": scriptapi.PlayerLogout,
		"PlayerFallen": scriptapi.PlayerFallen, "PlayerRecovered": scriptapi.PlayerRecovered, "PlayerSharing": scriptapi.PlayerSharing,
		"ControlText": scriptapi.ControlText, "ControlCheckbox": scriptapi.ControlCheckbox,
		"ControlDropdown": scriptapi.ControlDropdown, "ControlList": scriptapi.ControlList, "ControlColor": scriptapi.ControlColor,
		"ControlImage": scriptapi.ControlImage,
	} {
		m[name] = reflect.ValueOf(value)
	}
	m["HasPermission"] = reflect.ValueOf(func(permission string) bool { return scriptHasPermission(owner, permission) })
	m["CharacterStore"] = reflect.ValueOf(func() Storage {
		return Storage{owner: owner, character: normalizeScriptCharacter(scriptExecutionCharacter()), candidate: candidate}
	})
	m["QueueCommand"] = reflect.ValueOf(func(cmd string) CommandTicket {
		ticket := newScriptCommandTicket(owner, candidate.runtimeEventQueue(owner))
		ticket.state.candidate = candidate
		cmd = strings.TrimSpace(cmd)
		if candidate.runtimeEventQueue(owner) != nil {
			queueTrackedScriptCommand(ticket, cmd)
		} else {
			candidate.dispatch(owner, func() { queueTrackedScriptCommand(ticket, cmd) })
		}
		return ticket
	})
	m["StartTask"] = reflect.ValueOf(func(fn func()) Task {
		if fn == nil {
			return Task{}
		}
		task := newScriptTask(owner)
		if queue := candidate.runtimeEventQueue(owner); queue != nil {
			task.start(queue, fn)
		} else {
			candidate.dispatch(owner, func() { task.start(currentScriptEventQueue(owner), fn) })
		}
		return task
	})
	m["After"] = reflect.ValueOf(func(delay time.Duration, fn func()) Timer {
		timer := newScriptTimer()
		if fn == nil || delay < 0 {
			timer.Stop()
			return timer
		}
		if candidate.runtimeEventQueue(owner) != nil {
			startScriptAfter(owner, delay, fn, timer)
		} else {
			candidate.dispatch(owner, func() { startScriptAfter(owner, delay, fn, timer) })
		}
		return timer
	})
	m["OnPlayerChange"] = reflect.ValueOf(func(fn func(scriptapi.PlayerChangeEvent)) Subscription {
		if fn == nil {
			return Subscription{}
		}
		sub := newScriptSubscription()
		candidate.dispatch(owner, func() { sub.attach(registerScriptPlayerChange(owner, fn)) })
		return sub
	})
}

func startScriptAfter(owner string, delay time.Duration, fn func(), timer Timer) {
	queue := currentScriptEventQueue(owner)
	if queue == nil {
		timer.Stop()
		return
	}
	stop := make(chan struct{})
	handle := registerScriptResource(owner, timer.Stop)
	timer.attach(func() { close(stop); handle.release() })
	go func() {
		clock := time.NewTimer(delay)
		defer clock.Stop()
		select {
		case <-clock.C:
			if !queueScriptCallbackOn(queue, owner, "After", func() {
				if !timer.Active() {
					return
				}
				timer.Stop()
				fn()
			}) {
				timer.Stop()
			}
		case <-stop:
		case <-queue.done:
			timer.Stop()
		}
	}()
}

type scriptPlayerChangeHandler struct {
	owner  string
	queue  *scriptEventQueue
	handle scriptRegistrationHandle
	fn     func(scriptapi.PlayerChangeEvent)
}

var scriptPlayerChangeHandlers []scriptPlayerChangeHandler

func registerScriptPlayerChange(owner string, fn func(scriptapi.PlayerChangeEvent)) scriptRegistrationHandle {
	var handle scriptRegistrationHandle
	handle = registerScriptResource(owner, func() {
		chatHandlersMu.Lock()
		defer chatHandlersMu.Unlock()
		for i, h := range scriptPlayerChangeHandlers {
			if h.handle == handle {
				scriptPlayerChangeHandlers = append(scriptPlayerChangeHandlers[:i], scriptPlayerChangeHandlers[i+1:]...)
				break
			}
		}
	})
	if !handle.valid() {
		return handle
	}
	chatHandlersMu.Lock()
	scriptPlayerChangeHandlers = append(scriptPlayerChangeHandlers, scriptPlayerChangeHandler{owner, handle.queue, handle, fn})
	chatHandlersMu.Unlock()
	return handle
}

// Avoid scanning and copying player state when no script consumes these events.
func captureScriptPlayerChanges() ([]scriptapi.Player, bool) {
	chatHandlersMu.RLock()
	observed := len(scriptPlayerChangeHandlers) > 0
	chatHandlersMu.RUnlock()
	if !observed {
		return nil, false
	}
	return scriptPlayers(), true
}

func dispatchScriptPlayerChanges(previous, current []scriptapi.Player) {
	old := make(map[string]scriptapi.Player, len(previous))
	for _, p := range previous {
		old[normalizeScriptCharacter(p.Name)] = p
	}
	var events []scriptapi.PlayerChangeEvent
	for _, p := range current {
		key := normalizeScriptCharacter(p.Name)
		before, found := old[key]
		delete(old, key)
		add := func(kind string) {
			events = append(events, scriptapi.PlayerChangeEvent{Type: kind, Previous: before, Player: p})
		}
		if !found {
			add(scriptapi.PlayerDiscovered)
			continue
		}
		if before.Offline != p.Offline {
			if p.Offline {
				add(scriptapi.PlayerLogout)
			} else {
				add(scriptapi.PlayerLogin)
			}
		}
		if before.Dead != p.Dead {
			if p.Dead {
				add(scriptapi.PlayerFallen)
			} else {
				add(scriptapi.PlayerRecovered)
			}
		}
		if before.Sharing != p.Sharing || before.Sharee != p.Sharee {
			add(scriptapi.PlayerSharing)
		}
	}
	// Preserve the previous snapshot order for removals.
	for _, p := range previous {
		if _, found := old[normalizeScriptCharacter(p.Name)]; found {
			events = append(events, scriptapi.PlayerChangeEvent{Type: scriptapi.PlayerRemoved, Previous: p})
		}
	}
	chatHandlersMu.RLock()
	handlers := append([]scriptPlayerChangeHandler(nil), scriptPlayerChangeHandlers...)
	chatHandlersMu.RUnlock()
	for _, event := range events {
		for _, h := range handlers {
			snapshot := event
			snapshot.Previous.Colors = append([]byte(nil), event.Previous.Colors...)
			snapshot.Player.Colors = append([]byte(nil), event.Player.Colors...)
			queueScriptCallbackOn(h.queue, h.owner, "Player "+event.Type, func() { h.fn(snapshot) })
		}
	}
}
