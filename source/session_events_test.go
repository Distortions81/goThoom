package main

import (
	"fmt"
	"testing"
)

func TestSessionEventLogRetainsChronologicalHistory(t *testing.T) {
	for _, limit := range []int{1, 3, 0} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			log := newSessionEventLog(limit)
			if got := log.snapshot(); len(got) != 0 {
				t.Fatalf("empty log = %v", got)
			}
			for i := range 10 {
				ids := []uint16{uint16(i)}
				log.add(sessionEvent{Text: fmt.Sprint(i), SoundIDs: ids})
				ids[0] = 99
				wantLen := i + 1
				if limit > 0 {
					wantLen = min(wantLen, limit)
				}
				got := log.snapshot()
				if len(got) != wantLen {
					t.Fatalf("after event %d: length = %d, want %d", i, len(got), wantLen)
				}
				for j, event := range got {
					want := i + 1 - wantLen + j
					if event.Text != fmt.Sprint(want) || event.SoundIDs[0] != uint16(want) {
						t.Fatalf("event %d = %+v, want %d", j, event, want)
					}
					got[j].SoundIDs[0] = 98
				}
			}
		})
	}
}

func TestSessionLatestEventText(t *testing.T) {
	session := mustNewSession(primarySessionID)
	session.events.log = newSessionEventLog(3)
	if got := session.latestEventText(); got != "" {
		t.Fatalf("empty log text = %q", got)
	}
	session.events.log.add(sessionEvent{Kind: sessionEventConsole, Text: "console"})
	session.events.log.add(sessionEvent{Kind: sessionEventSound})
	if got := session.latestEventText(); got != "console" {
		t.Fatalf("latest text = %q, want console", got)
	}
	session.events.log.add(sessionEvent{Kind: sessionEventChat, Text: "chat"})
	session.events.log.add(sessionEvent{Kind: sessionEventSound})
	if got := session.latestEventText(); got != "chat" {
		t.Fatalf("latest text after wrap = %q, want chat", got)
	}
	session.events.log.add(sessionEvent{Kind: sessionEventSound})
	session.events.log.add(sessionEvent{Kind: sessionEventSound})
	if got := session.latestEventText(); got != "" {
		t.Fatalf("evicted text = %q", got)
	}
}

func BenchmarkSessionEventLogAdd(b *testing.B) {
	log := newSessionEventLog(maxSessionEvents)
	event := sessionEvent{Kind: sessionEventChat, Text: "Alice says, hello"}
	for range maxSessionEvents {
		log.add(event)
	}
	b.ReportAllocs()
	for b.Loop() {
		log.add(event)
	}
}

var benchmarkSessionEventText string

func BenchmarkSessionLatestEventText(b *testing.B) {
	session := mustNewSession(primarySessionID)
	for range maxSessionEvents {
		session.events.log.add(sessionEvent{Kind: sessionEventChat, Text: "Alice says, hello"})
	}
	b.ReportAllocs()
	for b.Loop() {
		benchmarkSessionEventText = session.latestEventText()
	}
}
