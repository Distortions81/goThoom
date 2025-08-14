package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"

	"gothoom/clsnd"
)

// writeTestCLS creates a minimal CL_Sounds archive containing
// a single 8-bit mono sound with ID 1.
func writeTestCLS(t *testing.T) string {
	t.Helper()
	data := []byte{0x80, 0x80, 0x80, 0x80}

	snd := make([]byte, 14)
	binary.BigEndian.PutUint16(snd[0:], 1)
	binary.BigEndian.PutUint16(snd[2:], 0)
	binary.BigEndian.PutUint16(snd[4:], 1)
	binary.BigEndian.PutUint16(snd[6:], 0x8051)
	binary.BigEndian.PutUint16(snd[8:], 0)
	binary.BigEndian.PutUint32(snd[10:], 14)

	hdr := make([]byte, 22)
	binary.BigEndian.PutUint32(hdr[4:], uint32(len(data)))
	binary.BigEndian.PutUint32(hdr[8:], uint32(22050<<16))
	hdr[20] = 0

	snd = append(snd, hdr...)
	snd = append(snd, data...)

	buf := bytes.NewBuffer(nil)
	buf.Write([]byte{0xff, 0xff})
	binary.Write(buf, binary.BigEndian, uint32(1))
	buf.Write(make([]byte, 10))
	binary.Write(buf, binary.BigEndian, uint32(32))
	binary.Write(buf, binary.BigEndian, uint32(len(snd)))
	binary.Write(buf, binary.BigEndian, uint32(0x736e6420))
	binary.Write(buf, binary.BigEndian, uint32(1))
	buf.Write(snd)

	f, err := os.CreateTemp(t.TempDir(), "CL_Sounds")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func TestPlaySoundIDs(t *testing.T) {
	path := writeTestCLS(t)
	var err error
	clSounds, err = clsnd.Load(path)
	if err != nil {
		t.Fatalf("load CL_Sounds: %v", err)
	}
	initSoundContext()
	gs.Volume = 1

	messages = nil
	soundMu.Lock()
	soundPlayers = make(map[*audio.Player]struct{})
	soundMu.Unlock()

	playSound(1)
	time.Sleep(50 * time.Millisecond)
	if len(messages) != 0 {
		t.Fatalf("unexpected messages for valid id: %v", messages)
	}
	soundMu.Lock()
	have := len(soundPlayers) > 0
	soundMu.Unlock()
	if !have {
		t.Fatalf("sound player not created for valid id")
	}

	messages = nil
	playSound(2)
	time.Sleep(50 * time.Millisecond)
	if len(messages) != 0 {
		t.Fatalf("unexpected messages for unknown id: %v", messages)
	}
}
