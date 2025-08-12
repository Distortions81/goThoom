//go:build test

package eui

import (
	"bytes"
	"log"
	"strings"
	"testing"
	"time"
)

func TestEmitDropsWhenChannelFull(t *testing.T) {
	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(orig)

	h := &EventHandler{Events: make(chan UIEvent, 1)}
	h.Events <- UIEvent{Type: EventClick}

	done := make(chan struct{})
	go func() {
		h.Emit(UIEvent{Type: EventSliderChanged})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("emit blocked")
	}

	if len(h.Events) != 1 {
		t.Fatalf("expected 1 event in channel, got %d", len(h.Events))
	}

	ev := <-h.Events
	if ev.Type != EventClick {
		t.Fatalf("expected EventClick, got %v", ev.Type)
	}

	if !strings.Contains(buf.String(), "dropping event") {
		t.Fatalf("expected log to report dropped event, got %q", buf.String())
	}
}
