package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var queueSelectedSessionUIUpdate = func() {}
var queueSessionWorkspaceUIUpdate = func() {}

// sessionManager is the app-owned registry for the stable session-tab slots.
// It starts with the primary tab and materializes additional sessions on demand.
type sessionManager struct {
	mu       sync.RWMutex
	slots    [maxSessions]*Session
	primary  *Session
	selected SessionID
	login    func(*Session, context.Context, int, []string) error
	wait     func(context.Context, time.Duration) error
}

func (m *sessionManager) anyConnected() bool {
	if m == nil {
		return false
	}
	for _, session := range m.snapshot() {
		if session != nil && session.transport.connected() {
			return true
		}
	}
	return false
}

const (
	sessionReconnectInitialDelay = time.Second
	sessionReconnectMaximumDelay = 30 * time.Second
)

func newSessionManager(primary *Session) *sessionManager {
	if primary == nil || primary.ID() != primarySessionID {
		panic("session manager requires the primary session in slot one")
	}
	manager := &sessionManager{primary: primary, selected: primarySessionID}
	manager.slots[0] = primary
	return manager
}

func (m *sessionManager) count() int {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, session := range m.slots {
		if session != nil {
			count++
		}
	}
	return count
}

// addSession opens and selects the first available tab slot.
func (m *sessionManager) addSession() (*Session, bool) {
	if m == nil {
		return nil, false
	}
	m.mu.Lock()
	var added *Session
	for slot := range m.slots {
		if m.slots[slot] != nil {
			continue
		}
		id, _ := sessionIDForSlot(slot)
		if id == primarySessionID {
			added = m.primary
		} else {
			added = mustNewSession(id)
		}
		m.slots[slot] = added
		m.selected = id
		break
	}
	m.mu.Unlock()
	if added == nil {
		return nil, false
	}
	if m == appSessions {
		selectSessionAudioSource(added.ID())
		markMultiSessionWorkspaceUsed()
		multiSessionWorkspace.Selected = added.ID()
		multiSessionWorkspaceDirty = true
		queueSelectedSessionUIUpdate()
		queueSessionWorkspaceUIUpdate()
	}
	return added, true
}

// closeSession removes one tab. At least one tab always remains open.
func (m *sessionManager) closeSession(id SessionID) bool {
	if m == nil {
		return false
	}
	slot, ok := id.Slot()
	if !ok {
		return false
	}
	m.mu.Lock()
	closing := m.slots[slot]
	if closing == nil {
		m.mu.Unlock()
		return false
	}
	count := 0
	for _, session := range m.slots {
		if session != nil {
			count++
		}
	}
	if count <= 1 {
		m.mu.Unlock()
		return false
	}
	m.slots[slot] = nil
	if m.selected == id {
		for offset := 1; offset <= len(m.slots); offset++ {
			candidate := (slot + offset) % len(m.slots)
			if m.slots[candidate] != nil {
				m.selected, _ = sessionIDForSlot(candidate)
				break
			}
		}
	}
	selected := m.selected
	m.mu.Unlock()
	closing.login.cancelSupervisor()
	closing.transport.disconnect()
	if m == appSessions {
		selectSessionAudioSource(selected)
		markMultiSessionWorkspaceUsed()
		multiSessionWorkspace.Selected = selected
		multiSessionWorkspaceDirty = true
		queueSelectedSessionUIUpdate()
		queueSessionWorkspaceUIUpdate()
	}
	return true
}

// restoreTabs materializes persisted tabs without starting any connections.
func (m *sessionManager) restoreTabs(open [maxSessions]bool, selected SessionID) {
	if m == nil {
		return
	}
	if !anyOpenSessionTabs(open) {
		open[0] = true
	}
	m.mu.Lock()
	for slot, shouldOpen := range open {
		if !shouldOpen {
			m.slots[slot] = nil
			continue
		}
		if m.slots[slot] != nil {
			continue
		}
		id, _ := sessionIDForSlot(slot)
		if id == primarySessionID {
			m.slots[slot] = m.primary
		} else {
			m.slots[slot] = mustNewSession(id)
		}
	}
	selectedSlot, selectedValid := selected.Slot()
	if !selectedValid || m.slots[selectedSlot] == nil {
		for slot, session := range m.slots {
			if session != nil {
				m.selected, _ = sessionIDForSlot(slot)
				break
			}
		}
	} else {
		m.selected = selected
	}
	m.mu.Unlock()
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
	return m.startLoginWithCandidates(ctx, id, request, version, nil)
}

