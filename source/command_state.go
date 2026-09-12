package main

import (
	"sync"
	"time"

	scriptapi "gt2"
)

// commandState owns the ordered command stream and its acknowledgement
// metadata for exactly one game session.
type commandState struct {
	mu                      sync.Mutex
	number                  uint32
	pending                 string
	pendingID               uint8
	pendingSent             bool
	pendingSentAt           time.Time
	pendingSentFrame        int32
	pendingSentPhase        time.Duration
	pendingSentInterval     time.Duration
	pendingSentPredictively bool
	pendingTicket           *scriptCommandState
	queue                   []queuedCommand
	lastCommandFrame        int32
}

func newCommandState() *commandState {
	return &commandState{number: 1, lastCommandFrame: -1}
}

func (s *commandState) enqueue(cmd string) {
	if cmd == "" {
		return
	}
	s.mu.Lock()
	s.queue = append(s.queue, queuedCommand{text: cmd})
	s.nextLocked()
	s.mu.Unlock()
}

func (s *commandState) next() {
	s.mu.Lock()
	s.nextLocked()
	s.mu.Unlock()
}

func (s *commandState) nextLocked() {
	if s.pending == "" && len(s.queue) > 0 {
		s.pending = s.queue[0].text
		s.pendingTicket = s.queue[0].ticket
		s.queue = s.queue[1:]
		s.pendingID = 0
		s.pendingSent = false
		s.resetPendingTimingLocked()
	}
}

func (s *commandState) resetPendingTimingLocked() {
	s.pendingSentAt = time.Time{}
	s.pendingSentFrame = 0
	s.pendingSentPhase = 0
	s.pendingSentInterval = 0
	s.pendingSentPredictively = false
}

func (s *commandState) nextNumberLocked() uint8 {
	s.number = (s.number + 1) & 0xff
	if s.number == 0 {
		s.number = 1
	}
	return uint8(s.number)
}

type commandAcknowledgement struct {
	sentAt       time.Time
	sentFrame    int32
	sentPhase    time.Duration
	sentInterval time.Duration
	predictive   bool
}

func (s *commandState) acknowledgeAt(ack uint8) (commandAcknowledgement, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending == "" || s.pendingID == 0 {
		return commandAcknowledgement{}, false
	}
	if s.pendingID != ack {
		s.pendingSent = false
		return commandAcknowledgement{}, false
	}
	result := commandAcknowledgement{
		sentAt:       s.pendingSentAt,
		sentFrame:    s.pendingSentFrame,
		sentPhase:    s.pendingSentPhase,
		sentInterval: s.pendingSentInterval,
		predictive:   s.pendingSentPredictively,
	}
	s.pending = ""
	s.pendingTicket = nil
	s.pendingID = 0
	s.pendingSent = false
	s.resetPendingTimingLocked()
	s.nextLocked()
	return result, true
}

func (s *commandState) idle() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pending == "" && len(s.queue) == 0
}

func (s *commandState) enqueueIfIdle(cmd string) bool {
	if cmd == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending != "" || len(s.queue) != 0 {
		return false
	}
	s.pending = cmd
	s.pendingTicket = nil
	s.pendingID = 0
	s.pendingSent = false
	s.resetPendingTimingLocked()
	return true
}

func (s *commandState) clear() {
	s.mu.Lock()
	if s.pendingTicket != nil && s.pendingTicket.status.State == scriptapi.CommandQueued {
		s.pendingTicket.status.State = scriptapi.CommandCancelled
	}
	for _, cmd := range s.queue {
		if cmd.ticket != nil && cmd.ticket.status.State == scriptapi.CommandQueued {
			cmd.ticket.status.State = scriptapi.CommandCancelled
		}
	}
	s.pending = ""
	s.pendingTicket = nil
	s.pendingID = 0
	s.pendingSent = false
	s.resetPendingTimingLocked()
	s.queue = nil
	s.lastCommandFrame = -1
	s.mu.Unlock()
}

func (s *commandState) lastFrameSnapshot() int32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastCommandFrame
}
