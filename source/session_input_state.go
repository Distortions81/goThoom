package main

import "sync"

// sessionInputState is the direct-input queue for a non-primary session. The
// current UI continues to use its primary compatibility globals until viewport
// selection can route input into the selected session.
type sessionInputState struct {
	mu         sync.Mutex
	latest     inputState
	queue      []inputState
	stopFrames int
}

func newSessionInputState() *sessionInputState {
	return &sessionInputState{}
}

func (s *sessionInputState) next() inputState {
	if s == nil {
		return inputState{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.queue) > 0 {
		input := s.queue[0]
		s.latest = input
		s.queue = s.queue[1:]
		if s.stopFrames > 0 && len(s.queue) == 0 && !input.mouseDown {
			input = inputState{mouseDown: true}
			s.stopFrames--
		}
		return input
	}
	input := s.latest
	if s.stopFrames > 0 {
		input = inputState{mouseDown: true}
		s.stopFrames--
	}
	return input
}

func (s *sessionInputState) reset() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.latest = inputState{}
	s.queue = nil
	s.stopFrames = 0
	s.mu.Unlock()
}
