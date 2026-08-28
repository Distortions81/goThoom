package main

import "testing"

func TestIndexedInventoryPacketKeepsClassicZeroBasedIndex(t *testing.T) {
	resetInventory()
	t.Cleanup(resetInventory)

	for idx, name := range []string{"First", "Second"} {
		data := []byte{0, 100, byte(idx)}
		data = append(data, name...)
		data = append(data, 0)
		if rest, ok := handleInvCmdOther(kInvCmdAdd|kInvCmdIndex, data); !ok || len(rest) != 0 {
			t.Fatalf("add indexed item %d failed: ok=%v rest=%v", idx, ok, rest)
		}
	}

	items := getInventory()
	if len(items) != 2 {
		t.Fatalf("inventory length = %d, want 2", len(items))
	}
	for idx, item := range items {
		if item.IDIndex != idx {
			t.Fatalf("item %d IDIndex = %d, want wire index %d", idx, item.IDIndex, idx)
		}
	}
	if got, want := formatEquipCommand(items[1].ID, items[1].IDIndex), "/equip 100 2"; got != want {
		t.Fatalf("second item equip command = %q, want %q", got, want)
	}
}
