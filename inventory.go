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
	Index    int // display order (global)
	IDIndex  int // per-ID index used by server (0-based)
	Quantity int
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
	if idx >= 0 {
		// Template item with explicit per-ID index; insert a new entry and renumber
		// existing items of the same ID whose IDIndex >= idx.
		for i := range inventoryItems {
			if inventoryItems[i].ID == id && inventoryItems[i].IDIndex >= idx {
				inventoryItems[i].IDIndex++
			}
		}
		// Append as a distinct instance; keep display order by placing at end
		item := inventoryItem{ID: id, Name: name, Equipped: equip, Index: len(inventoryItems), IDIndex: idx, Quantity: 1}
		inventoryItems = append(inventoryItems, item)
	} else {
		// Legacy/non-template: coalesce by ID and bump quantity
		found := false
		for i := range inventoryItems {
			if inventoryItems[i].ID == id && inventoryItems[i].IDIndex < 0 {
				inventoryItems[i].Quantity++
				if equip {
					inventoryItems[i].Equipped = true
				}
				found = true
				break
			}
		}
		if !found {
			item := inventoryItem{ID: id, Name: name, Equipped: equip, Index: len(inventoryItems), IDIndex: -1, Quantity: 1}
			inventoryItems = append(inventoryItems, item)
		}
	}
	inventoryNames = make(map[inventoryKey]string)
	for i := range inventoryItems {
		inventoryItems[i].Index = i
		if inventoryItems[i].Name != "" {
			inventoryNames[inventoryKey{ID: inventoryItems[i].ID, Index: uint16(i)}] = inventoryItems[i].Name
		}
	}
	// If this item was equipped, clear any other equipped items occupying the
	// same slot (e.g., hands, head). Mirrors BumpItemsFromSlot in the reference client.
	if equip && clImages != nil {
		slot := clImages.ItemSlot(uint32(id))
		for i := range inventoryItems {
			if inventoryItems[i].Equipped && (inventoryItems[i].ID != id || i != idx) {
				if clImages.ItemSlot(uint32(inventoryItems[i].ID)) == slot {
					inventoryItems[i].Equipped = false
				}
			}
		}
	}
	inventoryMu.Unlock()
	inventoryDirty = true
}

func removeInventoryItem(id uint16, idx int) {
	inventoryMu.Lock()
	removed := false
	if idx >= 0 {
		// Remove by per-ID index
		pos := -1
		for i, it := range inventoryItems {
			if it.ID == id && it.IDIndex == idx {
				pos = i
				break
			}
		}
		if pos >= 0 {
			// Remove and renumber subsequent per-ID indices
			inventoryItems = append(inventoryItems[:pos], inventoryItems[pos+1:]...)
			for i := range inventoryItems {
				if inventoryItems[i].ID == id && inventoryItems[i].IDIndex > idx {
					inventoryItems[i].IDIndex--
				}
			}
			removed = true
		}
	} else {
		for i, it := range inventoryItems {
			if it.ID == id && it.IDIndex < 0 {
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
	// Find target by per-ID index when provided; otherwise first by ID.
	target := -1
	if idx >= 0 {
		for i := range inventoryItems {
			if inventoryItems[i].ID == id && inventoryItems[i].IDIndex == idx {
				target = i
				break
			}
		}
	} else {
		for i := range inventoryItems {
			if inventoryItems[i].ID == id {
				target = i
				break
			}
		}
	}
	if target >= 0 {
		inventoryItems[target].Equipped = equip
	}
	// When equipping, make sure other items in the same slot are unequipped.
	if equip && clImages != nil {
		slot := clImages.ItemSlot(uint32(id))
		for i := range inventoryItems {
			if i == target {
				continue
			}
			if inventoryItems[i].Equipped && clImages.ItemSlot(uint32(inventoryItems[i].ID)) == slot {
				inventoryItems[i].Equipped = false
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
	seen := make(map[uint16]int)
	inventoryMu.Lock()
	newNames := make(map[inventoryKey]string)
	for i, id := range ids {
		key := inventoryKey{ID: id, Index: uint16(i)}
		name := inventoryNames[key]
		if name == "" {
			// Prefer name from CL_Images ClientItem metadata when available.
			if clImages != nil {
				if n := clImages.ItemName(uint32(id)); n != "" {
					name = n
				}
			}
			if name == "" {
				if n, ok := defaultInventoryNames[id]; ok {
					name = n
				} else {
					name = fmt.Sprintf("Item %d", id)
				}
			}
		}
		newNames[key] = name
		equip := false
		if i < len(equipped) && equipped[i] {
			equip = true
		}
		// Assign per-ID index sequentially
		idIdx := seen[id]
		items = append(items, inventoryItem{ID: id, Name: name, Equipped: equip, Index: len(items), IDIndex: idIdx, Quantity: 1})
		seen[id] = idIdx + 1
	}
	inventoryItems = items
	inventoryNames = newNames
	inventoryMu.Unlock()
	inventoryDirty = true
}
