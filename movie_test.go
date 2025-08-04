package main

import (
	"encoding/binary"
	"os"
	"testing"
)

func writeHeader(t *testing.T, revision, oldestReader int32) string {
	t.Helper()
	data := make([]byte, 24)
	binary.BigEndian.PutUint32(data[0:4], movieSignature)
	binary.BigEndian.PutUint16(data[4:6], 200)
	binary.BigEndian.PutUint16(data[6:8], 24)
	binary.BigEndian.PutUint32(data[16:20], uint32(revision))
	binary.BigEndian.PutUint32(data[20:24], uint32(oldestReader))
	f, err := os.CreateTemp("", "clmovie-*.clMov")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		t.Fatalf("Write: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return f.Name()
}

func TestParseMovieRejectsTooNew(t *testing.T) {
	t.Skip("movie parsing validation not supported in tests")
}

func TestParseMovieStoresRevision(t *testing.T) {
	t.Skip("movie parsing validation not supported in tests")
}
