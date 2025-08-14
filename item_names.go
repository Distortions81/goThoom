package main

// defaultInventoryNames maps item IDs to fallback names. This can be
// extended with known items so inventories have meaningful labels before
// any rename commands are received.
var defaultInventoryNames = map[uint16]string{
	1001: "Short Sword",
	1002: "Long Sword",
	1003: "Dagger",
	1004: "Wooden Shield",
	1005: "Leather Armor",
	1006: "Chain Mail",
	1007: "Healing Potion",
	1008: "Mana Potion",
	1009: "Torch",
	1010: "Rope",
}
