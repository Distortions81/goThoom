package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestRecordRoundTrip(t *testing.T) {
	moviePath := movieFixturePath(t, "test.clMov")
	orig := readMovieFixture(t, "test.clMov")
	if len(orig) < 24 {
		t.Fatalf("short file")
	}
	head := fileHead{
		Signature:    binary.BigEndian.Uint32(orig[0:4]),
		Version:      binary.BigEndian.Uint16(orig[4:6]),
		Len:          binary.BigEndian.Uint16(orig[6:8]),
		Frames:       int32(binary.BigEndian.Uint32(orig[8:12])),
		StartTime:    binary.BigEndian.Uint32(orig[12:16]),
		Revision:     int32(binary.BigEndian.Uint32(orig[16:20])),
		OldestReader: int32(binary.BigEndian.Uint32(orig[20:24])),
	}
	frames, err := parseMovie(moviePath, 0)
	if err != nil {
		t.Fatalf("parseMovie: %v", err)
	}
	tmp := filepath.Join(t.TempDir(), "roundtrip.clMov")
	mr, err := newMovieRecorder(tmp, int(head.Version), int(head.Revision))
	if err != nil {
		t.Fatalf("newMovieRecorder: %v", err)
	}
	mr.head.StartTime = head.StartTime
	mr.head.OldestReader = head.OldestReader
	if err := mr.writeHeader(); err != nil {
		t.Fatalf("writeHeader: %v", err)
	}
	const blockFlags = flagGameState | flagMobileData | flagPictureTable
	for _, fr := range frames {
		if len(fr.data) == 0 {
			if err := mr.WriteBlock(fr.preData, fr.flags); err != nil {
				t.Fatalf("WriteBlock: %v", err)
			}
			continue
		}
		if len(fr.preData) > 0 {
			mr.AddBlock(fr.preData, fr.flags&blockFlags)
		}
		if err := mr.WriteFrame(fr.data, fr.flags&^blockFlags); err != nil {
			t.Fatalf("WriteFrame: %v", err)
		}
	}
	if err := mr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	rec, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("ReadFile(tmp): %v", err)
	}
	if !bytes.Equal(orig, rec) {
		t.Fatalf("recording mismatch: %d vs %d bytes", len(orig), len(rec))
	}
	if _, err := parseMovie(tmp, 0); err != nil {
		t.Fatalf("parseMovie(tmp): %v", err)
	}
}

func TestGameStateBlock(t *testing.T) {
	payload := []byte{1, 2, 3}
	buf := gameStateBlock(1, 2, 3, 4, 5, 6, payload)
	if len(buf) != 24+len(payload) {
		t.Fatalf("len %d", len(buf))
	}
	if binary.BigEndian.Uint32(buf[0:4]) != 1 {
		t.Fatalf("left id")
	}
	if binary.BigEndian.Uint32(buf[4:8]) != 2 {
		t.Fatalf("right id")
	}
	if binary.BigEndian.Uint32(buf[8:12]) != 3 {
		t.Fatalf("mode")
	}
	if binary.BigEndian.Uint32(buf[12:16]) != 4 {
		t.Fatalf("maxSize")
	}
	if binary.BigEndian.Uint32(buf[16:20]) != 5 {
		t.Fatalf("curSize")
	}
	if binary.BigEndian.Uint32(buf[20:24]) != 6 {
		t.Fatalf("expectedSize")
	}
	if !bytes.Equal(buf[24:], payload) {
		t.Fatalf("payload")
	}
}

