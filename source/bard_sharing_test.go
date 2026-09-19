package main

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"gothoom/eui"
)

func bardShareFixture(t *testing.T) (*bardPanel, *Session, []string) {
	t.Helper()
	p, s := bardReadyPanel(t)
	p.partners.Text = "Blue, Pixy"
	p.showEnsembleWindow()
	if p.sharing.receive.Checked {
		t.Fatal("receiving enabled by default")
	}
	p.setReceiveParts(true)
	notes := "@90 c8g8 "
	for n := 0; n < 180; n++ {
		notes += fmt.Sprintf("%c%d", "cdefgab"[(n*n+3*n)%7], n%9+1)
	}
	messages, err := bardPartMessages("Moonlight — 月", bardPart{Name: "Accompaniment", Instrument: 0, Text: notes})
	if err != nil {
		t.Fatal(err)
	}
	return p, s, messages
}

func deliverBardPart(p *bardPanel, s *Session, sender string, messages []string) {
	for _, message := range messages {
		p.receivePartMessage(s, bardConnectionGeneration(s), sender, message, time.Now())
	}
}

func TestBardSharingRoundTripSaveSelectWithoutPlaying(t *testing.T) {
	p, s, messages := bardShareFixture(t)
	original := p.selected
	if len(messages) < 2 {
		t.Fatal("fixture should span multiple messages")
	}
	// Reordered and duplicated chunks must assemble exactly once.
	for n := len(messages) - 1; n >= 0; n-- {
		deliverBardPart(p, s, "Blue", []string{messages[n], messages[n]})
	}
	if p.selected == original || p.selectedPart != "Solo" || p.performance != nil || !s.commands.idle() {
		t.Fatal("did not select saved part without playing")
	}
	value, err := readBardTune(p.selected)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(value, "<@title: Moonlight — 月>\n<Received from Blue>\n") || strings.Contains(value, ";@") {
		t.Fatalf("received file did not use Clan Lord comments: %q", value)
	}
	score, err := parseBardScore(value, 0)
	if err != nil || score.Title != "Moonlight — 月" || len(score.Parts) != 1 || score.Parts[0].Instrument != 0 || !strings.Contains(score.Parts[0].Text, "@90c8g8") {
		t.Fatalf("bad received score: %+v %v", score, err)
	}
	selected := p.selected
	deliverBardPart(p, s, "Blue", messages)
	if p.selected != selected || len(p.tunes) != 2 {
		t.Fatal("duplicate transfer created another tune")
	}
	s.inventory.add(222, -1, "Lucky Lyra", false)
	p.play()
	if p.playConfirm == nil {
		t.Fatal("cannot play received part", p.statusMessage)
	}
	clickMacroEditorButton(t, p.playConfirm, "Play in Game")
	if p.performance == nil || !p.performance.ensemble || !strings.Contains(strings.Join(p.performance.commands, " "), "/with Blue /with Pixy") {
		t.Fatal("received solo part lost ensemble partners")
	}
}

func TestBardSharingReceiveGates(t *testing.T) {
	for _, scenario := range []string{"disabled", "unlisted", "prefix", "blocked", "different session", "reconnected", "closed", "before enabled", "incomplete"} {
		t.Run(scenario, func(t *testing.T) {
			p, s, messages := bardShareFixture(t)
			original := p.selected
			sender := "Blue"
			generation := bardConnectionGeneration(s)
			at := time.Now()
			switch scenario {
			case "disabled":
				p.setReceiveParts(false)
			case "unlisted":
				sender = "Stranger"
			case "prefix":
				p.partners.Text = "Bl, Pixy"
			case "blocked":
				s.players.players["Blue"] = &Player{Name: "Blue", GlobalLabel: 6}
			case "different session":
				s = bardConnectedSession(t)
			case "reconnected":
				s.transport.mu.Lock()
				s.transport.generation++
				s.transport.mu.Unlock()
			case "closed":
				p.win.Close()
			case "before enabled":
				at = p.sharing.enabledAt.Add(-time.Second)
			case "incomplete":
				messages = messages[:1]
			}
			for _, message := range messages {
				p.receivePartMessage(s, generation, sender, message, at)
			}
			if p.selected != original || len(p.tunes) != 1 || !s.commands.idle() {
				t.Fatal("receive gate allowed a transfer")
			}
		})
	}
}

