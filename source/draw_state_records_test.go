package main

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func packedStateRecordForTest(payload []byte) []byte {
	record := make([]byte, 2+len(payload))
	binary.BigEndian.PutUint16(record, uint16(len(payload)))
	copy(record[2:], payload)
	return record
}

func TestDrawStateRecordScratchKeepsPacketPendingAndRecordsSeparate(t *testing.T) {
	session := &Session{draw: &sessionDrawState{}}
	first := packedStateRecordForTest([]byte("first"))
	second := packedStateRecordForTest([]byte("second"))
	third := packedStateRecordForTest([]byte("third"))
	fragment := append(append(bytes.Clone(first), second...), third[:4]...)
	original := bytes.Clone(fragment)
	var scratch drawStateRecordScratch
	records := scratch.appendFragment(session, fragment)
	if len(records) != 2 || string(records[0]) != "first" || string(records[1]) != "second" {
		t.Fatalf("completed records = %q", records)
	}
	if !bytes.Equal(session.draw.current.stateDataStream, third[:4]) {
		t.Fatalf("pending bytes = %v", session.draw.current.stateDataStream)
	}
	checkpoint := cloneDrawState(session.draw.current)
	records[0][0] = 'F'
	_ = append(records[0], '!', '!', '!')
	if string(records[1]) != "second" {
		t.Fatal("appending to a record overwrote the following record")
	}
	records[1][0] = 'S'
	if !bytes.Equal(fragment, original) || !bytes.Equal(session.draw.current.stateDataStream, third[:4]) {
		t.Fatal("record mutation changed packet or pending stream")
	}
	clear(fragment)
	records = scratch.appendFragment(session, third[4:])
	if len(records) != 1 || string(records[0]) != "third" || len(session.draw.current.stateDataStream) != 0 {
		t.Fatalf("completed tail = %q, pending %v", records, session.draw.current.stateDataStream)
	}
	if !bytes.Equal(checkpoint.stateDataStream, third[:4]) {
		t.Fatal("completing the pending record changed its checkpoint")
	}
	scratch.reset()
	for _, record := range scratch.records[:cap(scratch.records)] {
		if record != nil {
			t.Fatal("reset retained an old record buffer")
		}
	}
}

func TestDrawStateRecordScratchHandlesEveryFragmentBoundary(t *testing.T) {
	stream := packedStateRecordForTest(nil)
	stream = append(stream, packedStateRecordForTest([]byte("one"))...)
	stream = append(stream, packedStateRecordForTest([]byte("two"))...)
	for split := 0; split <= len(stream); split++ {
		session := &Session{draw: &sessionDrawState{}}
		var scratch drawStateRecordScratch
		var got [][]byte
		for _, fragment := range [][]byte{stream[:split], nil, stream[split:]} {
			for _, record := range scratch.appendFragment(session, fragment) {
				got = append(got, bytes.Clone(record))
			}
		}
		if len(got) != 3 || got[0] != nil || string(got[1]) != "one" || string(got[2]) != "two" || len(session.draw.current.stateDataStream) != 0 {
			t.Fatalf("split %d: records %q, pending %v", split, got, session.draw.current.stateDataStream)
		}
	}
}

func TestDrawStateRecordScratchWaitsForMaximumLengthPayload(t *testing.T) {
	session := &Session{draw: &sessionDrawState{}}
	var scratch drawStateRecordScratch
	record := packedStateRecordForTest(bytes.Repeat([]byte{'x'}, 65535))
	if got := scratch.appendFragment(session, record[:len(record)-1]); len(got) != 0 {
		t.Fatalf("incomplete maximum-size record returned %d payloads", len(got))
	}
	got := scratch.appendFragment(session, record[len(record)-1:])
	if len(got) != 1 || !bytes.Equal(got[0], record[2:]) {
		t.Fatal("maximum-size fragmented record was not reconstructed")
	}
}

func TestAppendDrawStateRecordsRemainOwnedAcrossCalls(t *testing.T) {
	session := &Session{draw: &sessionDrawState{}}
	first := appendSessionDrawStateDataFragment(session, packedStateRecordForTest([]byte("first")))
	second := appendSessionDrawStateDataFragment(session, packedStateRecordForTest([]byte("second")))
	second[0][0] = 'S'
	if string(first[0]) != "first" {
		t.Fatal("standalone wrapper reused a previous call's record storage")
	}
}

func TestParsedStateOutputSurvivesRecordScratchReuse(t *testing.T) {
	session := mustNewSession(2)
	oldSettings, oldImages, oldPlayers := gs, clImages, players
	oldBlockSound, oldBlockTTS, oldCombined := blockSound, blockTTS, combinedSessionEvents
	t.Cleanup(func() {
		gs, clImages, players = oldSettings, oldImages, oldPlayers
		blockSound, blockTTS, combinedSessionEvents = oldBlockSound, oldBlockTTS, oldCombined
	})
	gs.SpeechBubbles, gs.BubbleNormal, gs.BubbleOtherPlayers = true, true, true
	gs.ThrottleSounds = false
	clImages, players = nil, map[string]*Player{}
	blockSound, blockTTS = true, true
	combinedSessionEvents = newSessionEventLog(maxSessionEvents)

	packet := sessionStateDataForTest() // A bubble, sound, and named inventory item.
	original := bytes.Clone(packet)
	for range 3 {
		if _, _, err := parseSessionDrawState(session, packet, false); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(packet, original) {
			t.Fatal("state decoding mutated its reusable packet")
		}
	}
	storedBubble := session.draw.current.bubbles[0]
	storedItems := session.inventory.snapshot()
	storedEvents := session.events.log.snapshot()
	if len(storedItems) == 0 || len(storedEvents) < 2 {
		t.Fatal("fixture did not produce inventory and event state")
	}
	clear(packet)
	replacement := buildDrawData("Other", kBubbleNormal, "A different message that overwrites the previous record bytes")
	for range 3 {
		if _, _, err := parseSessionDrawState(session, replacement, false); err != nil {
			t.Fatal(err)
		}
	}
	if storedBubble.Text != "hello" || storedBubble.OwnerName != "Bob" {
		t.Fatalf("scratch reuse changed stored bubble: %+v", storedBubble)
	}
	if storedItems[0].Extra != "Staff" || storedItems[0].Name != "Item 100 <Staff>" {
		t.Fatalf("scratch reuse changed stored inventory: %+v", storedItems)
	}
	if storedEvents[0].Text != "Bob says, hello" || len(storedEvents[1].SoundIDs) != 1 || storedEvents[1].SoundIDs[0] != 58 {
		t.Fatalf("scratch reuse changed stored chat/sound events: %+v", storedEvents[:2])
	}
}

func BenchmarkDrawStateRecords(b *testing.B) {
	record := packedStateRecordForTest([]byte{0, 0, 0, 0})
	b.Run("Owned", func(b *testing.B) {
		session := &Session{draw: &sessionDrawState{}}
		b.ReportAllocs()
		for b.Loop() {
			appendSessionDrawStateDataFragment(session, record)
		}
	})
	b.Run("Reusable", func(b *testing.B) {
		session := &Session{draw: &sessionDrawState{}}
		var scratch drawStateRecordScratch
		scratch.appendFragment(session, record)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			scratch.appendFragment(session, record)
		}
	})
	b.Run("FragmentedReusable", func(b *testing.B) {
		session := &Session{draw: &sessionDrawState{}}
		var scratch drawStateRecordScratch
		scratch.appendFragment(session, record)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			scratch.appendFragment(session, record[:1])
			scratch.appendFragment(session, record[1:3])
			scratch.appendFragment(session, record[3:])
		}
	})
}
