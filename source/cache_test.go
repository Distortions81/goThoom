package main

import "testing"

func TestClearCachesRefreshesImageBackedLists(t *testing.T) {
	inventoryDirty = false
	playersDirty = false
	clearCaches()

	if !inventoryDirty {
		t.Fatal("cache clear did not request an inventory refresh")
	}
	if !playersDirty {
		t.Fatal("cache clear did not request a players-list refresh")
	}
}
