package main

import (
	"encoding/binary"
	"image"
	"testing"

	"gothoom/climg"
)

func TestBubbleLifeModes(t *testing.T) {
	if got := bubbleLifeFrames("one two three", BubbleLifetimeClassic, 2, 1); got != 8*(1000/framems) {
		t.Fatalf("classic lifetime = %d frames", got)
	}
	if got := bubbleLifeFrames("one two three", BubbleLifetimeModern, 2, 1); got != 5*(1000/framems) {
		t.Fatalf("modern lifetime = %d frames", got)
	}
	if got := normalizeBubbleLifetimeMode("classic"); got != BubbleLifetimeClassic {
		t.Fatalf("normalized mode = %q", got)
	}
	if got := normalizeBubbleLifetimeMode("unknown"); got != BubbleLifetimeModern {
		t.Fatalf("unknown mode = %q", got)
	}
}

func TestNormalBubbleVerbUsesTerminalPunctuation(t *testing.T) {
	tests := map[string]string{
		"Hello there":  "says",
		"Are you ok?":  "asks",
		"Look out!   ": "exclaims",
	}
	for text, want := range tests {
		data := append([]byte{0, kBubbleNormal}, encodeMacRoman(text)...)
		data = append(data, 0)
		verb, _, _, _, _, _, _ := decodeBubble(data)
		if verb != want {
			t.Errorf("decodeBubble(%q) verb = %q, want %q", text, verb, want)
		}
	}
}

func TestBEPPClassicDisplayRouting(t *testing.T) {
	for _, prefix := range []string{"dd", "dl", "cf"} {
		raw := append([]byte{0xc2, prefix[0], prefix[1]}, []byte("hidden")...)
		if got := decodeBEPP(raw); got != "" {
			t.Errorf("BEPP %s displayed %q", prefix, got)
		}
	}
	raw := append([]byte{0xc2, 't', 'l'}, []byte("log me")...)
	if got := decodeBEPP(raw); got != "log me" {
		t.Fatalf("BEPP tl = %q", got)
	}
}

func TestBEPPInfoQueuesNamedPlayer(t *testing.T) {
	infoQueueMu.Lock()
	oldQueue := infoQueue
	infoQueue = map[string]struct{}{}
	infoQueueMu.Unlock()
	oldPlayers := players
	players = map[string]*Player{}
	t.Cleanup(func() {
		infoQueueMu.Lock()
		infoQueue = oldQueue
		infoQueueMu.Unlock()
		players = oldPlayers
	})
	raw := append([]byte{0xc2, 'i', 'n'}, pnTag("Bob")...)
	decodeBEPP(raw)
	infoQueueMu.Lock()
	_, ok := infoQueue["Bob"]
	infoQueueMu.Unlock()
	if !ok {
		t.Fatal("BEPP in did not queue Bob for be-info")
	}
}

func TestMobileNameStyleUsesWireBitsAndShareeUnderline(t *testing.T) {
	for colors, want := range map[uint8]uint8{
		0: styleRegular, 1: styleBold, 2: styleItalic, 3: styleBoldItalic,
	} {
		if got := mobileNameStyle(colors, false); got != want {
			t.Errorf("colors %#x style = %#x, want %#x", colors, got, want)
		}
		if got := mobileNameStyle(colors, true); got != want|styleUnderline {
			t.Errorf("sharee colors %#x style = %#x", colors, got)
		}
	}
}

func TestExplicitShadowPictureRules(t *testing.T) {
	oldSettings := gs
	gNight.mu.Lock()
	oldLevel, oldShadows, oldFlags := gNight.Level, gNight.Shadows, gNight.Flags
	gNight.Level, gNight.Shadows, gNight.Flags = 20, 0, 0
	gNight.mu.Unlock()
	t.Cleanup(func() {
		gs = oldSettings
		gNight.mu.Lock()
		gNight.Level, gNight.Shadows, gNight.Flags = oldLevel, oldShadows, oldFlags
		gNight.mu.Unlock()
	})
	if draw, _ := explicitShadowPictureAlpha(climg.PictDefIsShadow); draw {
		t.Fatal("explicit shadow drew with shadow level zero")
	}
	gNight.mu.Lock()
	gNight.Shadows, gNight.Level = 25, 34
	gNight.mu.Unlock()
	gs.MaxNightLevel = 100
	if draw, alpha := explicitShadowPictureAlpha(climg.PictDefIsShadow); !draw || alpha != 0.25 {
		t.Fatalf("dark explicit shadow = draw %v alpha %v", draw, alpha)
	}
	gs.MaxNightLevel = 33
	if draw, alpha := explicitShadowPictureAlpha(climg.PictDefIsShadow); !draw || alpha != 1 {
		t.Fatalf("limited-night explicit shadow = draw %v alpha %v", draw, alpha)
	}
	if draw, alpha := explicitShadowPictureAlpha(0); !draw || alpha != 1 {
		t.Fatalf("ordinary picture = draw %v alpha %v", draw, alpha)
	}
}

