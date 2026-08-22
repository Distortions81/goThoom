package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
)

// verifyClmov parses a .clMov file with parseMovie, re-encodes it using our
// writer format (header + per-frame: 12-byte header, preData, data) and
// compares the bytes against the original. It returns nil if identical.
func verifyClmov(path string, clVersion int) error {
	orig, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	if len(orig) < 24 || binary.BigEndian.Uint32(orig[:4]) != movieSignature {
		return fmt.Errorf("not a clMov or too short")
	}
	headerLen := int(binary.BigEndian.Uint16(orig[6:8]))
	if headerLen <= 0 || headerLen > len(orig) {
		headerLen = 24
	}

	frames, err := parseMovie(path, clVersion)
	if err != nil {
		return fmt.Errorf("parseMovie: %w", err)
	}

	// Re-encode
	var buf bytes.Buffer
	buf.Write(orig[:headerLen])
	for _, fr := range frames {
		h := make([]byte, 12)
		binary.BigEndian.PutUint32(h[0:], movieSignature)
		binary.BigEndian.PutUint32(h[4:], uint32(fr.index))
		binary.BigEndian.PutUint16(h[8:], uint16(len(fr.data)))
		binary.BigEndian.PutUint16(h[10:], fr.flags)
		buf.Write(h)
		if len(fr.preData) > 0 {
			buf.Write(fr.preData)
		}
		if len(fr.data) > 0 {
			buf.Write(fr.data)
		}
	}

	enc := buf.Bytes()
	if len(enc) != len(orig) || !bytes.Equal(enc, orig) {
		// Find first difference for debugging
		max := len(enc)
		if len(orig) < max {
			max = len(orig)
		}
		i := 0
		for ; i < max; i++ {
			if enc[i] != orig[i] {
				break
			}
		}
		return fmt.Errorf("mismatch at offset %d (encLen=%d origLen=%d)", i, len(enc), len(orig))
	}
	return nil
}
