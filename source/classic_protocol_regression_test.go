package main

import (
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	"gothoom/climg"
)

func TestPlainInfoTextRetainsWholeConsoleMessage(t *testing.T) {
	// Classic HandleInfoText passes ordinary strings straight to ShowInfoText;
	// speech bubble headers belong to the separate bubble section of the record.
	s := mustNewSession(2)
	lines := []string{"No room in your pack.", "It is raining.", "HP is full.", "OK"}
	handleSessionInfoText(s, []byte("\r"+strings.Join(lines, "\r")+"\r"))
	events := s.events.log.snapshot()
	if len(events) != len(lines) {
		t.Fatalf("info events = %+v, want %d console messages", events, len(lines))
	}
	for i, want := range lines {
		if events[i].Kind != sessionEventConsole || events[i].Text != want || events[i].MessageType != messageTextTypeSystem {
			t.Errorf("info line %q became %+v", want, events[i])
		}
	}
}

func TestWrappedBEPPStillUpdatesOwningSession(t *testing.T) {
	// Classic ShowInfoText removes dd/tl/gm before parsing the contained BEPP
	// message. A hidden backend response must still refresh the player list.
	for _, wrappers := range [][]string{{"dd"}, {"tl"}, {"gm"}, {"dd", "tl"}, {"dd", "dd"}} {
		t.Run(strings.Join(wrappers, "/"), func(t *testing.T) {
			s := mustNewSession(2)
			other := mustNewSession(3)
			var packet []byte
			for _, prefix := range wrappers {
				packet = append(packet, 0xc2, prefix[0], prefix[1])
			}
			packet = append(packet, 0xc2, 'b', 'e', 0xc2, 'i', 'n')
			packet = append(packet, pnTag("Alice")...)
			packet = append(packet, []byte("\tHuman\tFemale\tFighter\tSun Dragon Clan\x00")...)
			handleSessionInfoText(s, packet)
			alice, ok := s.players.player("Alice")
			if !ok || alice.Race != "Human" || alice.Gender != "Female" || alice.Class != "Fighter" || alice.clan != "Sun Dragon Clan" {
				t.Fatalf("wrapped backend response lost: player=%+v found=%v", alice, ok)
			}
			if _, ok := other.players.player("Alice"); ok {
				t.Fatal("wrapped backend response crossed sessions")
			}
			if events := s.events.log.snapshot(); len(events) != 0 {
				t.Fatalf("backend response displayed console text: %+v", events)
			}
		})
	}
	for _, prefix := range []string{"dd", "tl", "gm"} {
		s := mustNewSession(2)
		packet := append([]byte{0xc2, prefix[0], prefix[1]}, []byte("No room in your pack.\x00ignored")...)
		want := "No room in your pack."
		if prefix == "dd" {
			want = ""
		}
		if got := decodeSessionBEPP(s, packet); got != want {
			t.Errorf("wrapper %s returned %q, want %q", prefix, got, want)
		}
	}
}

func TestShareAddsRecipientWithoutClearingExistingShares(t *testing.T) {
	// Classic TestTextForShare resets recipients for a full list, but the
	// "You begin sharing" acknowledgement only adds the named recipient.
	for _, form := range []struct {
		name, full, begin, stop string
	}{
		{"classic", "You are sharing experiences with ", "You begin sharing your experiences with ", "You are no longer sharing experiences with "},
		{"third person", "Hero is sharing experiences with ", "Hero begins sharing experiences with ", "Hero is no longer sharing experiences with "},
	} {
		t.Run(form.name, func(t *testing.T) {
			s := mustNewSession(2)
			s.setCharacterName("Hero")
			other := mustNewSession(3)
			send := func(prefix, name string) {
				raw := append([]byte(prefix), pnTag(name)...)
				raw = append(raw, '.')
				decodeSessionBEPP(s, append([]byte{0xc2, 's', 'h'}, raw...))
			}
			check := func(bobShares, carolShares bool) {
				t.Helper()
				bob, _ := s.players.player("Bob")
				carol, _ := s.players.player("Carol")
				if bob.Sharee != bobShares || carol.Sharee != carolShares {
					t.Fatalf("recipients = Bob %v, Carol %v; want %v, %v", bob.Sharee, carol.Sharee, bobShares, carolShares)
				}
				if len(other.players.snapshot()) != 0 {
					t.Fatal("share acknowledgement changed another session")
				}
			}
			send(form.full, "Bob")
			check(true, false)
			send(form.begin, "Carol")
			check(true, true)
			send(form.stop, "Carol")
			check(true, false)
			send(form.full, "Carol")
			check(false, true)
			decodeSessionBEPP(s, append([]byte{0xc2, 's', 'u'}, []byte("You are no longer sharing experiences with anyone.")...))
			check(false, false)
		})
	}
}

