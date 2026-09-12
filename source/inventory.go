package main

import (
	"fmt"
	scriptapi "gt2"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/text/cases"
)

type InventoryItem = scriptapi.Item

// inventoryKey uniquely identifies an inventory item when storing custom names.
//
// Items that support templates are distinguished by a per-ID index provided by
// the server. Legacy items that do not expose an index use -1 so a name applies
// to all instances of the same ID.
type inventoryKey struct {
	ID      uint16
	IDIndex int16
}

type inventoryState struct {
	mu               sync.RWMutex
	items            []InventoryItem
	names            map[inventoryKey]string
	instanceSequence atomic.Uint64
	revision         atomic.Uint64
}

func newInventoryState() *inventoryState {
	return &inventoryState{names: make(map[inventoryKey]string)}
}

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
	primarySession.inventory.reset()
	inventoryDirty = true
}

func (s *inventoryState) reset() {
	s.mu.Lock()
	s.items = s.items[:0]
	s.names = make(map[inventoryKey]string)
	s.instanceSequence.Store(0)
	s.mu.Unlock()
	s.revision.Add(1)
}

// rebuildIndicesLocked recalculates sequential display indices for all
// inventory items and rebuilds the custom-name map based on the current state.
// s.mu must be held by the caller.
func (s *inventoryState) rebuildIndicesLocked() {
	s.names = make(map[inventoryKey]string)
	for i := range s.items {
		s.items[i].Index = i
		// Persist only the per-instance extra (custom) text, not the full display name.
		if s.items[i].Extra != "" {
			key := inventoryKey{ID: s.items[i].ID, IDIndex: int16(s.items[i].IDIndex)}
			if s.items[i].IDIndex < 0 {
				key.IDIndex = -1
			}
			s.names[key] = s.items[i].Extra
		}
	}
}

func addInventoryItem(id uint16, idx int, name string, equip bool) {
	primarySession.inventory.add(id, idx, name, equip)
	inventoryDirty = true
}

func (s *inventoryState) add(id uint16, idx int, name string, equip bool) {
	s.mu.Lock()
	target := -1
	if idx >= 0 {
		// Template item with explicit per-ID index; insert a new entry and renumber
		// existing items of the same ID whose IDIndex >= idx.
		for i := range s.items {
			if s.items[i].ID == id && s.items[i].IDIndex >= idx {
				s.items[i].IDIndex++
			}
		}
		// Append as a distinct instance; keep display order by placing at end
		disp := fmt.Sprintf("%s <#%d>", name, idx+1)
		target = len(s.items)
		item := InventoryItem{InstanceID: s.instanceSequence.Add(1), ID: id, Name: disp, Base: name, Extra: "", Equipped: equip, Index: target, IDIndex: idx, Quantity: 1}
		s.items = append(s.items, item)
	} else {
		// Legacy/non-template: coalesce by ID only when normalized names match.
		found := false
		normName := normalizeInventoryName(name)
		for i := range s.items {
			if s.items[i].ID == id && s.items[i].IDIndex < 0 && normalizeInventoryName(s.items[i].Name) == normName {
				target = i
				s.items[i].Quantity++
				if equip {
					s.items[i].Equipped = true
				}
				found = true
				break
			}
		}
		if !found {
			target = len(s.items)
			item := InventoryItem{InstanceID: s.instanceSequence.Add(1), ID: id, Name: name, Base: name, Extra: "", Equipped: equip, Index: target, IDIndex: -1, Quantity: 1}
			s.items = append(s.items, item)
		}
	}
	s.rebuildIndicesLocked()
	// If this item was equipped, clear any other equipped items occupying the
	// same slot (e.g., hands, head). Mirrors BumpItemsFromSlot in the reference client.
	if equip && clImages != nil {
		slot := clImages.ItemSlot(uint32(id))
		for i := range s.items {
			if i != target && s.items[i].Equipped {
				if clImages.ItemSlot(uint32(s.items[i].ID)) == slot {
					s.items[i].Equipped = false
				}
			}
		}
	}
	s.mu.Unlock()
	s.revision.Add(1)
}

func removeInventoryItem(id uint16, idx int) {
	primarySession.inventory.remove(id, idx)
	inventoryDirty = true
}