func (m *sessionManager) startLoginWithCandidates(ctx context.Context, id SessionID, request sessionLoginRequest, version int, demoCandidates []string) (<-chan error, error) {
	session, ok := m.session(id)
	if !ok {
		return nil, errors.New("session slot is unavailable")
	}
	request = request.normalized()
	// The standalone login surface starts a demo login with its built-in
	// placeholder selected. Materialize the first server-provided demo identity
	// before validating and storing the supervisor request. Subsequent candidates
	// are still selected by loginSessionWithDemoCandidates when a slot is busy.
	if len(demoCandidates) > 0 {
		request.character = demoCandidates[0]
		request.password = "demo"
		request.passwordHash = ""
		request = request.normalized()
	}
	if err := request.validate(); err != nil {
		return nil, err
	}
	if session.connectionBusy() {
		return nil, errors.New("session transport is busy")
	}
	supervisorCtx, cancel := context.WithCancel(ctx)
	generation, ok := session.login.beginSupervisor(cancel)
	if !ok {
		cancel()
		return nil, errors.New("session login is already active")
	}
	session.login.setRequest(request)
	session.login.setStatus("Connecting...", nil)
	result := make(chan error, 1)
	go func() {
		defer func() {
			session.login.finishSupervisor(generation)
			queueSessionWorkspaceUIUpdate()
		}()
		defer close(result)
		login := m.login
		if login == nil {
			login = loginSessionWithDemoCandidates
		}
		wait := m.wait
		if wait == nil {
			wait = waitForSessionReconnect
		}
		delay := sessionReconnectInitialDelay
		candidates := append([]string(nil), demoCandidates...)
		var finalErr error
		for {
			err := login(session, supervisorCtx, version, candidates)
			candidates = nil
			if supervisorCtx.Err() != nil {
				finalErr = supervisorCtx.Err()
				break
			}
			if err != nil && !sessionLoginErrorIsTransient(err) {
				finalErr = err
				break
			}
			if err == nil {
				delay = sessionReconnectInitialDelay
			}
			reconnectStatus := fmt.Sprintf("Reconnecting in %s...", delay)
			session.login.setStatus(reconnectStatus, err)
			queueSessionWorkspaceUIUpdate()
			if err := wait(supervisorCtx, delay); err != nil {
				finalErr = err
				break
			}
			if delay < sessionReconnectMaximumDelay {
				delay *= 2
				if delay > sessionReconnectMaximumDelay {
					delay = sessionReconnectMaximumDelay
				}
			}
		}
		if session.login.supervisorCurrent(generation) {
			if finalErr != nil && !errors.Is(finalErr, context.Canceled) {
				session.login.setStatus("Disconnected", finalErr)
			} else {
				session.login.setStatus("Disconnected", nil)
			}
			session.login.clearCredentials()
			if session != primarySession && !session.transport.connected() {
				session.setCharacterName("")
			}
		}
		result <- finalErr
	}()
	queueSessionWorkspaceUIUpdate()
	return result, nil
}

func waitForSessionReconnect(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *Session) connectionBusy() bool {
	return s != nil && (s.transport.busy() || s.login.supervisorActive())
}

func sessionLoginErrorIsTransient(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, errDemoSlotsUsed) {
		return false
	}
	var resultErr *loginResultError
	return !errors.As(err, &resultErr)
}

func (m *sessionManager) disconnectSession(id SessionID) bool {
	session, ok := m.session(id)
	if !ok {
		return false
	}
	disconnected := session.transport.disconnect()
	cancelled := session.login.cancelSupervisor()
	if disconnected || cancelled {
		session.login.setStatus("Disconnecting...", nil)
		queueSessionWorkspaceUIUpdate()
	}
	return disconnected || cancelled
}

func (m *sessionManager) disconnectAll() int {
	if m == nil {
		return 0
	}
	count := 0
	for _, session := range m.snapshot() {
		if session != nil {
			cancelled := session.login.cancelSupervisor()
			if !session.transport.disconnect() && !cancelled {
				continue
			}
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
		transportDone := session.transport.doneSnapshot()
		supervisorDone := session.login.supervisorDoneSnapshot()
		cancelled := session.login.cancelSupervisor()
		if session.transport.disconnect() || cancelled {
			session.login.setStatus("Disconnecting...", nil)
			count++
		}
		if transportDone != nil {
			pending = append(pending, transportDone)
		}
		if supervisorDone != nil {
			pending = append(pending, supervisorDone)
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
		if session != nil && session.connectionBusy() {
			return true
		}
	}
	return false
}

// materializeAllSessions is retained as a compact test setup for code that
// needs every stable session slot.
func (m *sessionManager) materializeAllSessions() [maxSessions]*Session {
	m.mu.Lock()
	for slot := range m.slots {
		if m.slots[slot] != nil {
			continue
		}
		id, _ := sessionIDForSlot(slot)
		m.slots[slot] = mustNewSession(id)
	}
	slots := m.slots
	m.mu.Unlock()
	return slots
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
		if m == appSessions {
			selectSessionAudioSource(id)
			inventoryDirty = true
			playersDirty = true
			markMultiSessionWorkspaceUsed()
			multiSessionWorkspace.Selected = id
			multiSessionWorkspaceDirty = true
			queueSelectedSessionUIUpdate()
		}
	}
	return true
}

var appSessions = newSessionManager(primarySession)
