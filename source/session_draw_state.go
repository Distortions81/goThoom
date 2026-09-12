package main

import (
	"sync"
	"sync/atomic"
)

// sessionDrawState owns the decoded world model and its logical clock for one
// session. GPU images and prepared renderer resources remain app-owned.
type sessionDrawState struct {
	mu         sync.Mutex
	current    drawState
	initial    drawState
	frame      int
	generation atomic.Uint64
}

func newSessionDrawState() *sessionDrawState {
	return &sessionDrawState{
		current: emptyDrawState(),
	}
}

func emptyDrawState() drawState {
	return drawState{
		descriptors: make(map[uint8]frameDescriptor),
		mobiles:     make(map[uint8]frameMobile),
		prevMobiles: make(map[uint8]frameMobile),
		prevDescs:   make(map[uint8]frameDescriptor),
	}
}

func (s *sessionDrawState) reset() {
	s.mu.Lock()
	s.current = emptyDrawState()
	s.initial = cloneDrawState(s.current)
	s.frame = 0
	s.generation.Add(1)
	s.mu.Unlock()
}

func (s *sessionDrawState) snapshot() (drawState, int, uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneDrawState(s.current), s.frame, s.generation.Load()
}

func (s *sessionDrawState) markChanged() {
	s.generation.Add(1)
}
