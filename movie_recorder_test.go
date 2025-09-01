package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRecordRoundTrip(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	moviePath := filepath.Join(filepath.Dir(file), "clmovFiles", "test.clMov")
	orig, err := os.ReadFile(moviePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
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
}
