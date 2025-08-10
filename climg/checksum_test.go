package climg

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

func buildTestFile(t *testing.T, corrupt bool) string {
	bits := []byte{1, 2, 3, 4}
	colors := []byte{5, 6, 7, 8}
	ref := &dataLocation{
		id:        1,
		version:   1,
		imageID:   2,
		colorID:   3,
		numFrames: 1,
		numAnims:  0,
	}
	sum := calculateChecksum(bits, colors, nil, ref)
	if corrupt {
		sum++
	}
	// build idref record
	idrefBuf := &bytes.Buffer{}
	binary.Write(idrefBuf, binary.BigEndian, ref.version)
	binary.Write(idrefBuf, binary.BigEndian, ref.imageID)
	binary.Write(idrefBuf, binary.BigEndian, ref.colorID)
	binary.Write(idrefBuf, binary.BigEndian, sum)
	binary.Write(idrefBuf, binary.BigEndian, uint32(0)) // flags
	binary.Write(idrefBuf, binary.BigEndian, uint32(0)) // unusedFlags
	binary.Write(idrefBuf, binary.BigEndian, uint32(0)) // unusedFlags2
	binary.Write(idrefBuf, binary.BigEndian, int32(0))  // lightingID
	binary.Write(idrefBuf, binary.BigEndian, int16(0))  // plane
	binary.Write(idrefBuf, binary.BigEndian, uint16(1)) // numFrames
	binary.Write(idrefBuf, binary.BigEndian, uint16(0)) // numAnims
	idrefData := idrefBuf.Bytes()

	entryCount := uint32(3)
	headerSize := 12
	tableSize := int(entryCount) * 16
	offset := uint32(headerSize + tableSize)

	buf := &bytes.Buffer{}
	binary.Write(buf, binary.BigEndian, uint16(0xFFFF))
	binary.Write(buf, binary.BigEndian, entryCount)
	binary.Write(buf, binary.BigEndian, uint32(0))
	binary.Write(buf, binary.BigEndian, uint16(0))

	// bits entry
	binary.Write(buf, binary.BigEndian, offset)
	binary.Write(buf, binary.BigEndian, uint32(len(bits)))
	binary.Write(buf, binary.BigEndian, uint32(TYPE_IMAGE))
	binary.Write(buf, binary.BigEndian, uint32(2))
	offset += uint32(len(bits))

	// color entry
	binary.Write(buf, binary.BigEndian, offset)
	binary.Write(buf, binary.BigEndian, uint32(len(colors)))
	binary.Write(buf, binary.BigEndian, uint32(TYPE_COLOR))
	binary.Write(buf, binary.BigEndian, uint32(3))
	offset += uint32(len(colors))

	// idref entry
	binary.Write(buf, binary.BigEndian, offset)
	binary.Write(buf, binary.BigEndian, uint32(len(idrefData)))
	binary.Write(buf, binary.BigEndian, uint32(TYPE_IDREF))
	binary.Write(buf, binary.BigEndian, uint32(1))

	// data
	buf.Write(bits)
	buf.Write(colors)
	buf.Write(idrefData)

	tmp, err := os.CreateTemp("", "climg-test-*.bin")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	tmp.Close()
	return tmp.Name()
}

func TestLoadChecksumValid(t *testing.T) {
	path := buildTestFile(t, false)
	defer os.Remove(path)
	if _, err := Load(path); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
}

func TestLoadChecksumMismatch(t *testing.T) {
	path := buildTestFile(t, true)
	defer os.Remove(path)
	if _, err := Load(path); err == nil {
		t.Fatalf("expected checksum error, got nil")
	}
}
