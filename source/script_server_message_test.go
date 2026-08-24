package main

import (
	"testing"

	scriptapi "gt2"
)

func TestLatestServerMessageSnapshot(t *testing.T) {
	scriptLatestServerMessageMu.Lock()
	originalMessage := scriptLatestServerMessageSnapshot
	originalSequence := scriptLatestServerMessageSequence
	originalHasMessage := scriptHasLatestServerMessage
	scriptLatestServerMessageSnapshot = scriptapi.ServerMessage{}
	scriptLatestServerMessageSequence = 0
	scriptHasLatestServerMessage = false
	scriptLatestServerMessageMu.Unlock()
	t.Cleanup(func() {
		scriptLatestServerMessageMu.Lock()
		scriptLatestServerMessageSnapshot = originalMessage
		scriptLatestServerMessageSequence = originalSequence
		scriptHasLatestServerMessage = originalHasMessage
		scriptLatestServerMessageMu.Unlock()
	})

	if _, ok := scriptLatestServerMessage(); ok {
		t.Fatal("latest message should initially be absent")
	}
	runServerMessageHandlers(scriptapi.ServerMessage{Message: "First", Type: "system"})
	first, ok := scriptLatestServerMessage()
	if !ok || first.Message != "First" || first.Type != "system" || first.Sequence != 1 || first.ReceivedAt.IsZero() {
		t.Fatalf("first latest message = %+v, %v", first, ok)
	}
	runServerMessageHandlers(scriptapi.ServerMessage{Message: "Second", Type: "info"})
	second, ok := scriptLatestServerMessage()
	if !ok || second.Message != "Second" || second.Sequence != 2 || second.ReceivedAt.Before(first.ReceivedAt) {
		t.Fatalf("second latest message = %+v, %v", second, ok)
	}
}
