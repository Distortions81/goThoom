package clsnd

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// TestDecodeAllSounds loads the CL_Sounds archive and attempts to decode
// every sound entry. It ensures that the decoded PCM data appears valid.
func TestDecodeAllSounds(t *testing.T) {
	path := filepath.Join("..", "data", "CL_Sounds")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			t.Skip("CL_Sounds file not present")
		}
		t.Fatalf("stat CL_Sounds: %v", err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load CL_Sounds: %v", err)
	}

	var (
		count8Bit    int
		countHighHz  int
		countIMAComp int
	)

	for _, id := range c.IDs() {
		s, err := c.Get(id)
		if err != nil {
			t.Fatalf("Get(%d): %v", id, err)
		}
		if s == nil {
			t.Fatalf("Get(%d) returned nil", id)
		}
		if s.SampleRate == 0 || s.Channels == 0 || s.Bits == 0 {
			t.Fatalf("sound %d has invalid parameters: %+v", id, s)
		}
		if len(s.Data) == 0 {
			t.Fatalf("sound %d has no data", id)
		}
		bytesPerSample := int(s.Bits) / 8
		if bytesPerSample == 0 || len(s.Data)%(bytesPerSample*int(s.Channels)) != 0 {
			t.Fatalf("sound %d: data length %d not aligned with params", id, len(s.Data))
		}

		if s.Bits == 8 {
			count8Bit++
		}
		if s.SampleRate > 22050 {
			countHighHz++
		}

		e := c.index[id]
		sndData := c.data[e.offset : e.offset+e.size]
		hdrOff, ok := soundHeaderOffset(sndData)
		if !ok {
			t.Fatalf("sound %d: unable to locate header", id)
		}
		if hdrOff+22 > len(sndData) {
			t.Fatalf("sound %d: truncated header", id)
		}
		if sndData[hdrOff+20] == 0xfe { // CmpSoundHeader
			if hdrOff+64 > len(sndData) {
				t.Fatalf("sound %d: short CmpSoundHeader", id)
			}
			compID := int16(binary.BigEndian.Uint16(sndData[hdrOff+56 : hdrOff+58]))
			if compID == -4 {
				countIMAComp++
			}
		}
	}
	t.Logf("8-bit sounds: %d, >22kHz sounds: %d, IMA sounds: %d", count8Bit, countHighHz, countIMAComp)
}
