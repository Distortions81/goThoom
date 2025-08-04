package main

import (
	"encoding/binary"
	"fmt"
	"testing"
)

func TestDecodeBubbleStripsTags(t *testing.T) {
	data := []byte{0x00, byte(kBubbleWhisper), 0x8A, 0xC2, 'p', 'n', ' ', 'p', 'i', 'n', 'g', '!', 0}
	verb, text, name, _, _, target := decodeBubble(data)
	if verb != "whispers" || text != "ping!" || name != "" || target != thinkNone {
		t.Fatalf("got verb %v text %v name %v target %d", verb, text, name, target)
	}
	assembled := fmt.Sprintf("Bob %v, %v", verb, text)
	if assembled != "Bob whispers, \"ping!\"" {
		t.Fatalf("assembled %v", assembled)
	}
}

func TestDecodeBubbleLanguageWhisperVerb(t *testing.T) {
	data := []byte{0x00, byte(kBubbleWhisper | kBubbleNotCommon), byte(kBubbleHalfling), 'h', 'i', 0}
	verb, text, _, lang, _, _ := decodeBubble(data)
	if lang != "Halfling" || verb != "squeaks softly" || text != "hi" {
		t.Fatalf("got lang %v verb %v text %v", lang, verb, text)
	}
}

func TestDecodeBubbleLanguageYellVerb(t *testing.T) {
	data := []byte{0x00, byte(kBubbleYell | kBubbleNotCommon), byte(kBubbleDwarf), 'h', 'o', 0}
	verb, text, _, lang, _, _ := decodeBubble(data)
	if lang != "Dwarven" || verb != "hollers" || text != "ho" {
		t.Fatalf("got lang %v verb %v text %v", lang, verb, text)
	}
}

func TestDecodeBubbleUnknownYellKeepsText(t *testing.T) {
	data := []byte{0x00, byte(kBubbleYell | kBubbleNotCommon), byte(kBubbleSylvan | kBubbleUnknownShort), 'O', 'k', 0}
	verb, text, _, lang, code, _ := decodeBubble(data)
	if lang != "Sylvan" || verb != "calls" || text != "Ok" || code != kBubbleUnknownShort {
		t.Fatalf("got lang %v verb %v text %v code %x", lang, verb, text, code)
	}
}

func TestDecodeBubbleUnknownMedium(t *testing.T) {
	data := []byte{0x00, byte(kBubbleNormal | kBubbleNotCommon), byte(kBubbleSylvan | kBubbleUnknownMedium), 'O', 'k', 0}
	verb, text, _, lang, code, _ := decodeBubble(data)
	if lang != "Sylvan" || verb != "says" || text != "" || code != kBubbleUnknownMedium {
		t.Fatalf("got lang %v verb %v text %v code %x", lang, verb, text, code)
	}
}

func TestDecodeBubbleEmptyAfterStripping(t *testing.T) {
	data := []byte{0x00, byte(kBubbleNormal), 0x8A, 0xC2, 'p', 'n', 0}
	if verb, txt, name, _, _, target := decodeBubble(data); verb != "" || txt != "" || name != "" || target != thinkNone {
		t.Fatalf("got %v %v %v %d", verb, txt, name, target)
	}
}

func TestDecodeBubbleThinkTargets(t *testing.T) {
	cases := []struct {
		marker byte
		suffix string
		want   thinkTarget
	}{
		{'t', " to you", thinkToYou},
		{'c', " to your clan", thinkToClan},
		{'g', " to a group", thinkToGroup},
	}
	for _, tc := range cases {
		data := []byte{0x00, byte(kBubbleThought), 0x8A}
		data = append(data, []byte("Alice")...)
		data = append(data, []byte{0xC2, 't', '_', 't', tc.marker}...)
		data = append(data, []byte(tc.suffix+": hi")...)
		data = append(data, 0)
		verb, text, name, _, _, target := decodeBubble(data)
		if verb != "thinks" || text != "hi" || name != "Alice" || target != tc.want {
			t.Fatalf("marker %v got %v %v %v %d", tc.marker, verb, text, name, target)
		}
	}
}