func TestChooseBubblePlacementAvoidsOccupiedQuadrant(t *testing.T) {
	metrics := bubbleMetrics{width: 40, height: 20, tailHeight: 10}
	anchor := image.Pt(100, 100)
	upperLeft := bubbleRectForPlacement(anchor.X, anchor.Y, metrics, bubblePosUpperLeft, false)
	pos, _ := chooseBubblePlacement(anchor, metrics, image.Rect(0, 0, 200, 200), nil, []image.Rectangle{upperLeft}, 2, bubblePosNone)
	if pos != bubblePosUpperRight {
		t.Fatalf("placement = %d, want upper-right %d", pos, bubblePosUpperRight)
	}
}

func TestMovieMobileTableRestoresBubbleAndPicturelessFallback(t *testing.T) {
	resetDrawState()
	t.Cleanup(resetDrawState)
	layout, ok := layoutForMobileTable(142)
	if !ok {
		t.Fatal("current movie layout unavailable")
	}
	data := make([]byte, 4+16+layout.descSize+2+5+4)
	pos := 0
	binary.BigEndian.PutUint32(data[pos:], 1)
	pos += 4
	binary.BigEndian.PutUint32(data[pos:], 0)
	binary.BigEndian.PutUint32(data[pos+4:], 10)
	binary.BigEndian.PutUint32(data[pos+8:], 20)
	pos += 16
	buf := data[pos : pos+layout.descSize]
	binary.BigEndian.PutUint32(buf[0:], 0xffffffff)
	binary.BigEndian.PutUint32(buf[layout.typeOffset:], kDescPlayer)
	binary.BigEndian.PutUint32(buf[layout.bubbleTypeOffset:], kBubbleNormal)
	binary.BigEndian.PutUint32(buf[layout.bubbleCounterOffset:], 40)
	copy(buf[layout.nameOffset:], "Bob")
	pos += layout.descSize
	binary.BigEndian.PutUint16(data[pos:], 5)
	copy(data[pos+2:], "Hello")
	pos += 7
	binary.BigEndian.PutUint32(data[pos:], 0xffffffff)
	parseMobileTable(data, 0, 142, 0)
	stateMu.Lock()
	desc := state.descriptors[1]
	bubbles := append([]bubble(nil), state.bubbles...)
	stateMu.Unlock()
	if desc.PictID != picturelessDescriptorFallbackPictID {
		t.Fatalf("movie fallback picture = %d", desc.PictID)
	}
	if len(bubbles) != 1 || bubbles[0].Text != "Hello" || bubbles[0].LifeFrames != 40 {
		t.Fatalf("restored bubbles = %#v", bubbles)
	}
}

func TestLivePicturelessDescriptorUsesClassicFallback(t *testing.T) {
	resetTestState()
	data := buildDrawData("Bob", kBubbleNormal, "hello")
	// descriptor count starts at byte 9; the first descriptor picture is 12:14.
	binary.BigEndian.PutUint16(data[12:14], 0xffff)
	if _, _, err := parseDrawState(data, false); err != nil {
		t.Fatal(err)
	}
	stateMu.Lock()
	pict := state.descriptors[1].PictID
	stateMu.Unlock()
	if pict != picturelessDescriptorFallbackPictID {
		t.Fatalf("live fallback picture = %d", pict)
	}
}

func liveDrawPacketForTest(ack, resent uint32, text string) []byte {
	body := buildDrawData("Bob", kBubbleNormal, text)
	binary.BigEndian.PutUint32(body[1:5], ack)
	binary.BigEndian.PutUint32(body[5:9], resent)
	packet := make([]byte, len(body)+2)
	copy(packet[2:], body)
	return packet
}

