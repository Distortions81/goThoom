package clsnd

import (
	"bytes"
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

// helper to create CL_Sounds file with one ExtSoundHeader sound
func createExtTestFile(t *testing.T, dir string) string {
	sample := []byte{0x01, 0x02, 0x03, 0x04}
	snd := make([]byte, 14+44+len(sample))
	binary.BigEndian.PutUint16(snd[0:2], 0x0001)  // format
	binary.BigEndian.PutUint16(snd[2:4], 0x0000)  // numModifiers
	binary.BigEndian.PutUint16(snd[4:6], 0x0001)  // numCommands
	binary.BigEndian.PutUint16(snd[6:8], 0x8051)  // cmd = bufferCmd | dataOffsetFlag
	binary.BigEndian.PutUint16(snd[8:10], 0x0000) // param1
	binary.BigEndian.PutUint32(snd[10:14], 14)    // param2 -> offset to SoundHeader
	hdr := 14
	binary.BigEndian.PutUint32(snd[hdr+0:hdr+4], 0)           // samplePtr
	binary.BigEndian.PutUint32(snd[hdr+4:hdr+8], 1)           // frames
	binary.BigEndian.PutUint32(snd[hdr+8:hdr+12], 0x56220000) // sampleRate 22050<<16
	binary.BigEndian.PutUint32(snd[hdr+12:hdr+16], 0)         // loopStart
	binary.BigEndian.PutUint32(snd[hdr+16:hdr+20], 1)         // loopEnd
	snd[hdr+20] = 0xff                                        // encode ExtSoundHeader
	snd[hdr+21] = 0x3c                                        // baseFrequency
	binary.BigEndian.PutUint32(snd[hdr+24:hdr+28], 2)         // channels
	binary.BigEndian.PutUint16(snd[hdr+28:hdr+30], 16)        // bits
	copy(snd[hdr+44:], sample)                                // sample data

	header := make([]byte, 12)
	binary.BigEndian.PutUint16(header[0:2], 0xffff)
	binary.BigEndian.PutUint32(header[2:6], 1) // one entry
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

// helper to create CL_Sounds file with truncated sample data
func createTruncTestFile(t *testing.T, dir string) string {
	snd := []byte{
		0x00, 0x01, // format
		0x00, 0x00, // numModifiers
		0x00, 0x01, // numCommands
		0x80, 0x51, // cmd = bufferCmd | dataOffsetFlag
		0x00, 0x00, // param1
		0x00, 0x00, 0x00, 0x0e, // param2 -> offset to SoundHeader (14)
		// SoundHeader at offset 14
		0x00, 0x00, 0x00, 0x00, // samplePtr
		0x00, 0x00, 0x00, 0x02, // length claims 2 bytes
		0x56, 0x22, 0x00, 0x00, // sampleRate 22050<<16
		0x00, 0x00, 0x00, 0x00, // loopStart
		0x00, 0x00, 0x00, 0x02, // loopEnd
		0x00, // encode stdSH
		0x3c, // baseFrequency
		0x80, // only 1 byte of sample data
	}
	header := make([]byte, 12)
	binary.BigEndian.PutUint16(header[0:2], 0xffff)
	binary.BigEndian.PutUint32(header[2:6], 1) // one entry
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

func TestLoadAndGetExt(t *testing.T) {
	dir := t.TempDir()
	path := createExtTestFile(t, dir)
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
	if s.Channels != 2 || s.Bits != 16 {
		t.Fatalf("channels %d bits %d", s.Channels, s.Bits)
	}
	want := []byte{0x01, 0x02, 0x03, 0x04}
	if !bytes.Equal(s.Data, want) {
		t.Fatalf("data %#v", s.Data)
	}
}

func TestTruncatedSound(t *testing.T) {
	dir := t.TempDir()
	path := createTruncTestFile(t, dir)
	cs, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	s := cs.Get(1)
	if s == nil {
		t.Fatalf("Get returned nil")
	}
	if len(s.Data) != 1 || s.Data[0] != 0x80 {
		t.Fatalf("data %#v", s.Data)
	}
}
