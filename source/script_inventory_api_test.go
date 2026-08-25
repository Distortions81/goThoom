package main

import (
	"testing"

	"gothoom/climg"
)

func withScriptInventoryTestState(t *testing.T) {
	t.Helper()
	inventoryMu.Lock()
	originalItems := inventoryItems
	originalNames := inventoryNames
	originalSequence := inventoryInstanceSequence.Load()
	inventoryItems = nil
	inventoryNames = map[inventoryKey]string{}
	inventoryInstanceSequence.Store(0)
	inventoryMu.Unlock()
	originalImages := clImages
	t.Cleanup(func() {
		inventoryMu.Lock()
		inventoryItems = originalItems
		inventoryNames = originalNames
		inventoryInstanceSequence.Store(originalSequence)
		inventoryMu.Unlock()
		clImages = originalImages
	})
}

func TestInventoryInstanceIDSurvivesIndexChanges(t *testing.T) {
	withScriptInventoryTestState(t)
	addInventoryItem(100, 0, "Rune", false)
	addInventoryItem(100, 1, "Rune", false)
	items := getInventory()
	if len(items) != 2 || items[0].InstanceID == 0 || items[1].InstanceID == 0 || items[0].InstanceID == items[1].InstanceID {
		t.Fatalf("instance IDs = %+v", items)
	}
	secondID := items[1].InstanceID
	removeInventoryItem(100, 0)
	items = getInventory()
	if len(items) != 1 || items[0].IDIndex != 0 || items[0].InstanceID != secondID {
		t.Fatalf("remaining instance = %+v, want ID %d", items, secondID)
	}
}

func TestScriptInventoryLookups(t *testing.T) {
	withScriptInventoryTestState(t)
	clImages = testCLImages(map[uint32]*climg.ClientItem{
		100: {Name: "Moon Blade", Slot: kItemSlotRightHand},
	})
	addInventoryItem(100, 0, "Moon Blade", true)
	addInventoryItem(100, 1, "Moon Blade", false)

	if _, ok := scriptFindItemExact("moon blade"); ok {
		t.Fatal("case-sensitive exact lookup accepted different casing")
	}
	item, ok := scriptFindItem("moon blade")
	if !ok || item.Base != "Moon Blade" || !item.Equipped || item.InstanceID == 0 {
		t.Fatalf("case-insensitive lookup = %+v, %v", item, ok)
	}
	if matches := scriptFindItems("MOON BLADE"); len(matches) != 2 || matches[0].InstanceID == matches[1].InstanceID {
		t.Fatalf("all exact matches = %+v", matches)
	}
	if matches := scriptSearchItems("blade"); len(matches) != 2 {
		t.Fatalf("partial matches = %+v", matches)
	}
	equipped, ok := scriptEquipped("right-hand")
	if !ok || equipped.InstanceID != item.InstanceID || equipped.Slot != "right-hand" {
		t.Fatalf("equipped slot = %+v, %v", equipped, ok)
	}
}