func TestDecodeBubbleThinkTargetsSuffixOnly(t *testing.T) {
	cases := []struct {
		suffix string
		want   thinkTarget
	}{
		{" to you", thinkToYou},
		{" to your clan", thinkToClan},
		{" to a group", thinkToGroup},
	}
	for _, tc := range cases {
		data := []byte{0x00, byte(kBubbleThought)}
		data = append(data, []byte("Alice"+tc.suffix+": hi")...)
		data = append(data, 0)
		verb, text, name, _, _, target := decodeBubble(data)
		if verb != "thinks" || text != "hi" || name != "Alice" || target != tc.want {
			t.Fatalf("suffix %v got %v %v %v %d", tc.suffix, verb, text, name, target)
		}
	}
}

func TestParseBackendInfo(t *testing.T) {
	playersMu.Lock()
	players = make(map[string]*Player)
	playersMu.Unlock()
	data := []byte("\xc2be\xc2in\xc2pnAlice\xc2pnHuman\tFemale\tFighter\t")
	decodeBEPP(data)
	playersMu.RLock()
	p := players["Alice"]
	playersMu.RUnlock()
	if p == nil || p.Class != "Fighter" || p.Race != "Human" {
		t.Fatalf("unexpected player: %#v", p)
	}
}

func TestParseBackendShare(t *testing.T) {
	playersMu.Lock()
	players = make(map[string]*Player)
	playersMu.Unlock()
	data := []byte("\xc2be\xc2sh\xc2pnAlice\xc2pn,\xc2pnBob\xc2pn\t\xc2pnCarol\xc2pn")
	decodeBEPP(data)
	playersMu.RLock()
	cond := !players["Alice"].Sharee || !players["Bob"].Sharee || !players["Carol"].Sharing
	playersMu.RUnlock()
	if cond {
		t.Fatalf("share parsing failed: %#v", players)
	}
}

func TestDecodeBEPPYouKilled(t *testing.T) {
	data := []byte("\xc2yk \xc2pnYou\xc2pn helped slaughter a Nocens Winder.")
	if got := decodeBEPP(data); got != "You helped slaughter a Nocens Winder." {
		t.Fatalf("got %v", got)
	}
}

func TestParseMovieNames(t *testing.T) {
	state.descriptors = nil
	state.mobiles = nil
	if _, err := parseMovie("test.clMov", 1440); err != nil {
		t.Fatalf("parseMovie: %v", err)
	}
	found := false
	for _, d := range state.descriptors {
		if d.Name != "" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no descriptor names parsed")
	}
}

func TestParseMobileTableVersions(t *testing.T) {
	cases := []struct {
		version uint16
		name    string
	}{
		{105, "v105"},
		{97, "v97"},
	}
	for _, tc := range cases {
		state.descriptors = nil
		state.mobiles = nil
		data := buildStubTable(tc.version, tc.name)
		parseMobileTable(data, 0, tc.version, 0)
		d := state.descriptors[0]
		if d.Name != tc.name {
			t.Fatalf("version %d got %v", tc.version, d.Name)
		}
	}
}

func buildStubTable(version uint16, name string) []byte {
	var descSize, nameOffset int
	switch {
	case version > 141:
		descSize, nameOffset = 156, 86
	case version > 113:
		descSize, nameOffset = 150, 82
	case version > 105:
		descSize, nameOffset = 142, 82
	case version > 97:
		descSize, nameOffset = 130, 70
	default:
		descSize, nameOffset = 126, 66
	}
	buf := make([]byte, 4+16+descSize+4)
	binary.BigEndian.PutUint32(buf[0:4], 0) // index
	// mobile (16 bytes) already zero
	copy(buf[4+16+nameOffset:], []byte(name))
	// numColors and bubble counter already zero
	binary.BigEndian.PutUint32(buf[4+16+descSize:], 0xffffffff) // terminator
	return buf
}
