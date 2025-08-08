package clsnd

import "testing"

// Test that soundHeaderOffset ignores commands other than bufferCmd when the
// dataOffsetFlag is set.
func TestSoundHeaderOffsetSkipsNonBufferCommands(t *testing.T) {
	cmd1 := dataOffsetFlag | 0x10 // not bufferCmd, high bit set
	cmd2 := dataOffsetFlag | bufferCmd
	data := []byte{
		0x00, 0x01, // format 1
		0x00, 0x00, // nMods
		0x00, 0x02, // nCmds
		byte(cmd1 >> 8), byte(cmd1), // cmd1
		0x00, 0x00, // param1
		0x00, 0x00, 0x00, 0x00, // param2 (ignored)
		byte(cmd2 >> 8), byte(cmd2), // cmd2: bufferCmd | dataOffsetFlag
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
