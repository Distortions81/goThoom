package main

import "testing"

func TestNameTagLRUProtectsBorrowedAndRecentlyUsedEntries(t *testing.T) {
	clearSharedNameTagCache()
	t.Cleanup(clearSharedNameTagCache)
	keys := []nameTagKey{{Text: "old"}, {Text: "recent"}, {Text: "borrowed"}}
	for _, key := range keys {
		img := nameTagTargets.Acquire(40, 16, false)
		entry := &cachedNameTagImage{image: img, bytes: nameTagTargets.Bytes(img)}
		touchSharedNameTag(entry)
		sharedNameTagCache[key] = entry
		sharedNameTagBytes += entry.bytes
	}
	borrowed := borrowSharedNameTag(keys[2])
	defer releaseSharedNameTag(borrowed)
	// Make the borrowed entry oldest; it must still survive eviction.
	borrowed.lastUsed = 0
	borrowSharedNameTag(keys[1])
	releaseSharedNameTag(sharedNameTagCache[keys[1]])
	if !evictOldestSharedNameTag() || sharedNameTagCache[keys[0]] != nil {
		t.Fatal("LRU did not evict the oldest unborrowed tag")
	}
	if sharedNameTagCache[keys[1]] == nil || sharedNameTagCache[keys[2]] != borrowed {
		t.Fatal("eviction discarded a recently used or borrowed tag")
	}
	if len(sharedNameTagCache) != 2 || nameTagTargets.Stats().Active != 2 {
		t.Fatal("eviction did not return exactly one allocation")
	}
	clearSharedNameTagCacheFor(keys[2].Text)
	if !borrowed.retired || nameTagTargets.Bytes(borrowed.image) == 0 {
		t.Fatal("invalidating a borrowed tag prematurely recycled its image")
	}
}

func TestNameTagClearReturnsRetiredImageAfterLastBorrower(t *testing.T) {
	clearSharedNameTagCache()
	t.Cleanup(clearSharedNameTagCache)
	key := nameTagKey{Text: "held during clear"}
	img := nameTagTargets.Acquire(40, 16, false)
	entry := &cachedNameTagImage{image: img, bytes: nameTagTargets.Bytes(img)}
	sharedNameTagCache[key] = entry
	sharedNameTagBytes = entry.bytes
	first, second := borrowSharedNameTag(key), borrowSharedNameTag(key)
	clearSharedNameTagCache()
	if len(sharedNameTagCache) != 0 || sharedNameTagBytes != 0 || nameTagTargets.Bytes(img) == 0 {
		t.Fatal("clear must unlink the tag while preserving outstanding borrows")
	}
	releaseSharedNameTag(first)
	if nameTagTargets.Bytes(img) == 0 {
		t.Fatal("first release recycled an image that still had a borrower")
	}
	releaseSharedNameTag(second)
	if nameTagTargets.Bytes(img) != 0 || nameTagTargets.Stats().Active != 0 {
		t.Fatal("last release did not return retired storage")
	}
}