func TestAddBlockWriteFrame(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "preblock.clMov")
	mr, err := newMovieRecorder(tmp, 400, 1)
	if err != nil {
		t.Fatalf("newMovieRecorder: %v", err)
	}
	pre := []byte{0xaa, 0xbb}
	mr.AddBlock(pre, flagGameState)
	f1 := []byte{0x01, 0x02, 0x03}
	if err := mr.WriteFrame(f1, flagPictureTable); err != nil {
		t.Fatalf("WriteFrame1: %v", err)
	}
	f2 := []byte{0x04}
	if err := mr.WriteFrame(f2, 0); err != nil {
		t.Fatalf("WriteFrame2: %v", err)
	}
	if err := mr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	data, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	pos := 24
	if len(data) < pos+12 {
		t.Fatalf("short file")
	}
	if binary.BigEndian.Uint32(data[pos:pos+4]) != movieSignature {
		t.Fatalf("sig1")
	}
	size1 := int(binary.BigEndian.Uint16(data[pos+8 : pos+10]))
	flags1 := binary.BigEndian.Uint16(data[pos+10 : pos+12])
	if flags1 != flagGameState|flagPictureTable {
		t.Fatalf("flags1 %x", flags1)
	}
	pos += 12
	if !bytes.Equal(data[pos:pos+len(pre)], pre) {
		t.Fatalf("preData")
	}
	pos += len(pre)
	if size1 != len(f1) {
		t.Fatalf("size1 %d", size1)
	}
	if !bytes.Equal(data[pos:pos+len(f1)], f1) {
		t.Fatalf("frame1")
	}
	pos += len(f1)
	if len(data) < pos+12 {
		t.Fatalf("short second frame")
	}
	if binary.BigEndian.Uint32(data[pos:pos+4]) != movieSignature {
		t.Fatalf("sig2")
	}
	size2 := int(binary.BigEndian.Uint16(data[pos+8 : pos+10]))
	flags2 := binary.BigEndian.Uint16(data[pos+10 : pos+12])
	if flags2 != 0 {
		t.Fatalf("flags2 %x", flags2)
	}
	pos += 12
	if size2 != len(f2) {
		t.Fatalf("size2 %d", size2)
	}
	if !bytes.Equal(data[pos:pos+len(f2)], f2) {
		t.Fatalf("frame2")
	}
	pos += len(f2)
	if pos != len(data) {
		t.Fatalf("extra %d", len(data)-pos)
	}
}

func TestStateSnapshotRoundTrip(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "snapshot.clMov")
	mr, err := newMovieRecorder(tmp, 1497, 0)
	if err != nil {
		t.Fatalf("newMovieRecorder: %v", err)
	}
	snapshot := drawState{
		descriptors: map[uint8]frameDescriptor{
			3: {Index: 3, Type: kDescPlayer, PictID: 447, Name: "Distortions", Colors: []byte{1, 2, 3, 4}},
			9: {Index: 9, Type: kDescNPC, PictID: 822, Name: "Trainer"},
		},
		mobiles: map[uint8]frameMobile{
			3: {Index: 3, State: 7, H: -123, V: 456, Colors: 5},
		},
		pictures: []framePicture{
			{PictID: 100, H: -20, V: 30},
			{PictID: 200, H: 40, V: -50},
		},
	}
	mr.AddStateSnapshot(snapshot, 1497, movieNightState{})
	frame := []byte{0, 2, 0, 0}
	if err := mr.WriteFrame(frame, 0); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}
	if err := mr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	frames, err := parseMovie(tmp, 1497)
	if err != nil {
		t.Fatalf("parseMovie: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("frames = %d, want 1", len(frames))
	}
	if got := frames[0].flags; got != flagGameState|flagMobileData|flagPictureTable {
		t.Fatalf("snapshot flags = %#x", got)
	}
	if !bytes.Equal(frames[0].data, frame) {
		t.Fatalf("frame payload changed: %v", frames[0].data)
	}

	stateMu.Lock()
	got := cloneDrawState(state)
	stateMu.Unlock()
	if len(got.pictures) != 2 || got.pictures[0].PictID != 100 || got.pictures[1].PictID != 200 {
		t.Fatalf("pictures not restored: %+v", got.pictures)
	}
	if got.pictures[0].H != -20 || got.pictures[1].V != -50 {
		t.Fatalf("picture positions not restored: %+v", got.pictures)
	}
	mob, ok := got.mobiles[3]
	if !ok || mob.State != 7 || mob.H != -123 || mob.V != 456 || mob.Colors != 5 {
		t.Fatalf("mobile not restored: %+v, present=%t", mob, ok)
	}
	desc := got.descriptors[3]
	if desc.Name != "Distortions" || desc.PictID != 447 || !bytes.Equal(desc.Colors, []byte{1, 2, 3, 4}) {
		t.Fatalf("descriptor not restored: %+v", desc)
	}
	if npc := got.descriptors[9]; npc.Name != "Trainer" || npc.PictID != 822 {
		t.Fatalf("descriptor-only entry not restored: %+v", npc)
	}
}

