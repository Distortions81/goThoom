package clsnd

import (
	"encoding/binary"
	"testing"
)

// Test that soundHeaderOffset ignores commands other than bufferCmd when the
// dataOffsetFlag is set.
func TestSoundHeaderOffsetSkipsNonBufferCommands(t *testing.T) {
	cmd := dataOffsetFlag | bufferCmd
	data := []byte{
		0x00, 0x01, // format 1
		0x00, 0x00, // nMods
		0x00, 0x02, // nCmds
		0x80, 0x10, // cmd1: not bufferCmd, high bit set
		0x00, 0x00, // param1
		0x00, 0x00, 0x00, 0x00, // param2 (ignored)
		byte(cmd >> 8), byte(cmd), // cmd2: bufferCmd | dataOffsetFlag (0x8051)
		0x00, 0x00, // param1
		0x00, 0x00, 0x00, 0x20, // param2 -> header offset 32
	}
	// pad up to offset 32
	if len(data) < 0x20 {
		data = append(data, make([]byte, 0x20-len(data))...)
	}
	// minimal header at offset 32
	header := make([]byte, 22)
	data = append(data, header...)

	off, ok := soundHeaderOffset(data)
	if !ok {
		t.Fatalf("soundHeaderOffset returned !ok")
	}
	if off != 0x20 {
		t.Fatalf("got offset %d want 32", off)
	}
}

// Test decoding of a minimal IMA4 compressed sound.
func TestDecodeHeaderIMA4(t *testing.T) {
	// Build a CmpSoundHeader at offset 0.
	hdr := make([]byte, 64)
	hdr[20] = 0xfe                                             // encode = CmpSoundHeader
	binary.BigEndian.PutUint32(hdr[4:8], 1)                    // channels
	binary.BigEndian.PutUint32(hdr[8:12], 22050<<16)           // sample rate 22050
	binary.BigEndian.PutUint32(hdr[22:26], 64)                 // frames
	binary.BigEndian.PutUint32(hdr[40:44], 0x696d6134)         // 'ima4'
	binary.BigEndian.PutUint16(hdr[56:58], uint16(^uint16(3))) // -4 as uint16
	binary.BigEndian.PutUint16(hdr[62:64], 16)                 // bits

	block := make([]byte, 36) // predictor=0, index=0, data=all zero -> silence

	data := append(hdr, block...)

	s, err := decodeHeader(data, 0, 1)
	if err != nil {
		t.Fatalf("decodeHeader returned error: %v", err)
	}
	if s.SampleRate != 22050 || s.Channels != 1 || s.Bits != 16 {
		t.Fatalf("unexpected params: %+v", s)
	}
	if len(s.Data) != 128 { // 64 samples * 2 bytes
		t.Fatalf("got %d bytes, want 128", len(s.Data))
	}
	for i, b := range s.Data {
		if b != 0 {
			t.Fatalf("data[%d]=%d, want 0", i, b)
		}
	}
}
