package main

import "testing"

// Test that parseInventory skips kInvCmdFull|kInvCmdIndex and continues.
func TestParseInventoryIgnoresFullWithIndex(t *testing.T) {
	data := []byte{
		kInvCmdMultiple,
		2,
		kInvCmdFull | kInvCmdIndex,
		kInvCmdFull,
		0,
		kInvCmdNone,
	}
	rest, ok := parseInventory(data)
	if !ok {
		t.Fatalf("parseInventory failed for kInvCmdFull|kInvCmdIndex")
	}
	if len(rest) != 0 {
		t.Fatalf("unexpected leftover bytes: %d", len(rest))
	}
}

// Test that parseInventory skips kInvCmdNone|kInvCmdIndex and continues.
func TestParseInventoryIgnoresNoneWithIndex(t *testing.T) {
	data := []byte{
		kInvCmdMultiple,
		2,
		kInvCmdNone | kInvCmdIndex,
		kInvCmdFull,
		0,
		kInvCmdNone,
	}
	rest, ok := parseInventory(data)
	if !ok {
		t.Fatalf("parseInventory failed for kInvCmdNone|kInvCmdIndex")
	}
	if len(rest) != 0 {
		t.Fatalf("unexpected leftover bytes: %d", len(rest))
	}
}

// Test that interpolatePictures matches moving pictures to the nearest
// previously seen positions even when input ordering differs.
func TestInterpolatePicturesFindsClosest(t *testing.T) {
	prev := []framePicture{{PictID: 1, H: 0, V: 0}, {PictID: 1, H: 10, V: 0}}
	newPics := []framePicture{{PictID: 1, H: 9, V: 0}, {PictID: 1, H: 1, V: 0}}
	interpolatePictures(prev, newPics, 0, 0, 0)
	if newPics[0].PrevH != 10 || newPics[1].PrevH != 0 {
		t.Fatalf("expected matches to nearest previous pictures, got %#v", newPics)
	}
}
