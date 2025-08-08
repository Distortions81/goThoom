package main

import "testing"

func TestParseInventoryEmpty(t *testing.T) {
	rem, ok := parseInventory(nil)
	if !ok || rem != nil {
		t.Fatalf("expected ok with nil remainder, got ok=%v rem=%v", ok, rem)
	}
}

func TestParseInventoryFullWithIndex(t *testing.T) {
	resetInventory()
	data := []byte{byte(kInvCmdFull | kInvCmdIndex), 0x00, 0x01, 0x00, 0x00, 0x02}
	rem, ok := parseInventory(data)
	if !ok || len(rem) != 0 {
		t.Fatalf("unexpected parse result ok=%v len(rem)=%d", ok, len(rem))
	}
	items := getInventory()
	if len(items) != 1 || items[0].ID != 2 {
		t.Fatalf("inventory not updated: %+v", items)
	}
}

func TestParseInventoryNoneWithIndex(t *testing.T) {
	data := []byte{byte(kInvCmdNone | kInvCmdIndex), 0x05}
	rem, ok := parseInventory(data)
	if !ok || len(rem) != 0 {
		t.Fatalf("unexpected parse result ok=%v len(rem)=%d", ok, len(rem))
	}
}
