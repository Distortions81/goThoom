package main

import (
	"fmt"
	"sort"
	"sync"
)

type inventoryItem struct {
	ID       uint16
	Name     string
	Equipped bool
	Index    int
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

func addInventoryItem(id uint16, idx int, name string, equip bool) {
	inventoryMu.Lock()
	defer inventoryMu.Unlock()
	if idx < 0 || idx > len(inventoryItems) {
		idx = len(inventoryItems)
	}
	item := inventoryItem{ID: id, Name: name, Equipped: equip, Index: idx}
	inventoryItems = append(inventoryItems, inventoryItem{})
	copy(inventoryItems[idx+1:], inventoryItems[idx:])
	inventoryItems[idx] = item
	for i := range inventoryItems {
		inventoryItems[i].Index = i
	}
}

func removeInventoryItem(id uint16, idx int) {
	inventoryMu.Lock()
	defer inventoryMu.Unlock()
	if idx >= 0 && idx < len(inventoryItems) && inventoryItems[idx].ID == id {
		inventoryItems = append(inventoryItems[:idx], inventoryItems[idx+1:]...)
	} else {
		for i, it := range inventoryItems {
			if it.ID == id {
				inventoryItems = append(inventoryItems[:i], inventoryItems[i+1:]...)
				break
			}
		}
	}
	for i := range inventoryItems {
		inventoryItems[i].Index = i
	}
}

func equipInventoryItem(id uint16, idx int, equip bool) {
	inventoryMu.Lock()
	if idx >= 0 && idx < len(inventoryItems) && inventoryItems[idx].ID == id {
		inventoryItems[idx].Equipped = equip
	} else {
		for i := range inventoryItems {
			if inventoryItems[i].ID == id {
				inventoryItems[i].Equipped = equip
				break
			}
		}
	}
	inventoryMu.Unlock()
}

func renameInventoryItem(id uint16, idx int, name string) {
	inventoryMu.Lock()
	if idx >= 0 && idx < len(inventoryItems) && inventoryItems[idx].ID == id {
		inventoryItems[idx].Name = name
	} else {
		for i := range inventoryItems {
			if inventoryItems[i].ID == id {
				inventoryItems[i].Name = name
				break
			}
		}
	}
	inventoryMu.Unlock()
}

func getInventory() []inventoryItem {
	inventoryMu.RLock()
	defer inventoryMu.RUnlock()
	out := make([]inventoryItem, len(inventoryItems))
	copy(out, inventoryItems)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Equipped != out[j].Equipped {
			return out[i].Equipped && !out[j].Equipped
		}
		return out[i].Index < out[j].Index
	})
	return out
}

func setFullInventory(ids []uint16, equipped []bool) {
	items := make([]inventoryItem, 0, len(ids))
	for i, id := range ids {
		name := fmt.Sprintf("Item %d", id)
		equip := false
		if i < len(equipped) && equipped[i] {
			equip = true
		}
		items = append(items, inventoryItem{ID: id, Name: name, Equipped: equip, Index: i})
	}
	inventoryMu.Lock()
	inventoryItems = items
	inventoryMu.Unlock()
}