func classicStateRecordForTest(text string) []byte {
	payload := []byte{0, 1, 1, byte(kBubbleNormal)}
	payload = append(payload, encodeMacRoman(text)...)
	payload = append(payload, 0, 0, 0)
	record := []byte{byte(len(payload) >> 8), byte(len(payload))}
	return append(record, payload...)
}

func liveDrawPacketWithStateFragmentForTest(ack, resent uint32, text string, fragment []byte) []byte {
	body := buildDrawData("Bob", kBubbleNormal, text)
	fullRecord := classicStateRecordForTest(text)
	body = append(body[:len(body)-len(fullRecord)], fragment...)
	binary.BigEndian.PutUint32(body[1:5], ack)
	binary.BigEndian.PutUint32(body[5:9], resent)
	packet := make([]byte, len(body)+2)
	binary.BigEndian.PutUint16(packet[:2], 2)
	copy(packet[2:], body)
	return packet
}

func prepareLiveStateFragmentTest(t *testing.T) {
	t.Helper()
	oldAck, oldResend, oldLastAck := ackFrame, resendFrame, lastAckFrame
	oldFrameCounter, oldMovieMode, oldEncrypted := frameCounter, movieMode, drawStateEncrypted
	oldSettings, oldPlayers := gs, players
	t.Cleanup(func() {
		ackFrame, resendFrame, lastAckFrame = oldAck, oldResend, oldLastAck
		frameCounter, movieMode, drawStateEncrypted = oldFrameCounter, oldMovieMode, oldEncrypted
		gs, players = oldSettings, oldPlayers
		resetDrawState()
	})
	resetDrawState()
	players = map[string]*Player{}
	gs.SpeechBubbles = true
	gs.BubbleNormal = true
	gs.BubbleOtherPlayers = true
	movieMode, drawStateEncrypted = false, false
	ackFrame, resendFrame, lastAckFrame, frameCounter = 0, 0, 0, 0
}

func TestLiveStateRecordCanSpanSizeAndPayloadAcrossFrames(t *testing.T) {
	prepareLiveStateFragmentTest(t)
	record := classicStateRecordForTest("fragmented")

	handleDrawState(liveDrawPacketWithStateFragmentForTest(1, 0, "fragmented", record[:1]), false)
	if ackFrame != 1 || resendFrame != 0 {
		t.Fatalf("size-high fragment ack=%d resend=%d, want 1/0", ackFrame, resendFrame)
	}
	handleDrawState(liveDrawPacketWithStateFragmentForTest(2, 0, "fragmented", record[1:6]), false)
	stateMu.Lock()
	bubblesBeforeCompletion := len(state.bubbles)
	pendingBeforeCompletion := len(state.stateDataStream)
	stateMu.Unlock()
	if bubblesBeforeCompletion != 0 || pendingBeforeCompletion != 6 {
		t.Fatalf("partial state = bubbles %d pending %d, want 0/6", bubblesBeforeCompletion, pendingBeforeCompletion)
	}

	handleDrawState(liveDrawPacketWithStateFragmentForTest(3, 0, "fragmented", record[6:]), false)
	stateMu.Lock()
	bubbles := append([]bubble(nil), state.bubbles...)
	pending := len(state.stateDataStream)
	stateMu.Unlock()
	if ackFrame != 3 || resendFrame != 0 || pending != 0 {
		t.Fatalf("completed state ack=%d resend=%d pending=%d, want 3/0/0", ackFrame, resendFrame, pending)
	}
	if len(bubbles) != 1 || bubbles[0].Text != "fragmented" {
		t.Fatalf("completed fragmented bubbles = %#v", bubbles)
	}
}

func TestLiveStateFragmentProcessesMultipleCompleteRecords(t *testing.T) {
	prepareLiveStateFragmentTest(t)
	first := classicStateRecordForTest("first")
	second := classicStateRecordForTest("second")
	fragment := append(append([]byte(nil), first...), second...)

	handleDrawState(liveDrawPacketWithStateFragmentForTest(1, 0, "first", fragment), false)
	stateMu.Lock()
	bubbles := append([]bubble(nil), state.bubbles...)
	pending := len(state.stateDataStream)
	stateMu.Unlock()
	if pending != 0 || len(bubbles) != 2 || bubbles[0].Text != "first" || bubbles[1].Text != "second" {
		t.Fatalf("multiple state records = pending %d bubbles %#v", pending, bubbles)
	}
}

