package main

import (
	"sync"
	"time"
)

type timedMessage struct {
	Text string
	Type string
	Time time.Time
}

type messageLog struct {
	mu      sync.Mutex
	entries []timedMessage
	max     int
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
	entry := timedMessage{Text: msg, Type: messageType, Time: time.Now()}

	l.mu.Lock()
	l.entries = append(l.entries, entry)
	if len(l.entries) > l.max {
		l.entries = l.entries[len(l.entries)-l.max:]
	}
	l.mu.Unlock()
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
