package main

import (
	"testing"
)

// helper function to check inventory
func getInventorySnapshot() []inventoryItem {
	inventoryMu.RLock()
	defer inventoryMu.RUnlock()
	out := make([]inventoryItem, len(inventoryItems))
	copy(out, inventoryItems)
	return out
}

func TestParseInventoryFullWithIndex(t *testing.T) {
	resetInventory()
	// command: full inventory with index byte
	pkt := []byte{
		kInvCmdFull | kInvCmdIndex,
		5,          // index byte to be ignored
		2,          // item count
		0x02,       // equip bits: second item equipped
		0x00, 0x0A, // item ID 10
		0x00, 0x14, // item ID 20
		kInvCmdNone | kInvCmdIndex,
		0, // trailing index
	}
	remain, ok := parseInventory(pkt)
	if !ok {
		t.Fatalf("parseInventory returned !ok")
	}
	if len(remain) != 0 {
		t.Fatalf("expected no remaining data, got %d bytes", len(remain))
	}
	inv := getInventorySnapshot()
	if len(inv) != 2 {
		t.Fatalf("expected 2 items, got %d", len(inv))
	}
	if inv[0].ID != 10 || inv[0].Index != 0 || inv[0].Equipped {
		t.Fatalf("unexpected first item: %+v", inv[0])
	}
	if inv[1].ID != 20 || inv[1].Index != 1 || !inv[1].Equipped {
		t.Fatalf("unexpected second item: %+v", inv[1])
	}
}

func TestParseInventoryNoneWithIndex(t *testing.T) {
	resetInventory()
	addInventoryItem(42, 0, "Item 42", false)
	pkt := []byte{
		kInvCmdNone | kInvCmdIndex,
		7, // index byte to be ignored
	}
	remain, ok := parseInventory(pkt)
	if !ok {
		t.Fatalf("parseInventory returned !ok")
	}
	if len(remain) != 0 {
		t.Fatalf("expected no remaining data, got %d bytes", len(remain))
	}
	inv := getInventorySnapshot()
	if len(inv) != 1 || inv[0].ID != 42 {
		t.Fatalf("inventory changed unexpectedly: %+v", inv)
	}
}
