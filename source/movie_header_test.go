package main

import (
	"encoding/binary"
	"fmt"
	"testing"
)

func TestParseMovieRejectsTruncatedHeader(t *testing.T) {
	header := make([]byte, 24)
	binary.BigEndian.PutUint32(header[:4], movieSignature)
	binary.BigEndian.PutUint16(header[4:6], 400)
	binary.BigEndian.PutUint16(header[6:8], 24)
	for size := range len(header) {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			defer func() {
				if value := recover(); value != nil {
					t.Fatalf("truncated header panicked: %v", value)
				}
			}()
			if _, err := parseMovieData(header[:size:size], 0); err == nil {
				t.Fatal("truncated header accepted")
			}
		})
	}
}
