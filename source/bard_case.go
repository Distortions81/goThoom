package main

import (
	"fmt"
	"strings"
	"time"

	scriptapi "gt2"
)

func bardItemInstrument(item InventoryItem) int {
	name := item.Base
	if name == "" {
		name = item.Name
	}
	return bardInstrumentIndex(name)
}

func bardInstrumentCase(session *Session) (InventoryItem, bool) {
	if session != nil {
		for _, item := range session.inventory.snapshot() {
			name := item.Base
			if name == "" {
				name = item.Name
			}
			if strings.EqualFold(strings.TrimSpace(name), "instrument case") {
				return item, true
			}
		}
	}
	return InventoryItem{}, false
}

func bardCanPrepareInstrument(session *Session, index int) bool {
	_, owned := bardOwnedInstrument(session, index)
	_, hasCase := bardInstrumentCase(session)
	return owned || hasCase
}

func bardInstrumentCount(items []InventoryItem, index int) int {
	count := 0
	for _, item := range items {
		if bardItemInstrument(item) == index {
			count += item.Quantity
		}
	}
	return count
}

func bardInventorySlots(items []InventoryItem) int {
	count := 0
	for _, item := range items {
		count += item.Quantity
	}
	return count
}

func bardCurrentItem(items []InventoryItem, original InventoryItem) (InventoryItem, bool) {
	for _, item := range items {
		if item.ID == original.ID && item.InstanceID == original.InstanceID {
			return item, true
		}
	}
	return InventoryItem{}, false
}

// Case operations are paced by server inventory updates, never by guessed delays.
// target is -1 when storing all carried instruments. Otherwise only enough
// instruments to make room for the requested one are stored.
type bardCaseOperation struct {
	session          *Session
	generation       uint64
	caseItem         InventoryItem
	previousLeft     InventoryItem
	target           int
	stores           []int
	tickets          []CommandTicket
	pending          CommandTicket
	stage            string
	deadline         time.Time
	count            int
	activeInstrument int
	done             bool
}

func startBardCaseOperation(session *Session, target int) (*bardCaseOperation, error) {
	if session == nil || !session.transport.connected() {
		return nil, fmt.Errorf("Connect a character to use the instrument case.")
	}
	caseItem, ok := bardInstrumentCase(session)
	if !ok {
		return nil, fmt.Errorf("Carry an instrument case to retrieve or store instruments.")
	}
	items := session.inventory.snapshot()
	op := &bardCaseOperation{session: session, generation: bardConnectionGeneration(session), caseItem: caseItem, target: target}
	for _, item := range items {
		if item.Equipped && item.Slot == "left-hand" && item.InstanceID != caseItem.InstanceID {
			op.previousLeft = item
		}
		index := bardItemInstrument(item)
		if index >= 0 && target < 0 {
			for n := 0; n < item.Quantity; n++ {
				op.stores = append(op.stores, index)
			}
		}
	}
	if target >= 0 && bardInventorySlots(items) >= inventoryMaxSlots {
		// Prefer an instrument that is not currently equipped.
		for _, equipped := range []bool{false, true} {
			for _, item := range items {
				if index := bardItemInstrument(item); index >= 0 && index != target && item.Equipped == equipped {
					op.stores = []int{index}
					break
				}
			}
			if len(op.stores) > 0 {
				break
			}
		}
		if len(op.stores) == 0 {
			return nil, fmt.Errorf("Inventory is full. Free a slot before taking out %s.", classicInstrumentNames[target])
		}
	}
	if target < 0 && len(op.stores) == 0 {
		return nil, fmt.Errorf("No carried instruments to put away.")
	}
	op.send("equip case", formatEquipCommand(caseItem.ID, caseItem.IDIndex), time.Now())
	return op, nil
}

func queueBardSessionCommands(session *Session, generation uint64, commands []string) []CommandTicket {
	transport := session.transport
	transport.mu.RLock()
	defer transport.mu.RUnlock()
	if transport.generation != generation || transport.status != sessionConnected || transport.tcp == nil {
		return nil
	}
	return queueBardCommands(session, commands)
}

