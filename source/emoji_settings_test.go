package main

import (
	"image"
	"strings"
	"testing"
)

func TestEmojiExpansionSettingPersistence(t *testing.T) {
	if !gsdef.ExpandEmojiNames {
		t.Fatal("emoji name expansion must default on")
	}
	missing, err := unmarshalSettingsDocument([]byte(`{"version":4}`), gsdef)
	if err != nil || !missing.ExpandEmojiNames {
		t.Fatalf("missing setting did not retain default: %v", err)
	}
	for _, enabled := range []bool{false, true} {
		want := cloneSettings(gsdef)
		want.ExpandEmojiNames = enabled
		data, err := marshalSettingsDocument(want)
		if err != nil {
			t.Fatal(err)
		}
		got, err := unmarshalSettingsDocument(data, gsdef)
		if err != nil || got.ExpandEmojiNames != enabled {
			t.Fatalf("setting %v did not round trip: %v", enabled, err)
		}
		profile, err := captureCharacterProfile("Emoji Test", want)
		if err != nil {
			t.Fatal(err)
		}
		got, err = applyCharacterProfile(gsdef, profile)
		if err != nil || got.ExpandEmojiNames != enabled {
			t.Fatalf("profile setting %v did not round trip: %v", enabled, err)
		}
	}
}

func TestEmojiExpansionRefreshesExistingMessagesAndBubbles(t *testing.T) {
	old := gs.ExpandEmojiNames
	t.Cleanup(func() { gs.ExpandEmojiNames = old })
	initFont()
	log := &messageLog{max: 10}
	const raw = "Self: :smile: 😄 :unknown:"
	log.Add(raw)
	var window messageWindowState
	for _, enabled := range []bool{true, false, true} {
		gs.ExpandEmojiNames = enabled
		want := raw
		if enabled {
			want = "Self: 😄 😄 :unknown:"
		}
		msgs, _, changed := window.Sync(log, "", false, false)
		if len(msgs) != 1 || msgs[0] != want || changed != 0 || !window.reset {
			t.Fatalf("enabled=%v: history did not refresh: %q, first changed=%d", enabled, msgs, changed)
		}
		m := measureBubble(raw, kBubbleNormal, 1, 1, image.Pt(600, 200))
		if got := strings.Join(m.lines, ""); strings.ReplaceAll(got, " ", "") != strings.ReplaceAll(want, " ", "") {
			t.Fatalf("enabled=%v: bubble = %q, want %q", enabled, got, want)
		}
		if got := encodeEmojiShortcodes(":smile: 😄"); got != ":smile: :smile:" {
			t.Fatalf("display setting changed wire text: %q", got)
		}
	}
	if log.entries[0].Text != raw {
		t.Fatal("display changed the stored message")
	}
}
