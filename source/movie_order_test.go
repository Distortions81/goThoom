package main

import (
	"encoding/binary"
	"os"
	"testing"
)

// helper to reset global state
func resetState() {
	stateMu.Lock()
	state = drawState{
		descriptors: make(map[uint8]frameDescriptor),
		mobiles:     make(map[uint8]frameMobile),
		prevMobiles: make(map[uint8]frameMobile),
		prevDescs:   make(map[uint8]frameDescriptor),
	}
	stateMu.Unlock()
}

func TestParseGameStatePictureTableOrder(t *testing.T) {
	resetState()
	// Build game state with unsorted picture table
	pt := make([]byte, 2+6*3+4)
	binary.BigEndian.PutUint16(pt[0:2], 3)
	// picture 1
	binary.BigEndian.PutUint16(pt[2:4], 1)
	binary.BigEndian.PutUint16(pt[4:6], 0)
	binary.BigEndian.PutUint16(pt[6:8], 20)
	// picture 2
	binary.BigEndian.PutUint16(pt[8:10], 2)
	binary.BigEndian.PutUint16(pt[10:12], 0)
	binary.BigEndian.PutUint16(pt[12:14], 10)
	// picture 3
	binary.BigEndian.PutUint16(pt[14:16], 3)
	binary.BigEndian.PutUint16(pt[16:18], 0)
	binary.BigEndian.PutUint16(pt[18:20], 30)
	// Prepend a dummy string and null terminator
	data := append([]byte("x\x00"), pt...)

	parseGameState(data, 200, 0)
	stateMu.Lock()
	pics := append([]framePicture(nil), state.pictures...)
	stateMu.Unlock()
	if len(pics) != 3 {
		t.Fatalf("expected 3 pictures, got %d", len(pics))
	}
	if pics[0].PictID != 1 || pics[1].PictID != 2 || pics[2].PictID != 3 {
		t.Fatalf("order preserved: %+v", pics)
	}
}

func TestParseMoviePictureTableOrder(t *testing.T) {
	resetState()
	header := make([]byte, 24)
	binary.BigEndian.PutUint32(header[0:4], movieSignature)
	binary.BigEndian.PutUint16(header[4:6], 200)
	binary.BigEndian.PutUint16(header[6:8], 24)
	binary.BigEndian.PutUint16(header[16:18], 0)

	frame := make([]byte, 12)
	binary.BigEndian.PutUint32(frame[0:4], movieSignature)
	// frame index 0
	binary.BigEndian.PutUint32(frame[4:8], 0)
	// size 0, flags picture table
	binary.BigEndian.PutUint16(frame[8:10], 0)
	binary.BigEndian.PutUint16(frame[10:12], flagPictureTable)

	pt := make([]byte, 2+6*3+4)
	binary.BigEndian.PutUint16(pt[0:2], 3)
	// picture 1
	binary.BigEndian.PutUint16(pt[2:4], 1)
	binary.BigEndian.PutUint16(pt[4:6], 0)
	binary.BigEndian.PutUint16(pt[6:8], 20)
	// picture 2
	binary.BigEndian.PutUint16(pt[8:10], 2)
	binary.BigEndian.PutUint16(pt[10:12], 0)
	binary.BigEndian.PutUint16(pt[12:14], 10)
	// picture 3
	binary.BigEndian.PutUint16(pt[14:16], 3)
	binary.BigEndian.PutUint16(pt[16:18], 0)
	binary.BigEndian.PutUint16(pt[18:20], 30)

	data := append(header, frame...)
	data = append(data, pt...)

	tmp, err := os.CreateTemp("", "movie-*.clMov")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		t.Fatalf("write: %v", err)
	}
	tmp.Close()

	if _, err := parseMovie(tmp.Name(), 200); err != nil {
		t.Fatalf("parseMovie: %v", err)
	}
	stateMu.Lock()
	pics := append([]framePicture(nil), state.pictures...)
	stateMu.Unlock()
	if len(pics) != 3 {
		t.Fatalf("expected 3 pictures, got %d", len(pics))
	}
	if pics[0].PictID != 1 || pics[1].PictID != 2 || pics[2].PictID != 3 {
		t.Fatalf("order preserved: %+v", pics)
	}
}
