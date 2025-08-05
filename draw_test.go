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
