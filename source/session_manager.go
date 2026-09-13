package main

import "sync"

// sessionManager is the app-owned registry for the four stable session slots.
// It starts with only the primary session; enabling the multi-session workspace
// materializes the remaining login-ready sessions without replacing slot one.
type sessionManager struct {
	mu       sync.RWMutex
	slots    [maxSessions]*Session
	selected SessionID
}

func newSessionManager(primary *Session) *sessionManager {
	if primary == nil || primary.ID() != primarySessionID {
		panic("session manager requires the primary session in slot one")
	}
	manager := &sessionManager{selected: primarySessionID}
	manager.slots[0] = primary
	return manager
}

func (m *sessionManager) session(id SessionID) (*Session, bool) {
	slot, ok := id.Slot()
	if !ok {
		return nil, false
	}
	m.mu.RLock()
	session := m.slots[slot]
	m.mu.RUnlock()
	return session, session != nil
}

func (m *sessionManager) enableMulti() [maxSessions]*Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	for slot := range m.slots {
		if m.slots[slot] != nil {
			continue
		}
		id, _ := sessionIDForSlot(slot)
		m.slots[slot] = mustNewSession(id)
	}
	return m.slots
}

func (m *sessionManager) selectedSession() *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	slot, ok := m.selected.Slot()
	if !ok {
		return nil
	}
	return m.slots[slot]
}

func (m *sessionManager) selectedID() SessionID {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	id := m.selected
	m.mu.RUnlock()
	return id
}

func selectedAppSession() *Session {
	if session := appSessions.selectedSession(); session != nil {
		return session
	}
	return primarySession
}

func (m *sessionManager) selectSession(id SessionID) bool {
	slot, ok := id.Slot()
	if !ok {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.slots[slot] == nil {
		return false
	}
	m.selected = id
	return true
}

var appSessions = newSessionManager(primarySession)
