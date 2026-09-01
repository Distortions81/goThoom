package main

import (
	"sync"
	"time"
)

type timedMessage struct {
	Text string
	Type string
	Time time.Time
	seq  uint64
}

type messageLog struct {
	mu      sync.Mutex
	entries []timedMessage
	max     int
	nextSeq uint64
}

type messageLogDelta struct {
	entries       []timedMessage
	firstSequence uint64
	lastSequence  uint64
	reset         bool
}

type messageWindowState struct {
	messages      []string
	types         []string
	firstSequence uint64
	lastSequence  uint64
	format        string
	timestamps    bool
	dropped       int
	reset         bool
}

func formatTimedMessage(msg timedMessage, format string, timestamps bool) string {
	if timestamps {
		return "[" + msg.Time.Format(format) + "] " + msg.Text
	}
	return msg.Text
}

// Sync advances a window model using only newly retained log entries. The
// returned index is the first row whose wrapping or presentation may need an
// update. A zero index denotes a full rebuild. Front trims return the first
// appended row after the retained rows; callers can adjust alternating styles
// without rewrapping the retained text.
func (state *messageWindowState) Sync(log *messageLog, format string, timestamps, force bool) ([]string, []string, int) {
	state.dropped = 0
	state.reset = false
	if format == "" {
		format = "3:04PM"
	}
	configChanged := state.format != format || state.timestamps != timestamps
	after := state.lastSequence
	if force || configChanged {
		after = 0
	}
	delta := log.Delta(after)
	if force || configChanged {
		delta.reset = true
	}
	firstChanged := len(state.messages)
	if delta.lastSequence == 0 {
		state.messages = state.messages[:0]
		state.types = state.types[:0]
		state.firstSequence = 0
		state.lastSequence = 0
		state.format = format
		state.timestamps = timestamps
		state.reset = true
		return state.messages, state.types, 0
	}
	if delta.reset || state.firstSequence == 0 {
		state.messages = state.messages[:0]
		state.types = state.types[:0]
		state.firstSequence = delta.firstSequence
		state.reset = true
		firstChanged = 0
	} else if delta.firstSequence > state.firstSequence {
		drop := int(delta.firstSequence - state.firstSequence)
		state.dropped = drop
		if drop >= len(state.messages) {
			state.messages = state.messages[:0]
			state.types = state.types[:0]
		} else {
			copy(state.messages, state.messages[drop:])
			copy(state.types, state.types[drop:])
			state.messages = state.messages[:len(state.messages)-drop]
			state.types = state.types[:len(state.types)-drop]
		}
		state.firstSequence = delta.firstSequence
		firstChanged = len(state.messages)
	}
	for _, msg := range delta.entries {
		state.messages = append(state.messages, formatTimedMessage(msg, format, timestamps))
		state.types = append(state.types, msg.Type)
	}
	state.lastSequence = delta.lastSequence
	state.format = format
	state.timestamps = timestamps
	if firstChanged > len(state.messages) {
		firstChanged = len(state.messages)
	}
	return state.messages, state.types, firstChanged
}

// ensureSequencesLocked assigns sequence numbers to logs restored by tests or
// older in-memory state that predates incremental window updates.
func (l *messageLog) ensureSequencesLocked() {
	if len(l.entries) == 0 {
		return
	}
	if l.nextSeq != 0 && l.entries[0].seq != 0 {
		return
	}
	for i := range l.entries {
		l.entries[i].seq = uint64(i + 1)
	}
	l.nextSeq = uint64(len(l.entries))
}

func (l *messageLog) Add(msg string) {
	l.AddTyped(msg, messageTextTypeSystem)
}

func (l *messageLog) AddTyped(msg, messageType string) {
	if msg == "" {
		return
	}
	if wasmPrivacyActive() {
		return
	}
	l.mu.Lock()
	l.ensureSequencesLocked()
	l.nextSeq++
	entry := timedMessage{Text: msg, Type: messageType, Time: time.Now(), seq: l.nextSeq}
	l.entries = append(l.entries, entry)
	if len(l.entries) > l.max {
		l.entries = l.entries[len(l.entries)-l.max:]
	}
	l.mu.Unlock()
}

// Delta returns entries newer than after. If retained history no longer
// contains the caller's next sequence, reset is true and the complete retained
// log is returned instead.
func (l *messageLog) Delta(after uint64) messageLogDelta {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ensureSequencesLocked()
	if len(l.entries) == 0 {
		return messageLogDelta{reset: after != 0}
	}
	first := l.entries[0].seq
	last := l.entries[len(l.entries)-1].seq
	start := len(l.entries)
	reset := after == 0 || after+1 < first || after > last
	if reset {
		start = 0
	} else if after < last {
		start = int(after - first + 1)
		if start < 0 || start > len(l.entries) {
			start = 0
			reset = true
		}
	}
	entries := append([]timedMessage(nil), l.entries[start:]...)
	return messageLogDelta{
		entries:       entries,
		firstSequence: first,
		lastSequence:  last,
		reset:         reset,
	}
}

func (l *messageLog) Entries(format string, useTimestamps bool) []string {
	entries, _ := l.EntriesWithTypes(format, useTimestamps)
	return entries
}

func (l *messageLog) EntriesWithTypes(format string, useTimestamps bool) ([]string, []string) {
	l.mu.Lock()
	entries := make([]timedMessage, len(l.entries))
	copy(entries, l.entries)
	l.mu.Unlock()

	out := make([]string, len(entries))
	types := make([]string, len(entries))
	if format == "" {
		format = "3:04PM"
	}
	if useTimestamps {
		for i, msg := range entries {
			out[i] = "[" + msg.Time.Format(format) + "] " + msg.Text
			types[i] = msg.Type
		}
		return out, types
	}
	for i, msg := range entries {
		out[i] = msg.Text
		types[i] = msg.Type
	}
	return out, types
}
