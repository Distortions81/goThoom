package main

import (
	"encoding/binary"
	"os"
	"strings"
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
	path := writeHeader(t, 0, 1450)
	defer os.Remove(path)
	if _, err := parseMovie(path, 1440); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("expected newer client error, got %v", err)
	}
}

func TestParseMovieStoresRevision(t *testing.T) {
	path := writeHeader(t, 7, 1400)
	defer os.Remove(path)
	movieRevision = 0
	if _, err := parseMovie(path, 1440); err != nil {
		t.Fatalf("parseMovie: %v", err)
	}
	if movieRevision != 7 {
		t.Fatalf("movieRevision = %d", movieRevision)
	}
}
