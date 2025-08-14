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

type inventoryKey struct {
	ID    uint16
	Index uint16
}

var (
	inventoryMu    sync.RWMutex
	inventoryItems []inventoryItem
	inventoryNames = make(map[inventoryKey]string)
)

func resetInventory() {
	inventoryMu.Lock()
	inventoryItems = inventoryItems[:0]
	inventoryNames = make(map[inventoryKey]string)
	inventoryMu.Unlock()
	inventoryDirty = true
}

func addInventoryItem(id uint16, idx int, name string, equip bool) {
	inventoryMu.Lock()
	if idx < 0 || idx > len(inventoryItems) {
		idx = len(inventoryItems)
	}
	item := inventoryItem{ID: id, Name: name, Equipped: equip, Index: idx}
	inventoryItems = append(inventoryItems, inventoryItem{})
	copy(inventoryItems[idx+1:], inventoryItems[idx:])
	inventoryItems[idx] = item
	inventoryNames = make(map[inventoryKey]string)
	for i := range inventoryItems {
		inventoryItems[i].Index = i
		if inventoryItems[i].Name != "" {
			inventoryNames[inventoryKey{ID: inventoryItems[i].ID, Index: uint16(i)}] = inventoryItems[i].Name
		}
	}
	inventoryMu.Unlock()
	inventoryDirty = true
}

func removeInventoryItem(id uint16, idx int) {
	inventoryMu.Lock()
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
	index := -1
	if idx >= 0 && idx < len(inventoryItems) && inventoryItems[idx].ID == id {
		inventoryItems[idx].Name = name
		index = idx
	} else {
		for i := range inventoryItems {
			if inventoryItems[i].ID == id {
				inventoryItems[i].Name = name
				index = i
				break
			}
		}
	}
	if name != "" && index >= 0 {
		inventoryNames[inventoryKey{ID: id, Index: uint16(index)}] = name
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
	inventoryMu.Lock()
	newNames := make(map[inventoryKey]string)
	for i, id := range ids {
		key := inventoryKey{ID: id, Index: uint16(i)}
		name := inventoryNames[key]
		if name == "" {
			if n, ok := defaultInventoryNames[id]; ok {
				name = n
			} else {
				name = fmt.Sprintf("Item %d", id)
			}
		}
		newNames[key] = name
		equip := false
		if i < len(equipped) && equipped[i] {
			equip = true
		}
		items = append(items, inventoryItem{ID: id, Name: name, Equipped: equip, Index: i})
	}
	inventoryItems = items
	inventoryNames = newNames
	inventoryMu.Unlock()
	inventoryDirty = true
}
