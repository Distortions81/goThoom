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

// Test that interpolatePictures chooses the closest unique previous picture
// when multiple pictures share the same ID.
func TestInterpolatePicturesFindsClosest(t *testing.T) {
	prev := []framePicture{{PictID: 1, H: 0, V: 0}, {PictID: 1, H: 2, V: 0}}
	newPics := []framePicture{{PictID: 1, H: 1, V: 0}, {PictID: 1, H: 1, V: 0}}
	interpolatePictures(prev, newPics, 0, 0, 0)
	seen := map[int16]bool{newPics[0].PrevH: true, newPics[1].PrevH: true}
	if !seen[0] || !seen[2] {
		t.Fatalf("expected matches to both previous pictures, got %#v", newPics)
	}
}
