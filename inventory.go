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
	Quantity int
}

var (
	inventoryMu    sync.RWMutex
	inventoryItems []inventoryItem
	inventoryNames = make(map[uint16]string)
)

func resetInventory() {
	inventoryMu.Lock()
	inventoryItems = inventoryItems[:0]
	inventoryNames = make(map[uint16]string)
	inventoryMu.Unlock()
	inventoryDirty = true
}

func addInventoryItem(id uint16, idx int, name string, equip bool) {
	inventoryMu.Lock()
	inserted := false
	if idx >= 0 && idx < len(inventoryItems) && inventoryItems[idx].ID == id {
		inventoryItems[idx].Quantity++
	} else {
		found := false
		for i := range inventoryItems {
			if inventoryItems[i].ID == id {
				inventoryItems[i].Quantity++
				found = true
				break
			}
		}
		if !found {
			if idx < 0 || idx > len(inventoryItems) {
				idx = len(inventoryItems)
			}
			item := inventoryItem{ID: id, Name: name, Equipped: equip, Index: idx, Quantity: 1}
			inventoryItems = append(inventoryItems, inventoryItem{})
			copy(inventoryItems[idx+1:], inventoryItems[idx:])
			inventoryItems[idx] = item
			inserted = true
		}
	}
	if inserted {
		for i := range inventoryItems {
			inventoryItems[i].Index = i
		}
	}
	if name != "" {
		inventoryNames[id] = name
	}
	inventoryMu.Unlock()
	inventoryDirty = true
}

func removeInventoryItem(id uint16, idx int) {
	inventoryMu.Lock()
	removed := false
	if idx >= 0 && idx < len(inventoryItems) && inventoryItems[idx].ID == id {
		if inventoryItems[idx].Quantity > 1 {
			inventoryItems[idx].Quantity--
		} else {
			inventoryItems = append(inventoryItems[:idx], inventoryItems[idx+1:]...)
			removed = true
		}
	} else {
		for i, it := range inventoryItems {
			if it.ID == id {
				if it.Quantity > 1 {
					inventoryItems[i].Quantity--
				} else {
					inventoryItems = append(inventoryItems[:i], inventoryItems[i+1:]...)
					removed = true
				}
				break
			}
		}
	}
	if removed {
		for i := range inventoryItems {
			inventoryItems[i].Index = i
		}
	}
	inventoryMu.Unlock()
	inventoryDirty = true
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
	inventoryDirty = true
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
	if name != "" {
		inventoryNames[id] = name
	}
	inventoryMu.Unlock()
	inventoryDirty = true
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
	seen := make(map[uint16]int)
	inventoryMu.Lock()
	for i, id := range ids {
		if idx, ok := seen[id]; ok {
			items[idx].Quantity++
			if i < len(equipped) && equipped[i] {
				items[idx].Equipped = true
			}
			continue
		}
		name := inventoryNames[id]
		if name == "" {
			if n, ok := defaultInventoryNames[id]; ok {
				name = n
			} else {
				name = fmt.Sprintf("Item %d", id)
			}
			inventoryNames[id] = name
		}
		equip := false
		if i < len(equipped) && equipped[i] {
			equip = true
		}
		items = append(items, inventoryItem{ID: id, Name: name, Equipped: equip, Index: len(items), Quantity: 1})
		seen[id] = len(items) - 1
	}
	inventoryItems = items
	inventoryMu.Unlock()
	inventoryDirty = true
}
