package main

import (
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
)

// sessionInputState is the direct-input queue for a non-primary session. The
// current UI continues to use its primary compatibility globals until viewport
// selection can route input into the selected session.
type sessionInputState struct {
	mu         sync.Mutex
	latest     inputState
	queue      []inputState
	stopFrames int
	draft      []rune
	draftPos   int
	history    []string
	historyPos int
	active     bool
	lastClick  ClickInfo
	clicks     map[ebiten.MouseButton]ClickInfo
	lastHover  ClickInfo
	hoverGen   uint64
	hoverValid bool
	macroMoved bool
}

func (s *sessionInputState) beginMacroInputFrame() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.macroMoved = false
	s.mu.Unlock()
}

func (s *sessionInputState) markMacroMoved() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.macroMoved = true
	s.mu.Unlock()
}

func (s *sessionInputState) macroMovedThisFrame() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	moved := s.macroMoved
	s.mu.Unlock()
	return moved
}

func sessionLegacyMacroMovedThisFrame(session *Session) bool {
	if session == nil || session == primarySession {
		return legacyMacroMovedThisFrame()
	}
	return session.input.macroMovedThisFrame()
}

func newSessionInputState() *sessionInputState {
	return &sessionInputState{clicks: make(map[ebiten.MouseButton]ClickInfo)}
}

func (s *sessionInputState) storeClick(info ClickInfo) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.lastClick = info
	if s.clicks == nil {
		s.clicks = make(map[ebiten.MouseButton]ClickInfo)
	}
	s.clicks[info.Button] = info
	s.mu.Unlock()
}

func (s *sessionInputState) clickSnapshot() ClickInfo {
	if s == nil {
		return ClickInfo{}
	}
	s.mu.Lock()
	info := s.lastClick
	s.mu.Unlock()
	return info
}

func (s *sessionInputState) buttonClickSnapshot(button ebiten.MouseButton) (ClickInfo, bool) {
	if s == nil {
		return ClickInfo{}, false
	}
	s.mu.Lock()
	info, ok := s.clicks[button]
	s.mu.Unlock()
	return info, ok
}

func (s *sessionInputState) cachedHover(generation uint64, x, y int16) (ClickInfo, bool) {
	if s == nil {
		return ClickInfo{}, false
	}
	s.mu.Lock()
	info := s.lastHover
	ok := s.hoverValid && s.hoverGen == generation && info.X == x && info.Y == y
	s.mu.Unlock()
	return info, ok
}

func (s *sessionInputState) storeHover(info ClickInfo, generation uint64) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	changed := s.lastHover.OnMobile != info.OnMobile ||
		(s.lastHover.OnMobile && info.OnMobile && s.lastHover.Mobile.Index != info.Mobile.Index)
	s.lastHover = info
	s.hoverGen = generation
	s.hoverValid = true
	s.mu.Unlock()
	return changed
}

func (s *sessionInputState) hoverSnapshot() ClickInfo {
	if s == nil {
		return ClickInfo{}
	}
	s.mu.Lock()
	info := s.lastHover
	s.mu.Unlock()
	return info
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

// enqueue adds direct input for this session without consulting the
// primary-session UI queue. Background macro movement uses the same bounded
// coalescing behavior as ordinary player input.
func (s *sessionInputState) enqueue(input inputState) {
	if s == nil {
		return
	}
	s.mu.Lock()
	switch len(s.queue) {
	case 0:
		if s.latest != input {
			s.queue = append(s.queue, input)
		}
	case 1:
		if s.queue[0] != input {
			s.queue = append(s.queue, input)
		}
	default:
		if s.queue[len(s.queue)-1] != input {
			s.queue[len(s.queue)-1] = input
		}
	}
	s.mu.Unlock()
}

func (s *sessionInputState) latestSnapshot() inputState {
	if s == nil {
		return inputState{}
	}
	s.mu.Lock()
	input := s.latest
	if len(s.queue) > 0 {
		input = s.queue[len(s.queue)-1]
	}
	s.mu.Unlock()
	return input
}

func (s *sessionInputState) setStopFrames(frames int) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.stopFrames = frames
	s.mu.Unlock()
}

type sessionMessageInput struct {
	draft      []rune
	draftPos   int
	history    []string
	historyPos int
	active     bool
}

func (s *sessionInputState) messageSnapshot() sessionMessageInput {
	if s == nil {
		return sessionMessageInput{}
	}
	s.mu.Lock()
	state := sessionMessageInput{
		draft:      append([]rune(nil), s.draft...),
		draftPos:   s.draftPos,
		history:    append([]string(nil), s.history...),
		historyPos: s.historyPos,
		active:     s.active,
	}
	s.mu.Unlock()
	return state
}

func (s *sessionInputState) storeMessage(state sessionMessageInput) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.draft = append(s.draft[:0], state.draft...)
	s.draftPos = state.draftPos
	s.history = append(s.history[:0], state.history...)
	s.historyPos = state.historyPos
	s.active = state.active
	s.mu.Unlock()
}

func (s *sessionInputState) setInputText(text string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.draft = []rune(text)
	s.draftPos = len(s.draft)
	s.active = true
	s.mu.Unlock()
}

func (s *sessionInputState) reset() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.latest = inputState{}
	s.queue = nil
	s.stopFrames = 0
	s.draft = nil
	s.draftPos = 0
	s.history = nil
	s.historyPos = 0
	s.active = false
	s.lastClick = ClickInfo{}
	clear(s.clicks)
	s.lastHover = ClickInfo{}
	s.hoverGen = 0
	s.hoverValid = false
	s.macroMoved = false
	s.mu.Unlock()
}

var boundMessageInputSession = primarySessionID

func bindMessageInputSession(session *Session) {
	if session == nil || session.ID() == boundMessageInputSession {
		return
	}
	if previous, ok := appSessions.session(boundMessageInputSession); ok {
		lastInput := currentSessionInput(previous)
		queueSessionInput(previous, inputState{mouseX: lastInput.mouseX, mouseY: lastInput.mouseY})
		previous.input.setStopFrames(0)
		previous.input.storeMessage(sessionMessageInput{
			draft: inputText, draftPos: inputPos, history: inputHistory,
			historyPos: historyPos, active: inputActive,
		})
	}
	state := session.input.messageSnapshot()
	inputText = append(inputText[:0], state.draft...)
	inputPos = state.draftPos
	inputHistory = append(inputHistory[:0], state.history...)
	historyPos = state.historyPos
	inputActive = state.active
	selectedMessageInput = nil
	walkToggled = false
	keyWalkPrev = false
	keyStopFrames = 0
	boundMessageInputSession = session.ID()
	spellDirty = true
	updateMessageInputWindows()
}

func storeMessageInputSession(session *Session) {
	if session == nil {
		return
	}
	session.input.storeMessage(sessionMessageInput{
		draft: inputText, draftPos: inputPos, history: inputHistory,
		historyPos: historyPos, active: inputActive,
	})
}

func currentSessionInput(session *Session) inputState {
	if session == nil {
		return inputState{}
	}
	if session == primarySession {
		inputMu.Lock()
		input := latestInput
		if len(inputQueue) > 0 {
			input = inputQueue[len(inputQueue)-1]
		}
		inputMu.Unlock()
		return input
	}
	return session.input.latestSnapshot()
}

func queueSessionInput(session *Session, input inputState) {
	if session == nil {
		return
	}
	if session == primarySession {
		queueInput(input)
		return
	}
	session.input.enqueue(input)
}
