package main

import (
	"fmt"
	"sync"
)

type inventoryItem struct {
	ID       uint16
	Name     string
	Equipped bool
}

var (
	inventoryMu    sync.RWMutex
	inventoryItems []inventoryItem
)

func resetInventory() {
	inventoryMu.Lock()
	inventoryItems = inventoryItems[:0]
	inventoryMu.Unlock()
}

func addInventoryItem(id uint16, name string, equip bool) {
	inventoryMu.Lock()
	inventoryItems = append(inventoryItems, inventoryItem{ID: id, Name: name, Equipped: equip})
	inventoryMu.Unlock()
}

func removeInventoryItem(id uint16) {
	inventoryMu.Lock()
	for i, it := range inventoryItems {
		if it.ID == id {
			inventoryItems = append(inventoryItems[:i], inventoryItems[i+1:]...)
			break
		}
	}
	inventoryMu.Unlock()
}

func equipInventoryItem(id uint16, equip bool) {
	inventoryMu.Lock()
	for i := range inventoryItems {
		if inventoryItems[i].ID == id {
			inventoryItems[i].Equipped = equip
			break
		}
	}
	inventoryMu.Unlock()
}

func renameInventoryItem(id uint16, name string) {
	inventoryMu.Lock()
	for i := range inventoryItems {
		if inventoryItems[i].ID == id {
			inventoryItems[i].Name = name
			break
		}
	}
	inventoryMu.Unlock()
}

func getInventory() []inventoryItem {
	inventoryMu.RLock()
	defer inventoryMu.RUnlock()
	out := make([]inventoryItem, len(inventoryItems))
	copy(out, inventoryItems)
	return out
}

func setFullInventory(ids []uint16, equipped []bool) {
	items := make([]inventoryItem, 0, len(ids))
	for i, id := range ids {
		name := ""
		if clImages != nil {
			name = clImages.ItemName(uint32(id))
		}
		if name == "" {
			name = fmt.Sprintf("Item %d", id)
		}
		equip := false
		if i < len(equipped) && equipped[i] {
			equip = true
		}
		items = append(items, inventoryItem{ID: id, Name: name, Equipped: equip})
	}
	inventoryMu.Lock()
	inventoryItems = items
	inventoryMu.Unlock()
}
