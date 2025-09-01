//go:build integration
// +build integration

package main

import "testing"

func TestInventoryOrderSortedWithShortcuts(t *testing.T) {
	resetInventory()
	inventoryShortcutMu.Lock()
	inventoryShortcuts = map[int]rune{}
	inventoryShortcutMu.Unlock()

	addInventoryItem(1, -1, "Banana", false)
	addInventoryItem(2, -1, "Ápple", false)
	addInventoryItem(3, -1, "apple", false)
	addInventoryItem(4, -1, "ápple", false)

	inventoryShortcutMu.Lock()
	inventoryShortcuts[1] = '1'
	inventoryShortcutMu.Unlock()

	inventoryWin = nil
	inventoryList = nil
	makeInventoryWindow()

	if len(inventoryList.Contents) < 4 {
		t.Fatalf("unexpected list length: %d", len(inventoryList.Contents))
	}
	got := []string{
		inventoryList.Contents[0].Contents[1].Text,
		inventoryList.Contents[1].Contents[1].Text,
		inventoryList.Contents[2].Contents[1].Text,
		inventoryList.Contents[3].Contents[1].Text,
	}
	want := []string{
		TitleCaser.String("Ápple"),
		TitleCaser.String("apple"),
		TitleCaser.String("ápple"),
		TitleCaser.String("Banana"),
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: got %q want %q", i, got[i], want[i])
		}
	}
}
