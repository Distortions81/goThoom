package main

import (
	"strings"
	"testing"

	"gothoom/eui"
)

func TestUnicodeEmojiCatalogGroupsAndShortcodes(t *testing.T) {
	if len(emojiCatalog) != 3944 {
		t.Fatalf("Emoji 17.0 catalog has %d entries", len(emojiCatalog))
	}
	groups := emojiGroupNames()
	if len(groups) != 9 || groups[0] != "Smileys & Emotion" || groups[len(groups)-1] != "Flags" {
		t.Fatalf("groups: %v", groups)
	}
	for _, entry := range emojiCatalog {
		wire := encodeEmojiShortcodes(entry.Emoji)
		if !strings.HasPrefix(wire, ":") || strings.Count(wire, ":") != 2 || expandEmojiShortcodes(wire) != entry.Emoji {
			t.Errorf("%s does not have one round-trippable shortcode: %q", entry.Name, wire)
		}
	}
	if got := encodeEmojiShortcodes("😄"); got != ":smile:" {
		t.Fatalf("existing smile alias changed: %s", got)
	}
}

func TestEmojiPickerSearchAndGroups(t *testing.T) {
	for _, entry := range filterEmojiCatalog("Food & Drink", "") {
		if entry.Group != "Food & Drink" {
			t.Fatal("group filter leaked")
		}
	}
	for _, query := range []string{"smile", ":smile:", "😄", "thumbs_up"} {
		if len(filterEmojiCatalog("Flags", query)) == 0 {
			t.Fatalf("search %q returned no matches", query)
		}
	}
	if len(filterEmojiCatalog("", "no-such-emoji-xyz")) != 0 {
		t.Fatal("unknown query matched")
	}
}

func TestEmojiPickerInsertsIntoDraftWithoutSending(t *testing.T) {
	oldText, oldPos, oldActive, oldSelected := inputText, inputPos, inputActive, selectedMessageInput
	oldConsole, oldChat := consoleWin, chatWin
	oldEnabled := gs.ExpandEmojiNames
	t.Cleanup(func() {
		inputText, inputPos, inputActive, selectedMessageInput = oldText, oldPos, oldActive, oldSelected
		consoleWin, chatWin = oldConsole, oldChat
		gs.ExpandEmojiNames = oldEnabled
	})
	consoleWin, chatWin = nil, nil
	flow := eui.NewColumn()
	inputText = []rune("café PLACE! ")
	inputPos = 5
	inputActive = false
	resetCommandStateForTest(t, 1)
	insertMessageEmoji(flow, ":smile:", 5, 10)
	if string(inputText) != "café :smile:! " || inputPos != 12 || !inputActive || selectedMessageInput != flow {
		t.Fatalf("draft %q cursor %d", string(inputText), inputPos)
	}
	if primarySession.commands.pending != "" || len(primarySession.commands.queue) != 0 {
		t.Fatal("picker sent the draft")
	}
	gs.ExpandEmojiNames = false
	if messageEmojiButton(flow) != nil {
		t.Fatal("emoji button visible when disabled")
	}
}
