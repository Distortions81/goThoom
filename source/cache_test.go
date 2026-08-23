package main

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestClearCachesRefreshesImageBackedLists(t *testing.T) {
	inventoryDirty = false
	playersDirty = false
	thoughtBubbleCompositeMask = ebiten.NewImage(8, 8)
	clearCaches()

	if !inventoryDirty {
		t.Fatal("cache clear did not request an inventory refresh")
	}
	if !playersDirty {
		t.Fatal("cache clear did not request a players-list refresh")
	}
	if thoughtBubbleCompositeMask != nil {
		t.Fatal("cache clear did not release the thought-bubble mask")
	}
}