func TestBardSharingPrivateThoughtOnly(t *testing.T) {
	for _, kind := range []string{"private", "public", "clan", "emote"} {
		t.Run(kind, func(t *testing.T) {
			p, s, messages := bardShareFixture(t)
			original := p.selected
			for _, message := range messages {
				typ := kBubbleThought
				body := "Blue: " + message
				switch kind {
				case "private":
					body = "Blue to you: " + message
				case "clan":
					body = "Blue to your clan: " + message
				case "emote":
					typ = kBubbleRealAction
					body = "Blue thinks to you, " + message
				}
				if _, _, err := parseSessionDrawState(s, buildDrawData("Blue", typ, body), false); err != nil {
					t.Fatal(err)
				}
			}
			drainMainThreadDispatcher()
			if got := p.selected != original; got != (kind == "private") {
				t.Fatalf("%s imported=%t", kind, got)
			}
		})
	}
}

func TestBardSharingPreservesPrivateThoughtTag(t *testing.T) {
	raw := []byte{1, kBubbleThought, 0xc2, 't', '_', 't', 't'}
	raw = append(raw, []byte("Blue: [GTPart1:test]")...)
	raw = append(raw, 0)
	_, message, name, _, _, _, target := decodeBubble(raw)
	if target != thinkToYou || name != "Blue" || message != "[GTPart1:test]" {
		t.Fatalf("private tag lost: %v %q %q", target, name, message)
	}
}

func TestBardSharingLimitsAndValidation(t *testing.T) {
	for _, message := range []string{
		"<1 of 1> cde",
		"<0 of 1 | A | Pine Flute | 4Au> cde",
		"<1 of 65 | A | Pine Flute | 4Au> cde",
		"<1 of 1 | A | Pine Flute | 0O1> cde",
	} {
		if _, _, _, _, _, err := parseBardPartMessage(message); err == nil {
			t.Fatal("accepted malformed chunk", message)
		}
	}
	for _, part := range []bardSharedPart{
		{Title: "Song", Instrument: -1, Notes: "cde"},
		{Title: "Song\n;@part: Injected", Instrument: 17, Notes: "cde"},
		{Title: "Song", Instrument: 17, Notes: "/quit"},
		{Title: "Song", Instrument: 17, Notes: ";@instrument: Lucky Lyra\ncde"},
	} {
		if validateBardSharedPart(part) == nil {
			t.Fatal("accepted unsafe/invalid payload", part)
		}
	}
	incoming := &bardIncomingPart{part: bardSharedPart{Title: "Song", Instrument: 17}, chunks: []string{strings.Repeat("c", bardShareMaxPayload+1)}}
	if _, err := decodeBardSharedPart(incoming); err == nil {
		t.Fatal("accepted oversized music")
	}
}

func TestBardSharingCRC16Base58(t *testing.T) {
	// CRC-16/CCITT-FALSE check value for "123456789" is 0x29b1.
	// Also cover the complete 16-bit range and zero padding.
	for _, test := range []struct{ value, want string }{
		{"123456789", "4B2"},
		{"", "LUv"},
		{"\xff\xff", "111"},
	} {
		if got := bardShareChecksum(test.value); got != test.want {
			t.Fatalf("checksum(%q) = %q, want %q", test.value, got, test.want)
		}
	}
}

