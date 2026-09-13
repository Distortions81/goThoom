package main

import (
	"sort"
	"strings"
	"sync/atomic"
)

// Script enablement must refer to a real selection, never the previous session
// or a LastCharacter preference when Login has no selected saved character.
func scriptScopeCharacter() string {
	scriptSessionMu.Lock()
	character, active := scriptSessionCharacter, scriptSessionActive
	scriptSessionMu.Unlock()
	if active {
		return character
	}
	if playingMovie || clmov != "" || pcapPath != "" || fake {
		return strings.TrimSpace(playerName)
	}
	selected := strings.TrimSpace(name)
	if selected == "" || selected == freeDemoSelection {
		return ""
	}
	if _, exists := selectedCharacter(selected); exists {
		return selected
	}
	return ""
}

func scriptScopeDescription(scope scriptScope) string {
	if scope.All {
		return "All players"
	}
	var names []string
	for character, enabled := range scope.Chars {
		if enabled {
			names = append(names, character)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "Disabled"
	}
	return strings.Join(names, ", ")
}

func scriptScopedStatus(scope scriptScope, character string, disabled, invalid bool, errorText string, reloadFailed bool) string {
	if disabled && !invalid && errorText == "" && !scope.empty() {
		if !scope.All && !scope.enablesFor(character) {
			if character == "" {
				return "Waiting for player selection"
			}
			return "Enabled for another player"
		}
		if scriptExecutionCharacter() == "" {
			return "Waiting for login"
		}
	}
	return scriptStatusLabel(disabled, invalid, errorText, reloadFailed)
}

func scriptExecutionCharacter() string {
	scriptSessionMu.Lock()
	defer scriptSessionMu.Unlock()
	if !scriptSessionActive {
		return ""
	}
	return scriptSessionCharacter
}

// Main-thread lifecycle boundaries. Tokens prevent a delayed disconnect from
// an earlier connection (including the same character) stopping a new session.
var scriptSessionGeneration atomic.Uint64

func startSessionScripts(character string) uint64 {
	stopScripts("session restart")
	scriptSessionMu.Lock()
	previous := scriptSessionCharacter
	scriptSessionActive = false
	scriptSessionMu.Unlock()
	switchCharacterProfile(character)
	clearCommands()
	scriptSessionGeneration.Add(1)
	clearScriptLatestServerMessage()
	setScriptLocation("")
	scriptChangeMu.Lock()
	scriptChanges = scriptChangeSnapshot{}
	scriptChangeMu.Unlock()
	scriptSessionLogin(character)
	applyEnabledScripts()
	if previous != "" && !strings.EqualFold(previous, character) {
		dispatchScriptLifecycle(LifecycleEvent{Type: lifecycleCharacterChange, Character: character, PreviousCharacter: previous})
	}
	dispatchScriptLifecycle(LifecycleEvent{Type: lifecycleLogin, Character: character, PreviousCharacter: previous})
	refreshscriptsWindow()
	refreshscriptDetails()
	return scriptSessionGeneration.Load()
}

func endSessionScripts(generation uint64) {
	if generation == 0 || generation != scriptSessionGeneration.Load() {
		return
	}
	character := scriptExecutionCharacter()
	scriptSessionMu.Lock()
	if !scriptSessionActive {
		scriptSessionMu.Unlock()
		return
	}
	scriptSessionActive = false
	scriptSessionMu.Unlock()
	stopScriptMovement("")
	// Complete logout callbacks before unregistering them and running Terminate.
	chatHandlersMu.RLock()
	handlers := append([]scriptLifecycleHandler(nil), scriptLifecycleHandlers...)
	chatHandlersMu.RUnlock()
	for _, handler := range handlers {
		if handler.kind == lifecycleLogout && handler.fn != nil {
			fn := handler.fn
			queueScriptCallbackWaitOn(handler.queue, handler.owner, "Lifecycle logout", func() {
				fn(LifecycleEvent{Type: lifecycleLogout, Character: character})
			})
		}
	}
	stopScripts("session ended")
	clearCommands()
	clearScriptLatestServerMessage()
	refreshscriptsWindow()
	refreshscriptDetails()
}

// Retained functions from a stopped interpreter must not read a later session
// or act with a replacement interpreter's permissions. Terminate may explicitly
// persist storage, but cannot resume controls or inspect another session.
func (c *scriptCandidate) apiCallAllowed(owner, permission string) bool {
	if c == nil {
		return true
	}
	c.mu.Lock()
	active, failed, terminating, queue := c.active, c.failed, c.terminating, c.eventQueue
	c.mu.Unlock()
	if failed {
		return false
	}
	if !active {
		return true
	}
	if c.session != nil {
		return scriptEventQueueIsCurrent(owner, queue) || (terminating && permission == "storage")
	}
	if c.generation != scriptSessionGeneration.Load() {
		return false
	}
	return scriptEventQueueIsCurrent(owner, queue) || (terminating && permission == "storage")
}
