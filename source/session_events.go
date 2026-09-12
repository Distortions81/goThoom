package main

import (
	"sync"
	"time"
)

const maxSessionEvents = 1000

type sessionEventKind string

const (
	sessionEventChat        sessionEventKind = "chat"
	sessionEventConsole     sessionEventKind = "console"
	sessionEventThink       sessionEventKind = "think"
	sessionEventSound       sessionEventKind = "sound"
	sessionEventInfoCommand sessionEventKind = "info-command"
)

// sessionEvent is detached data produced while decoding one session. The UI,
// audio mixer, and future script runtimes consume these events without needing
// an implicit current character.
type sessionEvent struct {
	Source      SessionID
	Kind        sessionEventKind
	Text        string
	MessageType string
	SoundIDs    []uint16
	At          time.Time
}

type sessionEventLog struct {
	mu      sync.Mutex
	entries []sessionEvent
	max     int
}

func newSessionEventLog(maxEntries int) *sessionEventLog {
	return &sessionEventLog{max: maxEntries}
}

func (l *sessionEventLog) add(event sessionEvent) {
	if l == nil {
		return
	}
	event.SoundIDs = append([]uint16(nil), event.SoundIDs...)
	if event.At.IsZero() {
		event.At = time.Now()
	}
	l.mu.Lock()
	l.entries = append(l.entries, event)
	if l.max > 0 && len(l.entries) > l.max {
		l.entries = append(l.entries[:0], l.entries[len(l.entries)-l.max:]...)
	}
	l.mu.Unlock()
}

func (l *sessionEventLog) snapshot() []sessionEvent {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]sessionEvent, len(l.entries))
	copy(out, l.entries)
	for i := range out {
		out[i].SoundIDs = append([]uint16(nil), out[i].SoundIDs...)
	}
	return out
}

type sessionEventState struct {
	log *sessionEventLog

	soundMu     sync.Mutex
	prevSounds  []uint16
	prev2Sounds []uint16
}

func newSessionEventState() *sessionEventState {
	return &sessionEventState{log: newSessionEventLog(maxSessionEvents)}
}

var combinedSessionEvents = newSessionEventLog(maxSessionEvents)

func (s *Session) publishEvent(event sessionEvent) {
	if s == nil || s.events == nil {
		return
	}
	event.Source = s.ID()
	if event.At.IsZero() {
		event.At = time.Now()
	}
	s.events.log.add(event)
	combinedSessionEvents.add(event)
}

func (s *Session) publishChat(text, messageType string) {
	if text == "" {
		return
	}
	s.publishEvent(sessionEvent{Kind: sessionEventChat, Text: text, MessageType: messageType})
	if s == primarySession {
		displayChatMessageTyped(text, messageType)
	}
}

func (s *Session) publishConsole(text, messageType string) {
	if text == "" {
		return
	}
	s.publishEvent(sessionEvent{Kind: sessionEventConsole, Text: text, MessageType: messageType})
	if s == primarySession {
		serverConsoleMessageTyped(text, messageType)
	}
}

func (s *Session) publishThink(text string) {
	if text == "" {
		return
	}
	s.publishEvent(sessionEvent{Kind: sessionEventThink, Text: text, MessageType: messageTextTypeThink})
	if s == primarySession {
		showThinkMessage(text)
	}
}

func (s *Session) publishInfoCommand(text string) {
	if text == "" {
		return
	}
	s.publishEvent(sessionEvent{Kind: sessionEventInfoCommand, Text: text})
}

func (s *Session) filterSounds(ids []uint16, throttle bool) []uint16 {
	if s == nil || s.events == nil {
		return nil
	}
	s.events.soundMu.Lock()
	defer s.events.soundMu.Unlock()
	filtered := make([]uint16, 0, len(ids))
	for _, id := range ids {
		if throttle && (containsSoundID(s.events.prevSounds, id) || containsSoundID(s.events.prev2Sounds, id)) {
			continue
		}
		filtered = appendUniqueSound(filtered, id)
	}
	s.events.prev2Sounds = append(s.events.prev2Sounds[:0], s.events.prevSounds...)
	s.events.prevSounds = append(s.events.prevSounds[:0], filtered...)
	return filtered
}

func containsSoundID(ids []uint16, id uint16) bool {
	for _, existing := range ids {
		if existing == id {
			return true
		}
	}
	return false
}

func (s *Session) publishSounds(ids []uint16) {
	if len(ids) == 0 {
		return
	}
	s.publishEvent(sessionEvent{Kind: sessionEventSound, SoundIDs: ids})
	playSessionSound(s, ids)
}