func TestBardSharingCompactHeaderAndChecksum(t *testing.T) {
	messages, err := bardPartMessages("Moonlight", bardPart{Name: "Solo", Instrument: 17, Text: "cde"})
	if err != nil || len(messages) != 1 {
		t.Fatalf("messages: %q %v", messages, err)
	}
	part, id, _, _, chunk, err := parseBardPartMessage(messages[0])
	if err != nil || id != "HYb" || messages[0] != "<1 of 1 | Moonlight | Pine Flute | HYb> cde" {
		t.Fatalf("header is not compact: %q %v", messages[0], err)
	}
	renamed, err := bardPartMessages("Moonlight", bardPart{Name: "Melody", Instrument: 17, Text: "cde"})
	if err != nil || !reflect.DeepEqual(renamed, messages) {
		t.Fatalf("local part name changed transmitted music: %q %v", renamed, err)
	}
	incoming := &bardIncomingPart{part: part, id: id, chunks: []string{chunk}}
	want := part
	want.Notes = chunk
	if got, err := decodeBardSharedPart(incoming); err != nil || got != want {
		t.Fatalf("checksum failed: %+v %v", got, err)
	}
	incoming.chunks[0] = "cdf"
	if _, err := decodeBardSharedPart(incoming); err == nil {
		t.Fatal("accepted damaged music")
	}
	incoming.chunks[0] = chunk
	incoming.id = "111"
	if incoming.id == id {
		incoming.id = "112"
	}
	if _, err := decodeBardSharedPart(incoming); err == nil {
		t.Fatal("accepted damaged checksum")
	}
	for _, invalid := range []string{"4A", "44Au", "0O1", "1Il", "342f9ada"} {
		if _, _, _, _, _, err := parseBardPartMessage(strings.Replace(messages[0], " | "+id+">", " | "+invalid+">", 1)); err == nil {
			t.Fatalf("accepted invalid checksum: %q", invalid)
		}
	}
}

func TestBardSharingAssignSendAndCancel(t *testing.T) {
	p, s := bardReadyPanel(t)
	p.partners.Text = "Blue, Pixy"
	p.showEnsembleWindow()
	if p.sharing.receive.Checked || len(p.sharing.assignments) != 2 || p.sharing.assignments[0].Selected != 2 || p.sharing.assignments[1].Selected != 0 {
		t.Fatal("bad ensemble defaults")
	}
	p.sharing.assignments[1].Selected = 1
	clickMacroEditorButton(t, p.sharing.win, "Send Parts")
	commands := bardQueueTexts(s)
	if len(commands) == 0 {
		t.Fatal("no messages queued", p.statusMessage)
	}
	recipients := map[string]bool{}
	for _, command := range commands {
		fields := strings.SplitN(command, " ", 3)
		if len(fields) != 3 || fields[0] != "/thinkto" || len(encodeMacRoman(encodeEmojiShortcodes(command))) > maxPlayerCommandBytes {
			t.Fatal("invalid private command", command)
		}
		recipients[fields[1]] = true
		if _, _, _, _, _, err := parseBardPartMessage(fields[2]); err != nil {
			t.Fatal(err)
		}
	}
	if !recipients["blue"] || !recipients["pixy"] || len(recipients) != 2 {
		t.Fatal("wrong recipients", recipients)
	}
	s.commands.enqueue("/pose sit")
	clickMacroEditorButton(t, p.sharing.win, "Cancel Sending")
	if got := bardQueueTexts(s); len(got) != 1 || got[0] != "/pose sit" {
		t.Fatal("cancel affected unrelated commands", got)
	}
}

func TestBardSharingRejectsStaleSendAndPartialTransfer(t *testing.T) {
	p, s, messages := bardShareFixture(t)
	deliverBardPart(p, s, "Blue", messages[:1])
	p.setReceiveParts(false)
	p.setReceiveParts(true)
	deliverBardPart(p, s, "Blue", messages[1:])
	if len(p.tunes) != 1 {
		t.Fatal("combined chunks across receiving toggle")
	}
	s.transport.mu.Lock()
	s.transport.generation++
	s.transport.mu.Unlock()
	clickMacroEditorButton(t, p.sharing.win, "Send Parts")
	if !s.commands.idle() {
		t.Fatal("stale dialog sent to new connection")
	}
	p.updateBardSharing()
	if p.sharing.receive.Checked {
		t.Fatal("receiving survived reconnect")
	}
}