func TestSpeechCannotExecuteServerDirectives(t *testing.T) {
	oldBlockMusic := blockMusic
	blockMusic = false
	t.Cleanup(func() { blockMusic = oldBlockMusic })

	s := mustNewSession(2)
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Path: "speech-directives.mac", Text: "hold\n{\npause 100\n}\n",
	}})
	runtime := newSessionLegacyMacroRuntime(s, program)
	s.automation.legacyRuntime = runtime
	execution, err := runtime.startFunction("hold")
	if err != nil {
		t.Fatal(err)
	}
	runtime.advance(0)

	// Classic interprets directives only in info text. Every speech shape must
	// preserve literal directive text and leave the receiver's state alone.
	for _, typ := range []int{kBubbleNormal, kBubbleWhisper, kBubbleYell, kBubbleRealAction,
		kBubbleMonster, kBubblePlayerAction, kBubblePonder, kBubbleNarrate} {
		for _, text := range []string{"/nt 100 /sa 90 /cl 0", "/m_interrupt"} {
			data := append([]byte{1, byte(typ)}, encodeMacRoman(text)...)
			data = append(data, 0)
			_, got, _, _, _, _, _ := decodeSessionBubble(s, data)
			if got != text {
				t.Errorf("bubble type %d: text=%q, want %q", typ, got, text)
			}
			if s.night.snapshot().baseLevel != 0 || execution.complete {
				t.Fatalf("bubble type %d executed %q", typ, text)
			}
		}
	}

	handleSessionInfoText(s, []byte("/nt 83 /sa -1 /cl 1\r"))
	night := s.night.snapshot()
	if night.baseLevel != 83 || night.azimuth != -1 || !night.cloudy {
		t.Fatalf("server night directive did not apply: %+v", night)
	}
	handleSessionInfoText(s, []byte("/m_interrupt\r"))
	if !execution.complete {
		t.Fatal("server info directive did not cancel the macro")
	}
}

func TestInventoryAddPacketsKeepOfficialAndCustomNames(t *testing.T) {
	oldImages := clImages
	clImages = testCLImages(map[uint32]*climg.ClientItem{
		100: {Name: "Bag", Flags: kItemFlagData},
		200: {Name: "Sword", Slot: kItemSlotRightHand},
	})
	t.Cleanup(func() { clImages = oldImages })

	for _, test := range []struct {
		name, base, custom, display string
		id                          uint16
		idx                         int
		equip                       bool
	}{
		{"unnamed template", "Bag", "", "Bag <#1>", 100, 0, false},
		{"named template", "Bag", "Supplies", "Bag <#1: Supplies>", 100, 0, false},
		{"equipped template", "Bag", "Supplies", "Bag <#1: Supplies>", 100, 0, true},
		{"MacRoman label", "Bag", "Café", "Bag <#1: Café>", 100, 0, false},
		{"unnamed legacy", "Sword", "", "Sword", 200, -1, false},
		{"named legacy", "Sword", "Spare", "Sword <Spare>", 200, -1, true},
		{"missing metadata", "Item 60000", "Spare", "Item 60000 <Spare>", 60000, -1, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := mustNewSession(2)
			cmd := kInvCmdAdd
			if test.equip {
				cmd = kInvCmdAddEquip
			}
			data := []byte{byte(test.id >> 8), byte(test.id)}
			if test.idx >= 0 {
				cmd |= kInvCmdIndex
				data = append(data, byte(test.idx))
			}
			data = append(data, encodeMacRoman(test.custom)...)
			data = append(data, 0)
			if rest, ok := handleSessionInvCmdOther(s, cmd, data); !ok || len(rest) != 0 {
				t.Fatalf("add packet: ok=%v rest=%v", ok, rest)
			}
			item := s.inventory.snapshot()[0]
			if item.Name != test.display || item.Base != test.base || item.Extra != test.custom ||
				item.IDIndex != test.idx || item.Equipped != test.equip {
				t.Fatalf("added item = %+v", item)
			}
			s.inventory.setFull([]uint16{test.id}, []bool{test.equip})
			refreshed := s.inventory.snapshot()[0]
			if refreshed.Name != item.Name || refreshed.Extra != item.Extra || refreshed.InstanceID != item.InstanceID {
				t.Fatalf("inventory refresh changed the item: before=%+v after=%+v", item, refreshed)
			}
			if test.custom != "" {
				s.inventory.rename(test.id, test.idx, "")
				s.inventory.setFull([]uint16{test.id}, []bool{test.equip})
				cleared := s.inventory.snapshot()[0]
				if cleared.Extra != "" || cleared.Name != inventoryDisplayName(test.base, test.idx, "") {
					t.Fatalf("inventory refresh restored a cleared label: %+v", cleared)
				}
			}
		})
	}
}

