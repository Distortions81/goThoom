package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"golang.org/x/text/cases"
)

type InventoryItem struct {
	ID   uint16
	Name string
	// Base is the official base name without any custom/index suffix.
	Base string
	// Extra is the per-instance extra data if present:
	// - Template items: custom text after the index (e.g. "Sea What").
	// - Legacy items: custom text inside angle brackets (e.g. color name).
	Extra    string
	Equipped bool
	Index    int // display order (global)
	IDIndex  int // per-ID index used by server (0-based)
	Quantity int
}

// inventoryKey uniquely identifies an inventory item when storing custom names.
//
// Items that support templates are distinguished by a per-ID index provided by
// the server. Legacy items that do not expose an index use -1 so a name applies
// to all instances of the same ID.
type inventoryKey struct {
	ID      uint16
	IDIndex int16
}

var (
	inventoryMu    sync.RWMutex
	inventoryItems []InventoryItem
	inventoryNames = make(map[inventoryKey]string)
)

var invFoldCaser = cases.Fold()

const kItemFlagData = 0x0400

// normalizeInventoryName returns a canonical form of an item name for comparisons.
// It trims whitespace and performs case folding so items with minor name
// variations (e.g. capitalization differences) can be coalesced. Accents are
// preserved.
func normalizeInventoryName(name string) string {
	name = strings.TrimSpace(name)
	return invFoldCaser.String(name)
}

func resetInventory() {
	inventoryMu.Lock()
	inventoryItems = inventoryItems[:0]
	inventoryNames = make(map[inventoryKey]string)
	inventoryMu.Unlock()
	inventoryDirty = true
}

// rebuildInventoryIndices recalculates sequential display indices for all
// inventory items and rebuilds the inventoryNames map based on the current
// state. inventoryMu must be held by the caller.
func rebuildInventoryIndices() {
	inventoryNames = make(map[inventoryKey]string)
	for i := range inventoryItems {
		inventoryItems[i].Index = i
		// Persist only the per-instance extra (custom) text, not the full display name.
		if inventoryItems[i].Extra != "" {
			key := inventoryKey{ID: inventoryItems[i].ID, IDIndex: int16(inventoryItems[i].IDIndex)}
			if inventoryItems[i].IDIndex < 0 {
				key.IDIndex = -1
			}
			inventoryNames[key] = inventoryItems[i].Extra
		}
	}
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
		disp := fmt.Sprintf("%s <#%d>", name, idx+1)
		item := InventoryItem{ID: id, Name: disp, Base: name, Extra: "", Equipped: equip, Index: len(inventoryItems), IDIndex: idx, Quantity: 1}
		inventoryItems = append(inventoryItems, item)
	} else {
		// Legacy/non-template: coalesce by ID only when normalized names match.
		found := false
		normName := normalizeInventoryName(name)
		for i := range inventoryItems {
			if inventoryItems[i].ID == id && inventoryItems[i].IDIndex < 0 && normalizeInventoryName(inventoryItems[i].Name) == normName {
				inventoryItems[i].Quantity++
				if equip {
					inventoryItems[i].Equipped = true
				}
				found = true
				break
			}
		}
		if !found {
			item := InventoryItem{ID: id, Name: name, Base: name, Extra: "", Equipped: equip, Index: len(inventoryItems), IDIndex: -1, Quantity: 1}
			inventoryItems = append(inventoryItems, item)
		}
	}
	rebuildInventoryIndices()
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
		rebuildInventoryIndices()
	}
	inventoryMu.Unlock()
	inventoryDirty = true
}

