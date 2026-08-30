package main

import (
	"testing"

	"gothoom/eui"
)

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

func TestMonsterGrowlsRouteToChat(t *testing.T) {
	if !isChatBubble(kBubbleMonster) {
		t.Fatal("monster growls route to Console instead of Chat")
	}
}

func TestMessageTextPalettesAndThemeDefault(t *testing.T) {
	original := gs
	t.Cleanup(func() { gs = original })
	gs.MessageTextColors = nil
	gs.MessageTextColorsLight = nil
	gs.OverrideThemeTextColor = false

	darkSay := messageTextColorForPalette(messageTextTypeSay, false)
	lightSay := messageTextColorForPalette(messageTextTypeSay, true)
	if darkSay == (eui.Color{R: 255, G: 255, B: 255, A: 255}) {
		t.Fatal("dark-theme Say color is still white")
	}
	if darkSay == lightSay {
		t.Fatal("light and dark message palettes use the same Say color")
	}
	if _, force := messageTextStyle(messageTextTypeSystem); force {
		t.Fatal("ordinary text overrides the theme without opt-in")
	}
	if _, force := messageTextStyle(messageTextTypeSay); !force {
		t.Fatal("Say did not apply its semantic highlight color")
	}
	gs.OverrideThemeTextColor = true
	if _, force := messageTextStyle(messageTextTypeSystem); !force {
		t.Fatal("ordinary text ignored the theme-color override")
	}
}

func TestLegacyMessagePaletteUpgradesWhiteSayDefault(t *testing.T) {
	original := gs
	t.Cleanup(func() { gs = original })
	gs.MessageTextColors = legacyMessageTextColors()
	gs.MessageTextColorsLight = nil
	ensureMessageTextColors()
	if got := gs.MessageTextColors[messageTextTypeSay]; got == (eui.Color{R: 255, G: 255, B: 255, A: 255}) {
		t.Fatal("legacy white Say default was not upgraded")
	}
	if gs.MessageTextColorsLight == nil {
		t.Fatal("light-theme message palette was not initialized")
	}
}

func TestClassicMessageColorsMatchClassicClientDefaults(t *testing.T) {
	originalSettings := gs
	originalPlayerName := playerName
	playersMu.Lock()
	originalPlayers := players
	players = map[string]*Player{
		utfFold("Best Friend"): {Name: "Best Friend", FriendLabel: 1},
	}
	playersMu.Unlock()
	t.Cleanup(func() {
		gs = originalSettings
		playerName = originalPlayerName
		playersMu.Lock()
		players = originalPlayers
		playersMu.Unlock()
	})
	playerName = "Hero"
	gs.ClassicMessageColors = true

	tests := []struct {
		name        string
		messageType string
		message     string
		want        eui.Color
	}{
		{"ordinary speech", messageTextTypeSay, `Random Stranger says, "Hello."`, eui.NewColor(0, 0, 0, 255)},
		{"labeled friend speech", messageTextTypeSay, `Best Friend says, "Hello."`, eui.NewColor(0, 80, 0, 255)},
		{"self speech", messageTextTypeSay, `Hero says, "Hello."`, eui.NewColor(80, 0, 51, 255)},
		{"private think-to", messageTextTypeThink, `Someone thinks to you, "Congrats!"`, eui.NewColor(44, 102, 0, 255)},
		{"ordinary thought", messageTextTypeThink, `Someone thinks, "Hmm."`, eui.NewColor(0, 0, 0, 255)},
		{"ordinary info", messageTextTypeSystem, "Welcome.", eui.NewColor(0, 0, 0, 255)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			color, force := messageTextStyleForMessage(test.messageType, test.message)
			if color != test.want || !force {
				t.Fatalf("style = %#v, force %v; want %#v, true", color, force, test.want)
			}
		})
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
