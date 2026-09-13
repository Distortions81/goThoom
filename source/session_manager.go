package main

import (
	"context"
	"errors"
	"sync"
)

var queueSelectedSessionUIUpdate = func() {}
var queueSessionWorkspaceUIUpdate = func() {}

// sessionManager is the app-owned registry for the four stable session slots.
// It starts with only the primary session; enabling the multi-session workspace
// materializes the remaining login-ready sessions without replacing slot one.
type sessionManager struct {
	mu       sync.RWMutex
	slots    [maxSessions]*Session
	selected SessionID
	multi    bool
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

func (m *sessionManager) snapshot() [maxSessions]*Session {
	if m == nil {
		return [maxSessions]*Session{}
	}
	m.mu.RLock()
	slots := m.slots
	m.mu.RUnlock()
	return slots
}

func (m *sessionManager) startLogin(ctx context.Context, id SessionID, request sessionLoginRequest, version int) (<-chan error, error) {
	session, ok := m.session(id)
	if !ok {
		return nil, errors.New("session slot is unavailable")
	}
	request = request.normalized()
	if err := request.validate(); err != nil {
		return nil, err
	}
	sessionCtx, cancel := context.WithCancel(ctx)
	if !session.transport.begin(cancel) {
		cancel()
		return nil, errors.New("session transport is busy")
	}
	session.login.setRequest(request)
	session.login.setStatus("Connecting...", nil)
	result := make(chan error, 1)
	go func() {
		err := loginSessionWithDemoCandidates(session, sessionCtx, version, nil)
		if err != nil {
			session.login.setStatus("Disconnected", err)
		} else {
			session.login.setStatus("Disconnected", nil)
		}
		queueSessionWorkspaceUIUpdate()
		result <- err
		close(result)
	}()
	queueSessionWorkspaceUIUpdate()
	return result, nil
}

func (m *sessionManager) disconnectSession(id SessionID) bool {
	session, ok := m.session(id)
	if !ok {
		return false
	}
	disconnected := session.transport.disconnect()
	if disconnected {
		session.login.setStatus("Disconnecting...", nil)
		queueSessionWorkspaceUIUpdate()
	}
	return disconnected
}

func (m *sessionManager) disconnectAll() int {
	if m == nil {
		return 0
	}
	count := 0
	for _, session := range m.snapshot() {
		if session != nil && session.transport.disconnect() {
			session.login.setStatus("Disconnecting...", nil)
			count++
		}
	}
	if count > 0 {
		queueSessionWorkspaceUIUpdate()
	}
	return count
}

func (m *sessionManager) disconnectAllAndWait(ctx context.Context) (int, error) {
	if m == nil {
		return 0, nil
	}
	var pending []<-chan struct{}
	count := 0
	for _, session := range m.snapshot() {
		if session == nil {
			continue
		}
		done := session.transport.doneSnapshot()
		if session.transport.disconnect() {
			session.login.setStatus("Disconnecting...", nil)
			count++
		}
		if done != nil {
			pending = append(pending, done)
		}
	}
	if count > 0 {
		queueSessionWorkspaceUIUpdate()
	}
	for _, done := range pending {
		select {
		case <-done:
		case <-ctx.Done():
			return count, ctx.Err()
		}
	}
	return count, nil
}

func (m *sessionManager) anyBusy() bool {
	for _, session := range m.snapshot() {
		if session != nil && session.transport.busy() {
			return true
		}
	}
	return false
}

func (m *sessionManager) enableMulti() [maxSessions]*Session {
	m.mu.Lock()
	m.multi = true
	for slot := range m.slots {
		if m.slots[slot] != nil {
			continue
		}
		id, _ := sessionIDForSlot(slot)
		m.slots[slot] = mustNewSession(id)
	}
	slots := m.slots
	m.mu.Unlock()
	if m == appSessions {
		appViewports.enableMulti(viewportLayoutFreeform)
		queueMusicSourceUIUpdate()
	}
	return slots
}

func (m *sessionManager) multiEnabled() bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	enabled := m.multi
	m.mu.RUnlock()
	return enabled
}

func (m *sessionManager) disableMulti() bool {
	if m == nil || m.anyBusy() {
		return false
	}
	m.mu.Lock()
	m.multi = false
	m.selected = primarySessionID
	m.mu.Unlock()
	if m == appSessions {
		appViewports.disableMulti()
	}
	queueSelectedSessionUIUpdate()
	queueSessionWorkspaceUIUpdate()
	return true
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
	if m.slots[slot] == nil {
		m.mu.Unlock()
		return false
	}
	changed := m.selected != id
	m.selected = id
	m.mu.Unlock()
	if changed {
		inventoryDirty = true
		playersDirty = true
		queueSelectedSessionUIUpdate()
	}
	return true
}

var appSessions = newSessionManager(primarySession)
