package clsnd

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// helper to create minimal CL_Sounds file with one 8-bit sound
func createTestFile(t *testing.T, dir string) string {
	snd := []byte{
		0x00, 0x01, // format
		0x00, 0x00, // numModifiers
		0x00, 0x01, // numCommands
		0x80, 0x51, // cmd = bufferCmd | dataOffsetFlag
		0x00, 0x00, // param1
		0x00, 0x00, 0x00, 0x0e, // param2 -> offset to SoundHeader (14)
		// SoundHeader at offset 14
		0x00, 0x00, 0x00, 0x00, // samplePtr
		0x00, 0x00, 0x00, 0x01, // length
		0x56, 0x22, 0x00, 0x00, // sampleRate 22050<<16
		0x00, 0x00, 0x00, 0x00, // loopStart
		0x00, 0x00, 0x00, 0x01, // loopEnd
		0x00, // encode stdSH
		0x3c, // baseFrequency
		0x80, // sample data (1 byte)
	}
	// Build keyfile
	header := make([]byte, 12)
	binary.BigEndian.PutUint16(header[0:2], 0xffff)
	binary.BigEndian.PutUint32(header[2:6], 1) // one entry
	// pad1 and pad2 already zero
	table := make([]byte, 16)
	offset := uint32(len(header) + len(table))
	binary.BigEndian.PutUint32(table[0:4], offset)
	binary.BigEndian.PutUint32(table[4:8], uint32(len(snd)))
	binary.BigEndian.PutUint32(table[8:12], typeSound)
	binary.BigEndian.PutUint32(table[12:16], 1)
	data := append(header, table...)
	data = append(data, snd...)
	path := filepath.Join(dir, "CL_Sounds")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	return path
}

func TestLoadAndGet(t *testing.T) {
	dir := t.TempDir()
	path := createTestFile(t, dir)
	cs, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	s := cs.Get(1)
	if s == nil {
		t.Fatalf("Get returned nil")
	}
	if s.SampleRate != 22050 {
		t.Fatalf("sample rate %d", s.SampleRate)
	}
	if s.Channels != 1 || s.Bits != 8 {
		t.Fatalf("channels %d bits %d", s.Channels, s.Bits)
	}
	if len(s.Data) != 1 || s.Data[0] != 0x80 {
		t.Fatalf("data %#v", s.Data)
	}
}