func TestInventoryDisplayNumbersFollowInsertionAndDeletion(t *testing.T) {
	s := mustNewSession(2)
	s.inventory.add(100, 0, "Bag", false)
	s.inventory.add(100, 1, "Bag", false)
	s.inventory.rename(100, 1, "Supplies")
	secondID := s.inventory.snapshot()[1].InstanceID
	s.inventory.remove(100, 0)
	item := s.inventory.snapshot()[0]
	if item.IDIndex != 0 || item.Name != "Bag <#1: Supplies>" || item.InstanceID != secondID {
		t.Fatalf("item after deletion = %+v", item)
	}
	if got := formatEquipCommand(item.ID, item.IDIndex); got != "/equip 100 1" {
		t.Fatalf("equip after deletion = %q", got)
	}
	s.inventory.add(100, 0, "Bag", false)
	item = s.inventory.snapshot()[0]
	if item.IDIndex != 1 || item.Name != "Bag <#2: Supplies>" || item.InstanceID != secondID {
		t.Fatalf("item after insertion = %+v", item)
	}
	if got := s.inventory.completionNames()[0]; got != item.Name {
		t.Fatalf("completion name = %q, want %q", got, item.Name)
	}
}

func TestExcessSoundsDoNotDisplaceInventoryUpdates(t *testing.T) {
	oldImages, oldThrottle := clImages, gs.ThrottleSounds
	clImages, gs.ThrottleSounds = nil, false
	t.Cleanup(func() { clImages, gs.ThrottleSounds = oldImages, oldThrottle })

	for _, count := range []int{maxSounds, maxSounds + 1, 255} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			s := mustNewSession(2)
			state := []byte{0, 0, byte(count)} // empty info, no bubbles, sounds
			for id := 1; id <= count; id++ {
				state = append(state, byte(id>>8), byte(id))
			}
			state = append(state, kInvCmdFull, 1, 1, 0, 100, kInvCmdNone)
			data := make([]byte, 9)
			binary.BigEndian.PutUint32(data[1:5], 1)
			data = append(data, 0)                  // no descriptors
			data = append(data, make([]byte, 7)...) // stats and light flags
			data = append(data, 0, 0)               // no pictures or mobiles
			data = append(data, byte(len(state)>>8), byte(len(state)))
			data = append(data, state...)
			packet := append([]byte{0, 2}, data...)
			oldMovie, oldEncrypted := movieMode, drawStateEncrypted
			movieMode, drawStateEncrypted = false, false
			t.Cleanup(func() { movieMode, drawStateEncrypted = oldMovie, oldEncrypted })
			if !handleSessionDrawState(s, packet, false) || s.frames.acknowledged() != 1 {
				t.Fatal("valid draw frame was not accepted")
			}
			items := s.inventory.snapshot()
			if len(items) != 1 || items[0].ID != 100 || !items[0].Equipped {
				t.Fatalf("inventory update lost after %d sounds: %+v", count, items)
			}
			events := s.events.log.snapshot()
			if len(events) != 1 || events[0].Kind != sessionEventSound || len(events[0].SoundIDs) != maxSounds {
				t.Fatalf("sound playback limit changed: %+v", events)
			}
		})
	}
}
