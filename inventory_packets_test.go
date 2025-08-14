package main

import "testing"

func TestParseInventoryFull(t *testing.T) {
	resetInventory()
	inventoryDirty = false
	data := []byte{byte(kInvCmdFull), 2, 0x02, 0x00, 0x64, 0x00, 0xC8, byte(kInvCmdNone), 0x99}
	rest, ok := parseInventory(data)
	if !ok {
		t.Fatalf("parse failed")
	}
	if len(rest) != 1 || rest[0] != 0x99 {
		t.Fatalf("unexpected rest %v", rest)
	}
	inv := getInventory()
	if len(inv) != 2 {
		t.Fatalf("unexpected inventory length %d", len(inv))
	}
	found100, found200 := false, false
	for _, it := range inv {
		if it.ID == 100 && !it.Equipped {
			found100 = true
		}
		if it.ID == 200 && it.Equipped {
			found200 = true
		}
	}
	if !found100 || !found200 {
		t.Fatalf("unexpected inventory %v", inv)
	}
	if !inventoryDirty {
		t.Fatalf("inventoryDirty not set")
	}
}

func TestParseInventoryOther(t *testing.T) {
	resetInventory()
	inventoryDirty = false
	data := []byte{
		byte(kInvCmdMultiple), 4, byte(kInvCmdAdd | kInvCmdIndex),
		0x00, 0x64, 0, 'S', 't', 'a', 'f', 'f', 0,
		byte(kInvCmdEquip | kInvCmdIndex), 0x00, 0x64, 0,
		byte(kInvCmdName | kInvCmdIndex), 0x00, 0x64, 0, 'S', 't', 'a', 'f', 'f', '+', 0,
		byte(kInvCmdDelete | kInvCmdIndex), 0x00, 0x64, 0,
		byte(kInvCmdNone), 0x77,
	}
	rest, ok := parseInventory(data)
	if !ok {
		t.Fatalf("parse failed")
	}
	if len(rest) != 1 || rest[0] != 0x77 {
		t.Fatalf("unexpected rest %v", rest)
	}
	if !inventoryDirty {
		t.Fatalf("inventoryDirty not set")
	}
}

func TestParseInventoryMacRomanName(t *testing.T) {
	resetInventory()
	inventoryDirty = false
	nameBytes := []byte{'M', 0x8e, 'm', 'e'}
	data := []byte{
		byte(kInvCmdAdd | kInvCmdIndex), 0x00, 0x64, 0,
	}
	data = append(data, nameBytes...)
	data = append(data, 0, byte(kInvCmdNone), 0x55)
	rest, ok := parseInventory(data)
	if !ok {
		t.Fatalf("parse failed")
	}
	if len(rest) != 1 || rest[0] != 0x55 {
		t.Fatalf("unexpected rest %v", rest)
	}
	inv := getInventory()
	if len(inv) != 1 || inv[0].Name != decodeMacRoman(nameBytes) {
		t.Fatalf("unexpected inventory %v", inv)
	}
	if !inventoryDirty {
		t.Fatalf("inventoryDirty not set")
	}
}

func TestParseInventoryTrailingB1(t *testing.T) {
	resetInventory()
	inventoryDirty = false
	data := []byte{
		byte(kInvCmdFull), 1, 0x00, 0x00, 0x64,
		kInvCmdLegacyPadding, byte(kInvCmdNone), 0x55,
	}
	rest, ok := parseInventory(data)
	if !ok {
		t.Fatalf("parse failed")
	}
	if len(rest) != 1 || rest[0] != 0x55 {
		t.Fatalf("unexpected rest %v", rest)
	}
	if !inventoryDirty {
		t.Fatalf("inventoryDirty not set")
	}
}