func TestBardSharingReceivedFileDoesNotOverwrite(t *testing.T) {
	p, s, _ := bardShareFixture(t)
	original := p.selected
	data, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	messages, err := bardPartMessages("../../duet", bardPart{Instrument: 17, Text: "cde"})
	if err != nil {
		t.Fatal(err)
	}
	deliverBardPart(p, s, "Blue", messages)
	current, err := os.ReadFile(original)
	if err != nil || !bytes.Equal(data, current) || p.selected == original {
		t.Fatal("received metadata overwrote an existing file")
	}

}

func TestBardSharingReceiveCheckbox(t *testing.T) {
	p, _ := bardReadyPanel(t)
	p.showEnsembleWindow()
	p.sharing.receive.Handler.Emit(eui.UIEvent{Type: eui.EventCheckboxChanged, Checked: true})
	if !p.sharing.receive.Checked {
		t.Fatal("checkbox did not enable receiving")
	}
}

func TestBardSharingCorruptionCanBeResent(t *testing.T) {
	p, s, messages := bardShareFixture(t)
	damaged := append([]string(nil), messages...)
	metadata, id, index, count, chunk, err := parseBardPartMessage(damaged[0])
	if err != nil {
		t.Fatal(err)
	}
	replacement := "A"
	if chunk[:1] == replacement {
		replacement = "B"
	}
	damaged[0] = bardPartMessage(metadata, id, index, count, replacement+chunk[1:])
	deliverBardPart(p, s, "Blue", damaged)
	if len(p.tunes) != 1 {
		t.Fatal("saved corrupted music")
	}
	deliverBardPart(p, s, "Blue", messages)
	if len(p.tunes) != 2 {
		t.Fatal("valid resend was rejected")
	}
}

func TestBardSharingReadableMessagesArePlayableWhenJoined(t *testing.T) {
	part := bardPart{Name: "Melody", Instrument: 17, Text: "@90 " + strings.Repeat("(c#2d.e4gab/c)2 ", 90)}
	messages, err := bardPartMessages("Moonlight", part)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) < 3 {
		t.Fatal("expected several segments")
	}
	for n, message := range messages {
		want := fmt.Sprintf("<%d of %d | Moonlight | Pine Flute | ", n+1, len(messages))
		if !strings.HasPrefix(message, want) {
			t.Fatalf("missing readable comment: %s", message)
		}
		if len(encodeMacRoman(encodeEmojiShortcodes(message))) > bardShareMaxMessage {
			t.Fatal("message exceeds wire budget")
		}
	}
	// This models a player on another client copying entire message bodies,
	// leaving the comments in place and putting a newline between messages.
	original, err := validateBardTune(part.Text, part.Instrument)
	if err != nil {
		t.Fatal(err)
	}
	combined, err := validateBardTune(strings.Join(messages, "\n"), part.Instrument)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, combined) {
		t.Fatal("manual assembly changed the music")
	}
	if _, err := bardTuneCommands(strings.Join(messages, "\n")); err != nil {
		t.Fatal("manual assembly cannot be performed", err)
	}
}

func TestBardSharingReadableHeadersSurviveWireEncoding(t *testing.T) {
	p, s, _ := bardShareFixture(t)
	title := `Café | <Moonlight> 月 🎵 100% \u1234`
	part := bardPart{Name: `Melody | 1 <test>`, Instrument: 17, Text: "@90cdef"}
	messages, err := bardPartMessages(title, part)
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range messages {
		wire := encodeMacRoman(encodeEmojiShortcodes(message))
		if len(wire) > bardShareMaxMessage {
			t.Fatal("wire expansion exceeds limit")
		}
		decoded := decodeServerText(wire)
		p.receivePartMessage(s, bardConnectionGeneration(s), "Blue", decoded, time.Now())
	}
	value, err := readBardTune(p.selected)
	if err != nil {
		t.Fatal(err)
	}
	score, err := parseBardScore(value, 0)
	if err != nil || score.Title != title || score.Parts[0].Name != "Solo" {
		t.Fatalf("metadata damaged: %+v %v", score, err)
	}
}
