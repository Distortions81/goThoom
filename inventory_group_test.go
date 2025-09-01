//go:build integration
// +build integration

package main

import "testing"

func TestInventorySeparateNames(t *testing.T) {
	resetInventory()
	addInventoryItem(100, -1, "First", false)
	addInventoryItem(100, -1, "Second", false)
	items := getInventory()
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
}

func TestInventoryGroupNormalizedNames(t *testing.T) {
	resetInventory()
	addInventoryItem(100, -1, "Shadow Bell", false)
	addInventoryItem(100, -1, "shadow bell", false)
	items := getInventory()
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Quantity != 2 {
		t.Fatalf("expected quantity 2, got %d", items[0].Quantity)
	}
}

func TestToggleInventoryEquipAt(t *testing.T) {
	resetInventory()
	addInventoryItem(100, 0, "Ring A", false)
	addInventoryItem(100, 1, "Ring B", false)
	toggleInventoryEquipAt(100, 1)
	items := getInventory()
	if !items[1].Equipped {
		t.Fatalf("expected second item equipped")
	}
}