func TestNightSnapshotRoundTrip(t *testing.T) {
	originalNight := captureMovieNightState()
	t.Cleanup(func() { restoreMovieNightState(originalNight) })

	tmp := filepath.Join(t.TempDir(), "night-snapshot.clMov")
	mr, err := newMovieRecorder(tmp, 1497, 0)
	if err != nil {
		t.Fatalf("newMovieRecorder: %v", err)
	}
	want := movieNightState{baseLevel: 72, azimuth: 135, cloudy: true}
	mr.AddStateSnapshot(drawState{}, 1497, want)
	if err := mr.WriteFrame([]byte{0, 2, 0, 0}, 0); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}
	if err := mr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	restoreMovieNightState(movieNightState{})
	frames, err := parseMovie(tmp, 1497)
	if err != nil {
		t.Fatalf("parseMovie: %v", err)
	}
	if len(frames) != 1 || frames[0].flags&flagGameState == 0 {
		t.Fatalf("night snapshot frame = %+v", frames)
	}
	got := captureMovieNightState()
	if got.baseLevel != want.baseLevel || got.azimuth != want.azimuth || got.cloudy != want.cloudy {
		t.Fatalf("night snapshot = (%d, %d, %t), want (%d, %d, %t)", got.baseLevel, got.azimuth, got.cloudy, want.baseLevel, want.azimuth, want.cloudy)
	}
}

func TestWriteNetworkMessageQueuesExistingStateBlock(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "network-state.clMov")
	mr, err := newMovieRecorder(tmp, 1497, 0)
	if err != nil {
		t.Fatalf("newMovieRecorder: %v", err)
	}
	snapshot := drawState{
		descriptors: map[uint8]frameDescriptor{
			1: {Index: 1, Type: kDescPlayer, PictID: 447, Name: "Changed Clothes", Colors: []byte{9, 8, 7}},
		},
		mobiles: map[uint8]frameMobile{
			1: {Index: 1, H: 10, V: 20, Colors: 2},
		},
	}
	mobileBlock := encodeMobileTableSnapshot(snapshot, 1497)
	stateMessage := append([]byte{0, 3}, mobileBlock...)
	if err := mr.WriteNetworkMessage(stateMessage, flagMobileData); err != nil {
		t.Fatalf("WriteNetworkMessage state: %v", err)
	}
	drawFrame := []byte{0, 2, 0, 0}
	if err := mr.WriteNetworkMessage(drawFrame, 0); err != nil {
		t.Fatalf("WriteNetworkMessage draw: %v", err)
	}
	if err := mr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	frames, err := parseMovie(tmp, 1497)
	if err != nil {
		t.Fatalf("parseMovie: %v", err)
	}
	if len(frames) != 1 || !bytes.Equal(frames[0].data, drawFrame) {
		t.Fatalf("state message became a movie frame: %+v", frames)
	}
	if frames[0].flags&flagMobileData == 0 {
		t.Fatalf("mobile state flag missing: %#x", frames[0].flags)
	}
	stateMu.Lock()
	desc := state.descriptors[1]
	stateMu.Unlock()
	if desc.Name != "Changed Clothes" || !bytes.Equal(desc.Colors, []byte{9, 8, 7}) {
		t.Fatalf("mobile update not restored: %+v", desc)
	}
}

func TestCloseFlushesSnapshotWithoutAnotherFrame(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "snapshot-only.clMov")
	mr, err := newMovieRecorder(tmp, 1497, 0)
	if err != nil {
		t.Fatalf("newMovieRecorder: %v", err)
	}
	mr.AddStateSnapshot(drawState{
		descriptors: map[uint8]frameDescriptor{
			2: {Index: 2, Type: kDescPlayer, PictID: 447, Name: "Already Playing"},
		},
		mobiles: map[uint8]frameMobile{
			2: {Index: 2, H: 25, V: 50},
		},
	}, 1497, movieNightState{})
	if err := mr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	frames, err := parseMovie(tmp, 1497)
	if err != nil {
		t.Fatalf("parseMovie: %v", err)
	}
	if len(frames) != 1 || len(frames[0].data) != 0 {
		t.Fatalf("snapshot-only frames: %+v", frames)
	}
	stateMu.Lock()
	desc := state.descriptors[2]
	mob := state.mobiles[2]
	stateMu.Unlock()
	if desc.Name != "Already Playing" || mob.H != 25 || mob.V != 50 {
		t.Fatalf("snapshot not restored: desc=%+v mob=%+v", desc, mob)
	}
}

func TestParseMovieZip(t *testing.T) {
	dir := t.TempDir()
	moviePath := filepath.Join(dir, "test.clMov")
	if err := os.WriteFile(moviePath, readMovieFixture(t, "test.clMov"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	zipPath := filepath.Join(dir, "test.clMov.zip")
	if err := compressZip(moviePath, zipPath); err != nil {
		t.Fatalf("compressZip: %v", err)
	}
	if _, err := parseMovie(zipPath, 0); err != nil {
		t.Fatalf("parseMovie zip: %v", err)
	}
}
