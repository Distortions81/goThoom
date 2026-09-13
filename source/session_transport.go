package main

import (
	"context"
	"net"
	"sync"
)

type sessionConnectionStatus uint8

const (
	sessionDisconnected sessionConnectionStatus = iota
	sessionConnecting
	sessionConnected
)

// sessionTransportState owns the sockets and cancellation boundary for one
// connection. Closing it detaches the sockets before cancellation/Close so a
// concurrent failure cannot disconnect a replacement connection.
type sessionTransportState struct {
	mu         sync.RWMutex
	tcp        net.Conn
	udp        net.Conn
	cancel     context.CancelFunc
	done       chan struct{}
	status     sessionConnectionStatus
	generation uint64
}

func newSessionTransportState() *sessionTransportState {
	return &sessionTransportState{}
}

func (s *sessionTransportState) begin(cancel context.CancelFunc) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status != sessionDisconnected || s.tcp != nil || s.udp != nil || s.done != nil {
		return false
	}
	s.cancel = cancel
	s.done = make(chan struct{})
	s.status = sessionConnecting
	s.generation++
	return true
}

func (s *sessionTransportState) attach(tcp, udp net.Conn) (uint64, bool) {
	if s == nil || tcp == nil || udp == nil {
		return 0, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tcp != nil || s.udp != nil {
		return 0, false
	}
	if s.status == sessionDisconnected {
		s.generation++
	}
	s.tcp = tcp
	s.udp = udp
	s.status = sessionConnected
	return s.generation, true
}

func (s *sessionTransportState) connections() (tcp, udp net.Conn, status sessionConnectionStatus) {
	if s == nil {
		return nil, nil, sessionDisconnected
	}
	s.mu.RLock()
	tcp, udp, status = s.tcp, s.udp, s.status
	s.mu.RUnlock()
	return
}

func (s *sessionTransportState) connected() bool {
	tcp, _, status := s.connections()
	return status == sessionConnected && tcp != nil
}

func (s *sessionTransportState) busy() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	busy := s.status != sessionDisconnected || s.done != nil
	s.mu.RUnlock()
	return busy
}

// finish clears this connection only if the supplied sockets are still the
// active pair. It is safe for stale read loops to call after a reconnect.
func (s *sessionTransportState) finish(generation uint64) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	currentLifecycle := s.generation == generation ||
		(s.generation == generation+1 && s.status == sessionDisconnected && s.tcp == nil && s.udp == nil)
	if currentLifecycle {
		s.tcp = nil
		s.udp = nil
		s.cancel = nil
		s.status = sessionDisconnected
		if s.done != nil {
			close(s.done)
			s.done = nil
		}
	}
	s.mu.Unlock()
	return currentLifecycle
}

func (s *sessionTransportState) failConnect() {
	if s == nil {
		return
	}
	s.mu.Lock()
	var cancel context.CancelFunc
	if (s.status == sessionConnecting || s.status == sessionDisconnected) && s.tcp == nil && s.udp == nil && s.done != nil {
		cancel = s.cancel
		s.cancel = nil
		s.status = sessionDisconnected
		if s.done != nil {
			close(s.done)
			s.done = nil
		}
	}
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *sessionTransportState) disconnect() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	active := s.status != sessionDisconnected || s.tcp != nil || s.udp != nil || s.cancel != nil
	tcp, udp, cancel := s.tcp, s.udp, s.cancel
	s.tcp = nil
	s.udp = nil
	s.cancel = nil
	s.status = sessionDisconnected
	if active {
		s.generation++
	}
	s.mu.Unlock()
	if !active {
		return false
	}
	if cancel != nil {
		cancel()
	}
	if tcp != nil {
		_ = tcp.Close()
	}
	if udp != nil {
		_ = udp.Close()
	}
	return true
}

func (s *sessionTransportState) doneSnapshot() <-chan struct{} {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	done := s.done
	s.mu.RUnlock()
	return done
}