func (op *bardCaseOperation) send(stage, command string, now time.Time) {
	op.stage, op.deadline = stage, now.Add(10*time.Second)
	tickets := queueBardSessionCommands(op.session, op.generation, []string{command})
	op.tickets = append(op.tickets, tickets...)
	op.pending = CommandTicket{}
	if len(tickets) > 0 {
		op.pending = tickets[0]
	}
}

func (op *bardCaseOperation) cancel() {
	if op != nil {
		for _, ticket := range op.tickets {
			ticket.Cancel()
		}
	}
}

func (op *bardCaseOperation) update(now time.Time) (err error) {
	defer func() {
		if err != nil {
			op.cancel()
		}
	}()
	if !op.session.transport.connectedGeneration(op.generation) {
		return fmt.Errorf("Instrument case operation ended: character disconnected.")
	}
	if op.done {
		return nil
	}
	items := op.session.inventory.snapshot()
	caseItem, exists := bardCurrentItem(items, op.caseItem)
	if !exists {
		return fmt.Errorf("The instrument case is no longer in inventory.")
	}
	if now.After(op.deadline) {
		switch op.stage {
		case "store":
			return fmt.Errorf("%s did not return to the instrument case. Check the game's response.", classicInstrumentNames[op.activeInstrument])
		case "retrieve":
			return fmt.Errorf("The instrument case did not produce %s. Check the game's response.", classicInstrumentNames[op.target])
		default:
			return fmt.Errorf("Equipment was not confirmed while using the instrument case. Check the game's response.")
		}
	}
	state := op.pending.Status().State
	if state == scriptapi.CommandCancelled || state == scriptapi.CommandRejected {
		return fmt.Errorf("Instrument case command was canceled.")
	}
	if state != scriptapi.CommandSent || !op.session.commands.idle() {
		return nil
	}
	switch op.stage {
	case "equip case":
		if !caseItem.Equipped {
			return nil
		}
	case "store":
		if bardInstrumentCount(items, op.activeInstrument) >= op.count {
			return nil
		}
	case "retrieve":
		if _, ok := bardOwnedInstrument(op.session, op.target); !ok {
			return nil
		}
		op.restore(items, caseItem, now)
		return nil
	case "restore":
		if op.previousLeft.InstanceID != 0 {
			previous, ok := bardCurrentItem(items, op.previousLeft)
			if ok && !previous.Equipped {
				return nil
			}
		} else if !op.caseItem.Equipped && caseItem.Equipped {
			return nil
		}
		op.done = true
		return nil
	}
	if !caseItem.Equipped {
		return fmt.Errorf("The instrument case was unequipped. Try again.")
	}
	for len(op.stores) > 0 {
		index := op.stores[0]
		op.stores = op.stores[1:]
		count := bardInstrumentCount(items, index)
		if count == 0 {
			continue
		}
		op.activeInstrument, op.count = index, count
		op.send("store", "/useitem instrument case /add "+classicInstrumentNames[index], now)
		return nil
	}
	if op.target >= 0 {
		if bardInventorySlots(items) >= inventoryMaxSlots {
			return fmt.Errorf("Inventory is still full. Free a slot and try again.")
		}
		op.send("retrieve", "/useitem instrument case /remove "+classicInstrumentNames[op.target], now)
	} else {
		op.restore(items, caseItem, now)
	}
	return nil
}

func (op *bardCaseOperation) restore(items []InventoryItem, caseItem InventoryItem, now time.Time) {
	if previous, ok := bardCurrentItem(items, op.previousLeft); ok && caseItem.Equipped {
		op.send("restore", formatEquipCommand(previous.ID, previous.IDIndex), now)
	} else if !op.caseItem.Equipped && caseItem.Equipped {
		op.previousLeft = InventoryItem{}
		op.send("restore", fmt.Sprintf("/unequip %d", caseItem.ID), now)
	} else {
		op.done = true
	}
}
