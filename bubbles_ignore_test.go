package main

import "testing"

// buildDrawData constructs minimal draw state packet containing a single bubble
// from the given player.
func buildDrawData(name string, bubbleType int, text string) []byte {
	bubble := []byte{1, byte(bubbleType)}
	bubble = append(bubble, encodeMacRoman(text)...)
	bubble = append(bubble, 0)
	stateData := []byte{0, 1}
	stateData = append(stateData, bubble...)
	stateData = append(stateData, 0) // sound count
	stateData = append(stateData, 0) // inventory
	stateLen := len(stateData)

	desc := []byte{1, kDescPlayer, 0, 0}
	desc = append(desc, encodeMacRoman(name)...)
	desc = append(desc, 0) // terminator
	desc = append(desc, 0) // color count

	buf := make([]byte, 0, 32+stateLen)
	buf = append(buf, 0)                  // ackCmd
	buf = append(buf, make([]byte, 8)...) // ackFrame/resendFrame
	buf = append(buf, 1)                  // descriptor count
	buf = append(buf, desc...)
	buf = append(buf, make([]byte, 7)...) // stats
	buf = append(buf, 0)                  // picture count
	buf = append(buf, 0)                  // mobile count
	buf = append(buf, byte(stateLen>>8), byte(stateLen))
	buf = append(buf, stateData...)
	return buf
}

func resetTestState() {
	resetDrawState()
	players = make(map[string]*Player)
	thinkMessages = nil
}

func TestBubbleDroppedForBlockedPlayer(t *testing.T) {
	resetTestState()
	players["Bob"] = &Player{Name: "Bob", Blocked: true}
	data := buildDrawData("Bob", kBubbleNormal, "hello")
	if _, _, err := parseDrawState(data, false); err != nil {
		t.Fatalf("parseDrawState: %v", err)
	}
	stateMu.Lock()
	got := len(state.bubbles)
	stateMu.Unlock()
	if got != 0 {
		t.Fatalf("expected no bubbles, got %d", got)
	}
}

func TestThinkMessageDroppedForIgnoredPlayer(t *testing.T) {
	resetTestState()
	players["Bob"] = &Player{Name: "Bob", Ignored: true}
	data := buildDrawData("Bob", kBubbleThought, "hmm")
	if _, _, err := parseDrawState(data, false); err != nil {
		t.Fatalf("parseDrawState: %v", err)
	}
	if len(thinkMessages) != 0 {
		t.Fatalf("expected no think messages, got %d", len(thinkMessages))
	}
}