func equipInventoryItem(id uint16, idx int, equip bool) {
	inventoryMu.Lock()
	// Find target by per-ID index when provided. Without an explicit index
	// choose an item by ID, preferring an already equipped instance when
	// unequipping.
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
			if inventoryItems[i].ID != id {
				continue
			}
			if !equip && inventoryItems[i].Equipped {
				target = i
				break
			}
			if target < 0 {
				target = i
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

// queueEquipCommand enqueues the server command to equip an item. The server
// automatically bumps clothing that occupies the same slot, so no explicit
// /unequip commands are sent here. The local inventory state is adjusted via
// equipInventoryItem to mirror the server's behavior. idx is the server-
// provided 0-based index for template items or -1 otherwise.
func queueEquipCommand(id uint16, idx int) {
	if idx >= 0 {
		enqueueCommand(fmt.Sprintf("/equip %d %d", id, idx+1))
	} else {
		enqueueCommand(fmt.Sprintf("/equip %d", id))
	}
	nextCommand()
}

// toggleInventoryEquipAt equips or unequips a specific item index. When idx is
// negative, the first matching item is targeted similar to the legacy
// behavior. The server is informed via pendingCommand and local inventory state
// is updated immediately.
func toggleInventoryEquipAt(id uint16, idx int) {
	items := getInventory()
	equip := true
	if idx >= 0 {
		for _, it := range items {
			if it.ID == id && it.IDIndex == idx {
				if it.Equipped {
					equip = false
				}
				break
			}
		}
	} else {
		for _, it := range items {
			if it.ID != id {
				continue
			}
			if it.Equipped {
				equip = false
				break
			}
			if idx < 0 {
				idx = it.IDIndex
			}
		}
	}
	if equip {
		queueEquipCommand(id, idx)
		equipInventoryItem(id, idx, true)
	} else {
		enqueueCommand(fmt.Sprintf("/unequip %d", id))
		nextCommand()
		equipInventoryItem(id, -1, false)
	}
}

func renameInventoryItem(id uint16, idx int, name string) {
	inventoryMu.Lock()
	if idx >= 0 {
		// Template items are addressed by a per-ID index. Update only the
		// matching instance so multiple containers of the same type can
		// retain distinct names.
		for i := range inventoryItems {
			if inventoryItems[i].ID == id && inventoryItems[i].IDIndex == idx {
				// Determine base (official) name without any suffix
				base := inventoryItems[i].Name
				if p := strings.Index(base, " <#"); p >= 0 {
					base = base[:p]
				}
				if base == "" && clImages != nil {
					if n := clImages.ItemName(uint32(id)); n != "" {
						base = n
					}
				}
				if base == "" {
					base = fmt.Sprintf("Item %d", id)
				}
				if name != "" {
					// Canonical: include colon for custom template names
					inventoryItems[i].Name = fmt.Sprintf("%s <#%d: %s>", base, idx+1, name)
					inventoryItems[i].Base = base
					inventoryItems[i].Extra = name
					inventoryNames[inventoryKey{ID: id, IDIndex: int16(idx)}] = name
				} else {
					inventoryItems[i].Name = fmt.Sprintf("%s <#%d>", base, idx+1)
					inventoryItems[i].Base = base
					inventoryItems[i].Extra = ""
				}
				break
			}
		}
	} else {
		// Legacy items without a template index: rename all matching IDs.
		if name != "" {
			inventoryNames[inventoryKey{ID: id, IDIndex: -1}] = name
		}
		for i := range inventoryItems {
			// Only update legacy instances; do not override template instances.
			if inventoryItems[i].ID == id && inventoryItems[i].IDIndex < 0 {
				// Compose canonical legacy name: Base <custom> when set, otherwise Base
				base := inventoryItems[i].Name
				if p := strings.Index(base, " <"); p >= 0 {
					base = base[:p]
				}
				if base == "" && clImages != nil {
					if n := clImages.ItemName(uint32(id)); n != "" {
						base = n
					}
				}
				if base == "" {
					if n, ok := defaultInventoryNames[id]; ok {
						base = n
					} else {
						base = fmt.Sprintf("Item %d", id)
					}
				}
				if name != "" {
					inventoryItems[i].Name = fmt.Sprintf("%s <%s>", base, name)
					inventoryItems[i].Base = base
					inventoryItems[i].Extra = name
				} else {
					inventoryItems[i].Name = base
					inventoryItems[i].Base = base
					inventoryItems[i].Extra = ""
				}
			}
		}
	}
	inventoryMu.Unlock()
	inventoryDirty = true
}

func getInventory() []InventoryItem {
	inventoryMu.RLock()
	defer inventoryMu.RUnlock()
	out := make([]InventoryItem, len(inventoryItems))
	copy(out, inventoryItems)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Equipped != out[j].Equipped {
			return out[i].Equipped && !out[j].Equipped
		}
		return out[i].Index < out[j].Index
	})
	return out
}