func TestPendingStateFragmentSurvivesDrawStateClone(t *testing.T) {
	original := drawState{stateDataStream: []byte{0, 12, 1, 2, 3}}
	cloned := cloneDrawState(original)
	if got := cloned.stateDataStream; len(got) != 5 || got[4] != 3 {
		t.Fatalf("cloned pending state = %v", got)
	}
	cloned.stateDataStream[2] = 99
	if original.stateDataStream[2] != 1 {
		t.Fatal("cloned pending state aliases the original")
	}
}

func TestLiveStateFragmentRecoveryMatchesClassicResendFlow(t *testing.T) {
	prepareLiveStateFragmentTest(t)
	record := classicStateRecordForTest("recovered split")
	split := 7

	handleDrawState(liveDrawPacketWithStateFragmentForTest(1, 0, "recovered split", record[:split]), false)
	ignored := classicStateRecordForTest("must be ignored")
	handleDrawState(liveDrawPacketWithStateFragmentForTest(3, 0, "must be ignored", ignored), false)
	if ackFrame != 3 || resendFrame != 2 {
		t.Fatalf("gap state ack=%d resend=%d, want 3/2", ackFrame, resendFrame)
	}

	handleDrawState(liveDrawPacketWithStateFragmentForTest(4, 2, "recovered split", record[split:]), false)
	stateMu.Lock()
	bubbles := append([]bubble(nil), state.bubbles...)
	pending := len(state.stateDataStream)
	stateMu.Unlock()
	if ackFrame != 4 || resendFrame != 0 || pending != 0 {
		t.Fatalf("recovered split ack=%d resend=%d pending=%d, want 4/0/0", ackFrame, resendFrame, pending)
	}
	if len(bubbles) != 1 || bubbles[0].Text != "recovered split" {
		t.Fatalf("recovered split bubbles = %#v", bubbles)
	}
}

func TestLiveFrameOrderingAndStateRecovery(t *testing.T) {
	oldAck, oldResend, oldLastAck := ackFrame, resendFrame, lastAckFrame
	oldFrameCounter, oldMovieMode, oldEncrypted := frameCounter, movieMode, drawStateEncrypted
	oldSettings, oldPlayers := gs, players
	t.Cleanup(func() {
		ackFrame, resendFrame, lastAckFrame = oldAck, oldResend, oldLastAck
		frameCounter, movieMode, drawStateEncrypted = oldFrameCounter, oldMovieMode, oldEncrypted
		gs, players = oldSettings, oldPlayers
		resetDrawState()
	})
	resetDrawState()
	players = map[string]*Player{}
	gs.SpeechBubbles = true
	gs.BubbleNormal = true
	gs.BubbleOtherPlayers = true
	movieMode, drawStateEncrypted = false, false
	ackFrame, resendFrame, lastAckFrame, frameCounter = 0, 0, 0, 0

	handleDrawState(liveDrawPacketForTest(1, 0, "first"), false)
	if ackFrame != 1 || resendFrame != 0 {
		t.Fatalf("first frame ack=%d resend=%d", ackFrame, resendFrame)
	}

	handleDrawState(liveDrawPacketForTest(3, 0, "skipped state"), false)
	if ackFrame != 3 || resendFrame != 2 {
		t.Fatalf("gap frame ack=%d resend=%d, want 3/2", ackFrame, resendFrame)
	}
	stateMu.Lock()
	if len(state.bubbles) != 1 || state.bubbles[0].Text != "first" {
		got := append([]bubble(nil), state.bubbles...)
		stateMu.Unlock()
		t.Fatalf("gap applied state data: %#v", got)
	}
	stateMu.Unlock()

	handleDrawState(liveDrawPacketForTest(4, 2, "recovered"), false)
	if ackFrame != 4 || resendFrame != 0 {
		t.Fatalf("recovery frame ack=%d resend=%d, want 4/0", ackFrame, resendFrame)
	}
	stateMu.Lock()
	lastBubble := state.bubbles[len(state.bubbles)-1]
	stateMu.Unlock()
	if lastBubble.Text != "recovered" {
		t.Fatalf("recovered state bubble = %q", lastBubble.Text)
	}

	beforeFrame := frameCounter
	handleDrawState(liveDrawPacketForTest(4, 0, "duplicate"), false)
	if ackFrame != 4 || frameCounter != beforeFrame {
		t.Fatalf("duplicate changed ack/frame counter to %d/%d", ackFrame, frameCounter)
	}
}
