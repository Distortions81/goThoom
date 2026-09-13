package main

import (
	"context"
	"errors"
	"strings"
	"sync"
)

// sessionLoginRequest is the connection identity captured for one session.
// Network login code uses a value snapshot so later UI edits for another slot
// cannot change an in-flight authentication attempt.
type sessionLoginRequest struct {
	host         string
	character    string
	password     string
	passwordHash string
}

func (r sessionLoginRequest) normalized() sessionLoginRequest {
	r.host = strings.TrimSpace(r.host)
	r.character = strings.TrimSpace(r.character)
	return r
}

func (r sessionLoginRequest) validate() error {
	if r.host == "" {
		return errors.New("server address is empty")
	}
	if r.character == "" {
		return errors.New("character name is empty")
	}
	if r.password == "" && r.passwordHash == "" {
		return errors.New("character password required")
	}
	return nil
}

type sessionLoginState struct {
	mu                   sync.Mutex
	request              sessionLoginRequest
	staged               *stagedPasswordUpdate
	status               string
	lastErr              string
	supervisorCancel     context.CancelFunc
	supervisorDone       chan struct{}
	supervisorGeneration uint64
	supervisorCanceled   bool
}

func (s *sessionLoginState) setStatus(status string, err error) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.status = strings.TrimSpace(status)
	s.lastErr = ""
	if err != nil {
		s.lastErr = err.Error()
	}
	s.mu.Unlock()
}

func (s *sessionLoginState) statusSnapshot() (string, string) {
	if s == nil {
		return "", ""
	}
	s.mu.Lock()
	status, lastErr := s.status, s.lastErr
	s.mu.Unlock()
	return status, lastErr
}

func newSessionLoginState() *sessionLoginState {
	return &sessionLoginState{}
}

func (s *sessionLoginState) beginSupervisor(cancel context.CancelFunc) (uint64, bool) {
	if s == nil || cancel == nil {
		return 0, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.supervisorCancel != nil {
		return 0, false
	}
	s.supervisorGeneration++
	s.supervisorCancel = cancel
	s.supervisorDone = make(chan struct{})
	s.supervisorCanceled = false
	return s.supervisorGeneration, true
}

func (s *sessionLoginState) finishSupervisor(generation uint64) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	current := s.supervisorGeneration == generation
	if current {
		s.supervisorCancel = nil
		s.supervisorCanceled = false
		if s.supervisorDone != nil {
			close(s.supervisorDone)
			s.supervisorDone = nil
		}
	}
	s.mu.Unlock()
	return current
}

func (s *sessionLoginState) cancelSupervisor() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	cancel := s.supervisorCancel
	if cancel != nil {
		s.supervisorCanceled = true
	}
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		return true
	}
	return false
}

func (s *sessionLoginState) supervisorActive() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	active := s.supervisorCancel != nil
	s.mu.Unlock()
	return active
}

func (s *sessionLoginState) supervisorCurrent(generation uint64) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	current := s.supervisorCancel != nil && s.supervisorGeneration == generation
	s.mu.Unlock()
	return current
}

func (s *sessionLoginState) reconnectExpected() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	expected := s.supervisorCancel != nil && !s.supervisorCanceled
	s.mu.Unlock()
	return expected
}

func (s *sessionLoginState) supervisorDoneSnapshot() <-chan struct{} {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	done := s.supervisorDone
	s.mu.Unlock()
	return done
}

func (s *sessionLoginState) setRequest(request sessionLoginRequest) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.request = request.normalized()
	s.mu.Unlock()
}

func (s *sessionLoginState) requestSnapshot() sessionLoginRequest {
	if s == nil {
		return sessionLoginRequest{}
	}
	s.mu.Lock()
	request := s.request
	s.mu.Unlock()
	return request
}

func (s *sessionLoginState) setDemoCandidate(candidate string) sessionLoginRequest {
	if s == nil {
		return sessionLoginRequest{}
	}
	s.mu.Lock()
	s.request.character = strings.TrimSpace(candidate)
	s.request.password = "demo"
	s.request.passwordHash = ""
	request := s.request
	s.mu.Unlock()
	return request
}

func (s *sessionLoginState) clearCredentials() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.request.password = ""
	s.request.passwordHash = ""
	s.mu.Unlock()
}

func (s *sessionLoginState) stagePassword(character, password string, remember bool) string {
	if s == nil {
		return ""
	}
	hash := hashPassword(password)
	s.mu.Lock()
	s.staged = &stagedPasswordUpdate{
		character: character,
		hash:      hash,
		remember:  remember,
	}
	s.mu.Unlock()
	return hash
}

func (s *sessionLoginState) stagedPasswordHash(character string) (string, bool) {
	if s == nil {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.staged == nil || !strings.EqualFold(s.staged.character, character) {
		return "", false
	}
	return s.staged.hash, true
}

func (s *sessionLoginState) stagedPasswordSettings(character string) (hash string, remember bool, ok bool) {
	if s == nil {
		return "", false, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.staged == nil || !strings.EqualFold(s.staged.character, character) {
		return "", false, false
	}
	return s.staged.hash, s.staged.remember, true
}

func (s *sessionLoginState) updateStagedPasswordRemember(character string, remember bool) (hash string, ok bool) {
	if s == nil {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.staged == nil || !strings.EqualFold(s.staged.character, character) {
		return "", false
	}
	s.staged.remember = remember
	return s.staged.hash, true
}

func (s *sessionLoginState) discardStagedPasswordFor(character string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.staged != nil && strings.EqualFold(s.staged.character, character) {
		s.staged = nil
	}
	s.mu.Unlock()
}

func (s *sessionLoginState) takeStagedPassword(character string) (stagedPasswordUpdate, bool) {
	if s == nil {
		return stagedPasswordUpdate{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.staged == nil || !strings.EqualFold(s.staged.character, character) {
		return stagedPasswordUpdate{}, false
	}
	update := *s.staged
	s.staged = nil
	return update, true
}

func (s *sessionLoginState) discardStagedPassword() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.staged = nil
	s.mu.Unlock()
}
