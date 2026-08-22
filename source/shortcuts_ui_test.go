//go:build integration
// +build integration

package main

import (
	"sync"
	"testing"
)

// Test that the macros window lists registered macros.
func TestShortcutsWindowListsShortcuts(t *testing.T) {
	// Reset state and ensure cleanup after the test.
	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	scriptDisplayNames = map[string]string{}
	scriptCategories = map[string]string{}
	scriptSubCategories = map[string]string{}
	shortcutsWin = nil
	shortcutsList = nil
	t.Cleanup(func() {
		shortcutMu = sync.RWMutex{}
		shortcutMaps = map[string]map[string]string{}
		scriptDisplayNames = map[string]string{}
		scriptCategories = map[string]string{}
		scriptSubCategories = map[string]string{}
		shortcutsWin = nil
		shortcutsList = nil
	})

	makeShortcutsWindow()
	if shortcutsList == nil {
		t.Fatalf("shortcuts window not initialized")
	}
	if len(shortcutsList.Contents) != 0 {
		t.Fatalf("expected empty shortcuts list")
	}

	scriptAddShortcut("tester", "yy", "/yell ")
	if len(shortcutsList.Contents) != 2 {
		t.Fatalf("items not added to list: %d", len(shortcutsList.Contents))
	}
	if got := shortcutsList.Contents[0].Text; got != "tester:" {
		t.Fatalf("unexpected script text: %q", got)
	}
	if got := shortcutsList.Contents[1].Text; got != "  yy = /yell" {
		t.Fatalf("unexpected shortcut text: %q", got)
	}
}

// Test that removing macros refreshes the window and clears the list.
func TestscriptRemoveShortcutsRefresh(t *testing.T) {
	// Reset state and ensure cleanup after the test.
	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	scriptDisplayNames = map[string]string{}
	scriptCategories = map[string]string{}
	scriptSubCategories = map[string]string{}
	shortcutsWin = nil
	shortcutsList = nil
	t.Cleanup(func() {
		shortcutMu = sync.RWMutex{}
		shortcutMaps = map[string]map[string]string{}
		scriptDisplayNames = map[string]string{}
		scriptCategories = map[string]string{}
		scriptSubCategories = map[string]string{}
		shortcutsWin = nil
		shortcutsList = nil
	})

	makeShortcutsWindow()
	if shortcutsList == nil {
		t.Fatalf("shortcuts window not initialized")
	}

	scriptAddShortcut("tester", "yy", "/yell ")
	if len(shortcutsList.Contents) != 2 {
		t.Fatalf("items not added to list: %d", len(shortcutsList.Contents))
	}

	// Clear dirty flag so we can detect refresh.
	shortcutsWin.Dirty = false

	scriptRemoveShortcuts("tester")
	if len(shortcutsList.Contents) != 0 {
		t.Fatalf("shortcuts list not cleared: %d", len(shortcutsList.Contents))
	}
	if !shortcutsWin.Dirty {
		t.Fatalf("shortcuts window not refreshed")
	}
}

// Test that removing a user shortcut refreshes the window and clears the entry.
func TestRemoveUserShortcutRefresh(t *testing.T) {
	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	scriptDisplayNames = map[string]string{}
	scriptCategories = map[string]string{}
	scriptSubCategories = map[string]string{}
	shortcutsWin = nil
	shortcutsList = nil
	t.Cleanup(func() {
		shortcutMu = sync.RWMutex{}
		shortcutMaps = map[string]map[string]string{}
		scriptDisplayNames = map[string]string{}
		scriptCategories = map[string]string{}
		scriptSubCategories = map[string]string{}
		shortcutsWin = nil
		shortcutsList = nil
	})

	makeShortcutsWindow()
	if shortcutsList == nil {
		t.Fatalf("shortcuts window not initialized")
	}

	addUserShortcut("yy", "/yell ")
	if len(shortcutsList.Contents) != 2 {
		t.Fatalf("items not added to list: %d", len(shortcutsList.Contents))
	}

	shortcutsWin.Dirty = false
	removeShortcut("user", "yy")
	if len(shortcutsList.Contents) != 0 {
		t.Fatalf("shortcuts list not cleared: %d", len(shortcutsList.Contents))
	}
	if !shortcutsWin.Dirty {
		t.Fatalf("shortcuts window not refreshed")
	}
}

// Test that opening the macros window after macros have been added lists them correctly.
func TestShortcutsWindowLoadsExistingShortcuts(t *testing.T) {
	// Reset state and ensure cleanup after the test.
	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	scriptDisplayNames = map[string]string{}
	scriptCategories = map[string]string{}
	scriptSubCategories = map[string]string{}
	shortcutsWin = nil
	shortcutsList = nil
	t.Cleanup(func() {
		shortcutMu = sync.RWMutex{}
		shortcutMaps = map[string]map[string]string{}
		scriptDisplayNames = map[string]string{}
		scriptCategories = map[string]string{}
		scriptSubCategories = map[string]string{}
		shortcutsWin = nil
		shortcutsList = nil
	})

	// Add shortcuts before creating the window to mimic scripts registering at startup.
	scriptAddShortcut("tester", "yy", "/yell ")

	// Now create the window; it should populate with existing macros.
	makeShortcutsWindow()
	if shortcutsList == nil {
		t.Fatalf("shortcuts window not initialized")
	}
	if len(shortcutsList.Contents) != 2 {
		t.Fatalf("items not added to list: %d", len(shortcutsList.Contents))
	}
	if got := shortcutsList.Contents[0].Text; got != "tester:" {
		t.Fatalf("unexpected script text: %q", got)
	}
	if got := shortcutsList.Contents[1].Text; got != "  yy = /yell" {
		t.Fatalf("unexpected shortcut text: %q", got)
	}
}