func (s *inventoryState) remove(id uint16, idx int) {
	s.mu.Lock()
	removed := false
	if idx >= 0 {
		// Remove by per-ID index
		pos := -1
		for i, it := range s.items {
			if it.ID == id && it.IDIndex == idx {
				pos = i
				break
			}
		}
		if pos >= 0 {
			// Remove and renumber subsequent per-ID indices
			s.items = append(s.items[:pos], s.items[pos+1:]...)
			for i := range s.items {
				if s.items[i].ID == id && s.items[i].IDIndex > idx {
					s.items[i].IDIndex--
				}
			}
			removed = true
		}
	} else {
		for i, it := range s.items {
			if it.ID == id && it.IDIndex < 0 {
				if it.Quantity > 1 {
					s.items[i].Quantity--
				} else {
					s.items = append(s.items[:i], s.items[i+1:]...)
					removed = true
				}
				break
			}
		}
	}
	if removed {
		s.rebuildIndicesLocked()
	}
	s.mu.Unlock()
	s.revision.Add(1)
}

func equipInventoryItem(id uint16, idx int, equip bool) {
	primarySession.inventory.equip(id, idx, equip)
	inventoryDirty = true
}

func (s *inventoryState) equip(id uint16, idx int, equip bool) {
	s.mu.Lock()
	// Find target by per-ID index when provided. Without an explicit index
	// choose an item by ID, preferring an already equipped instance when
	// unequipping.
	target := -1
	if idx >= 0 {
		for i := range s.items {
			if s.items[i].ID == id && s.items[i].IDIndex == idx {
				target = i
				break
			}
		}
	} else {
		for i := range s.items {
			if s.items[i].ID != id {
				continue
			}
			if !equip && s.items[i].Equipped {
				target = i
				break
			}
			if target < 0 {
				target = i
			}
		}
	}
	if target >= 0 {
		s.items[target].Equipped = equip
	}
	// When equipping, make sure other items in the same slot are unequipped.
	if equip && clImages != nil {
		slot := clImages.ItemSlot(uint32(id))
		for i := range s.items {
			if i == target {
				continue
			}
			if s.items[i].Equipped && clImages.ItemSlot(uint32(s.items[i].ID)) == slot {
				s.items[i].Equipped = false
			}
		}
	}
	s.mu.Unlock()
	s.revision.Add(1)
}

// queueEquipCommand enqueues the server command to equip an item. The server
// automatically bumps clothing that occupies the same slot, so no explicit
// /unequip commands are sent here. idx is the server-provided 0-based index for
// template items or -1 otherwise. Local state changes only when the server's
// inventory update arrives.
func queueEquipCommand(id uint16, idx int) {
	enqueueCommand(formatEquipCommand(id, idx))
	nextCommand()
}

func formatEquipCommand(id uint16, idx int) string {
	if idx >= 0 {
		return fmt.Sprintf("/equip %d %d", id, idx+1)
	}
	return fmt.Sprintf("/equip %d", id)
}

// toggleInventoryEquipAt equips or unequips a specific item index. When idx is
// negative, the first matching item is targeted similar to the legacy
// behavior. The server is informed through the session command stream; local
// inventory state remains authoritative to the server response.
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
	} else {
		enqueueCommand(fmt.Sprintf("/unequip %d", id))
		nextCommand()
	}
}

func renameInventoryItem(id uint16, idx int, name string) {
	primarySession.inventory.rename(id, idx, name)
	inventoryDirty = true
}

func (s *inventoryState) rename(id uint16, idx int, name string) {
	s.mu.Lock()
	if idx >= 0 {
		// Template items are addressed by a per-ID index. Update only the
		// matching instance so multiple containers of the same type can
		// retain distinct names.
		for i := range s.items {
			if s.items[i].ID == id && s.items[i].IDIndex == idx {
				// Determine base (official) name without any suffix
				base := s.items[i].Name
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
					s.items[i].Name = fmt.Sprintf("%s <#%d: %s>", base, idx+1, name)
					s.items[i].Base = base
					s.items[i].Extra = name
					s.names[inventoryKey{ID: id, IDIndex: int16(idx)}] = name
				} else {
					s.items[i].Name = fmt.Sprintf("%s <#%d>", base, idx+1)
					s.items[i].Base = base
					s.items[i].Extra = ""
				}
				break
			}
		}
	} else {
		// Legacy items without a template index: rename all matching IDs.
		if name != "" {
			s.names[inventoryKey{ID: id, IDIndex: -1}] = name
		}
		for i := range s.items {
			// Only update legacy instances; do not override template instances.
			if s.items[i].ID == id && s.items[i].IDIndex < 0 {
				// Compose canonical legacy name: Base <custom> when set, otherwise Base
				base := s.items[i].Name
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
					s.items[i].Name = fmt.Sprintf("%s <%s>", base, name)
					s.items[i].Base = base
					s.items[i].Extra = name
				} else {
					s.items[i].Name = base
					s.items[i].Base = base
					s.items[i].Extra = ""
				}
			}
		}
	}
	s.mu.Unlock()
	s.revision.Add(1)
}

