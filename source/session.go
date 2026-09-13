package main

import (
	"fmt"
	"sync"
)

const maxSessions = 4

// SessionID is the stable internal identity of one live or login-ready game
// session. Character names are presentation data and must never be used as the
// ownership key because names can be unknown during login or duplicated.
type SessionID uint8

const primarySessionID SessionID = 1

func (id SessionID) Valid() bool {
	return id >= primarySessionID && id <= SessionID(maxSessions)
}

func sessionIDForSlot(slot int) (SessionID, bool) {
	if slot < 0 || slot >= maxSessions {
		return 0, false
	}
	id := SessionID(slot + 1)
	return id, true
}

func (id SessionID) Slot() (int, bool) {
	if !id.Valid() {
		return 0, false
	}
	return int(id) - 1, true
}

// Session owns state that must not cross character connections. More
// subsystems move here incrementally while the primary session preserves the
// current one-client UI and call sites.
type Session struct {
	id         SessionID
	inventory  *inventoryState
	commands   *commandState
	frames     *frameState
	timing     *networkTimingState
	draw       *sessionDrawState
	events     *sessionEventState
	players    *sessionPlayerState
	automation *sessionAutomationState
	transport  *sessionTransportState
	input      *sessionInputState
	login      *sessionLoginState

	identityMu  sync.RWMutex
	character   string
	playerIndex uint8
}

func newSession(id SessionID) (*Session, error) {
	if !id.Valid() {
		return nil, fmt.Errorf("invalid session ID %d", id)
	}
	return &Session{
		id:          id,
		inventory:   newInventoryState(),
		commands:    newCommandState(),
		frames:      newFrameState(),
		timing:      newNetworkTimingState(),
		draw:        newSessionDrawState(),
		events:      newSessionEventState(),
		players:     newSessionPlayerState(),
		automation:  newSessionAutomationState(),
		transport:   newSessionTransportState(),
		input:       newSessionInputState(),
		login:       newSessionLoginState(),
		playerIndex: 0xff,
	}, nil
}

func (s *Session) setPlayerIndex(index uint8) {
	if s == nil {
		return
	}
	s.identityMu.Lock()
	s.playerIndex = index
	s.identityMu.Unlock()
}

func (s *Session) playerIndexSnapshot() uint8 {
	if s == nil {
		return 0xff
	}
	s.identityMu.RLock()
	index := s.playerIndex
	s.identityMu.RUnlock()
	return index
}

func (s *Session) ID() SessionID {
	if s == nil {
		return 0
	}
	return s.id
}

func (s *Session) setCharacterName(name string) {
	if s == nil {
		return
	}
	s.identityMu.Lock()
	s.character = name
	s.identityMu.Unlock()
}

func (s *Session) characterName() string {
	if s == nil {
		return ""
	}
	s.identityMu.RLock()
	name := s.character
	s.identityMu.RUnlock()
	return name
}

func (s *Session) resetConnectionModels() {
	if s == nil {
		return
	}
	s.draw.reset()
	s.commands.clear()
	s.frames.set(0, -1)
	s.frames.resetStatistics()
	s.timing.drainWake()
	s.timing.resetCadence()
	s.timing.resetReply()
	s.timing.resetController()
	s.timing.resetFallback()
	s.inventory.reset()
	s.players.reset()
	// Secondary script instances are independent of the primary Scripts UI,
	// but they still receive their logout lifecycle before connection teardown.
	s.dispatchSessionScriptLifecycle(LifecycleEvent{Type: lifecycleLogout, Character: s.characterName()}, true)
	s.automation.reset()
	s.setPlayerIndex(0xff)
	s.input.reset()
}

func mustNewSession(id SessionID) *Session {
	session, err := newSession(id)
	if err != nil {
		panic(err)
	}
	return session
}

var primarySession = mustNewSession(primarySessionID)