// inventoryItemByIndex returns the InventoryItem at the given index.
func inventoryItemByIndex(idx int) (InventoryItem, bool) {
	inventoryMu.RLock()
	defer inventoryMu.RUnlock()
	if idx < 0 || idx >= len(inventoryItems) {
		return InventoryItem{}, false
	}
	return inventoryItems[idx], true
}

func setFullInventory(ids []uint16, equipped []bool) {
	oldNames := make(map[inventoryKey]string)
	inventoryMu.RLock()
	for k, v := range inventoryNames {
		oldNames[k] = v
	}
	inventoryMu.RUnlock()

	type groupKey struct {
		id   uint16
		name string
	}

	grouped := make([]InventoryItem, 0, len(ids))
	groupPos := make(map[groupKey]int)
	tmplCounts := make(map[uint16]int)
	newNames := make(map[inventoryKey]string)

	for i, id := range ids {
		equip := i < len(equipped) && equipped[i]

		isTemplate := false
		if clImages != nil {
			if it, ok := clImages.Item(uint32(id)); ok {
				if it.Flags&kItemFlagData != 0 {
					isTemplate = true
				}
			}
		}

		var name string
		var idx int
		if isTemplate {
			idx = tmplCounts[id]
			tmplCounts[id] = idx + 1
			// Only use per-index custom for template items; do not fall back to legacy (-1).
			name = oldNames[inventoryKey{ID: id, IDIndex: int16(idx)}]
		} else {
			name = oldNames[inventoryKey{ID: id, IDIndex: -1}]
		}

		// Determine base (official) name
		base := ""
		if clImages != nil {
			if n := clImages.ItemName(uint32(id)); n != "" {
				base = n
			}
		}
		if base == "" {
			if n, ok := defaultInventoryNames[id]; ok {
				base = n
			} else {
				base = fmt.Sprintf("Item %d", id)
			}
		}

		// Compose canonical display name for the new list
		disp := base
		if isTemplate {
			if strings.TrimSpace(name) != "" {
				disp = fmt.Sprintf("%s <#%d: %s>", base, idx+1, name)
			} else {
				disp = fmt.Sprintf("%s <#%d>", base, idx+1)
			}
			item := InventoryItem{ID: id, Name: disp, Base: base, Extra: strings.TrimSpace(name), Equipped: equip, Index: len(grouped), IDIndex: idx, Quantity: 1}
			grouped = append(grouped, item)
			if name != "" {
				newNames[inventoryKey{ID: id, IDIndex: int16(idx)}] = name
			}
			continue
		}

		// Legacy items: If a custom exists and differs from base, append as "<custom>"
		if strings.TrimSpace(name) != "" && normalizeInventoryName(name) != normalizeInventoryName(base) {
			disp = fmt.Sprintf("%s <%s>", base, name)
		}

		gk := groupKey{id: id, name: normalizeInventoryName(disp)}
		if pos, ok := groupPos[gk]; ok {
			grouped[pos].Quantity++
			if equip {
				grouped[pos].Equipped = true
			}
			continue
		}

		legacyExtra := ""
		if strings.TrimSpace(name) != "" && normalizeInventoryName(name) != normalizeInventoryName(base) {
			legacyExtra = strings.TrimSpace(name)
		}
		item := InventoryItem{ID: id, Name: disp, Base: base, Extra: legacyExtra, Equipped: equip, Index: len(grouped), IDIndex: -1, Quantity: 1}
		grouped = append(grouped, item)
		groupPos[gk] = len(grouped) - 1
		if name != "" {
			newNames[inventoryKey{ID: id, IDIndex: -1}] = name
		}
	}

	inventoryMu.Lock()
	inventoryItems = grouped
	inventoryNames = newNames
	inventoryMu.Unlock()
	inventoryDirty = true
}