func getInventory() []InventoryItem {
	return primarySession.inventory.snapshot()
}

func (s *inventoryState) snapshot() []InventoryItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]InventoryItem, len(s.items))
	copy(out, s.items)
	if clImages != nil {
		for index := range out {
			slot := clImages.ItemSlot(uint32(out[index].ID))
			if slot >= kItemSlotFirstReal && slot <= kItemSlotLastReal {
				out[index].Slot = scriptItemSlotName(slot)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Equipped != out[j].Equipped {
			return out[i].Equipped && !out[j].Equipped
		}
		return out[i].Index < out[j].Index
	})
	return out
}

func (s *inventoryState) completionNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.items))
	for _, item := range s.items {
		if item.Name != "" {
			names = append(names, item.Name)
		}
	}
	return names
}

func getInventoryCompletionNames() []string {
	return primarySession.inventory.completionNames()
}

func scriptItemSlotName(slot int) string {
	names := [...]string{
		kItemSlotForehead: "forehead", kItemSlotNeck: "neck", kItemSlotShoulder: "shoulder",
		kItemSlotArms: "arms", kItemSlotGloves: "gloves", kItemSlotFinger: "finger",
		kItemSlotCoat: "coat", kItemSlotCloak: "cloak", kItemSlotTorso: "torso",
		kItemSlotWaist: "waist", kItemSlotLegs: "legs", kItemSlotFeet: "feet",
		kItemSlotRightHand: "right-hand", kItemSlotLeftHand: "left-hand",
		kItemSlotBothHands: "both-hands", kItemSlotHead: "head",
	}
	if slot < 0 || slot >= len(names) {
		return ""
	}
	return names[slot]
}

// inventoryItemByIndex returns the InventoryItem at the given index.
func inventoryItemByIndex(idx int) (InventoryItem, bool) {
	return primarySession.inventory.itemByIndex(idx)
}

func (s *inventoryState) itemByIndex(idx int) (InventoryItem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if idx < 0 || idx >= len(s.items) {
		return InventoryItem{}, false
	}
	return s.items[idx], true
}

func setFullInventory(ids []uint16, equipped []bool) {
	primarySession.inventory.setFull(ids, equipped)
	inventoryDirty = true
}

func (s *inventoryState) setFull(ids []uint16, equipped []bool) {
	type groupKey struct {
		id   uint16
		name string
	}
	oldNames := make(map[inventoryKey]string)
	oldTemplateIDs := make(map[inventoryKey]uint64)
	oldGroupIDs := make(map[groupKey]uint64)
	s.mu.RLock()
	for k, v := range s.names {
		oldNames[k] = v
	}
	for _, item := range s.items {
		if item.IDIndex >= 0 {
			oldTemplateIDs[inventoryKey{ID: item.ID, IDIndex: int16(item.IDIndex)}] = item.InstanceID
		} else {
			oldGroupIDs[groupKey{id: item.ID, name: normalizeInventoryName(item.Name)}] = item.InstanceID
		}
	}
	s.mu.RUnlock()

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
			instanceID := oldTemplateIDs[inventoryKey{ID: id, IDIndex: int16(idx)}]
			if instanceID == 0 {
				instanceID = s.instanceSequence.Add(1)
			}
			item := InventoryItem{InstanceID: instanceID, ID: id, Name: disp, Base: base, Extra: strings.TrimSpace(name), Equipped: equip, Index: len(grouped), IDIndex: idx, Quantity: 1}
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
		instanceID := oldGroupIDs[gk]
		if instanceID == 0 {
			instanceID = s.instanceSequence.Add(1)
		}
		item := InventoryItem{InstanceID: instanceID, ID: id, Name: disp, Base: base, Extra: legacyExtra, Equipped: equip, Index: len(grouped), IDIndex: -1, Quantity: 1}
		grouped = append(grouped, item)
		groupPos[gk] = len(grouped) - 1
		if name != "" {
			newNames[inventoryKey{ID: id, IDIndex: -1}] = name
		}
	}

	s.mu.Lock()
	s.items = grouped
	s.names = newNames
	s.mu.Unlock()
	s.revision.Add(1)
}
