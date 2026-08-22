package main

import "testing"

func TestMessageTextTypeForBubble(t *testing.T) {
	tests := map[int]string{
		kBubbleNormal:       messageTextTypeSay,
		kBubbleWhisper:      messageTextTypeWhisper,
		kBubbleYell:         messageTextTypeYell,
		kBubbleThought:      messageTextTypeThink,
		kBubbleRealAction:   messageTextTypeAction,
		kBubblePlayerAction: messageTextTypeAction,
		kBubblePonder:       messageTextTypePonder,
		kBubbleNarrate:      messageTextTypeNarration,
		kBubbleMonster:      messageTextTypeMonster,
		99:                  messageTextTypeSystem,
	}
	for bubbleType, want := range tests {
		if got := messageTextTypeForBubble(bubbleType); got != want {
			t.Errorf("messageTextTypeForBubble(%d) = %q, want %q", bubbleType, got, want)
		}
	}
}

func TestMessageLogEntriesKeepTypes(t *testing.T) {
	log := messageLog{max: 2}
	log.AddTyped("quiet", messageTextTypeWhisper)
	log.AddTyped("loud", messageTextTypeYell)
	entries, types := log.EntriesWithTypes("3:04PM", false)
	if len(entries) != 2 || len(types) != 2 {
		t.Fatalf("entries/types lengths = %d/%d, want 2/2", len(entries), len(types))
	}
	if types[0] != messageTextTypeWhisper || types[1] != messageTextTypeYell {
		t.Fatalf("types = %q, want whisper/yell", types)
	}
}
