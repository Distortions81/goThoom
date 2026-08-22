//go:build integration
// +build integration

package main

import (
	"sync"
	"testing"
)

// Test that the triggers window lists registered triggers.
func TestTriggersWindowListsTriggers(t *testing.T) {
	triggerHandlersMu = sync.RWMutex{}
	scriptTriggers = map[string][]triggerHandler{}
	scriptDisplayNames = map[string]string{}
	scriptCategories = map[string]string{}
	scriptSubCategories = map[string]string{}
	triggersWin = nil
	triggersList = nil
	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	t.Cleanup(func() {
		triggerHandlersMu = sync.RWMutex{}
		scriptTriggers = map[string][]triggerHandler{}
		scriptDisplayNames = map[string]string{}
		scriptCategories = map[string]string{}
		scriptSubCategories = map[string]string{}
		triggersWin = nil
		triggersList = nil
		shortcutMu = sync.RWMutex{}
		shortcutMaps = map[string]map[string]string{}
	})

	makeTriggersWindow()
	if triggersList == nil {
		t.Fatalf("triggers window not initialized")
	}
	if len(triggersList.Contents) != 0 {
		t.Fatalf("expected empty triggers list")
	}

	scriptRegisterTriggers("tester", "", []string{"hi"}, func() {})
	if len(triggersList.Contents) != 2 {
		t.Fatalf("items not added to list: %d", len(triggersList.Contents))
	}
	if got := triggersList.Contents[0].Text; got != "tester:" {
		t.Fatalf("unexpected script text: %q", got)
	}
	if got := triggersList.Contents[1].Text; got != "  hi" {
		t.Fatalf("unexpected trigger text: %q", got)
	}
}

// Test that disabling a script refreshes and clears triggers.
func TestDisablescriptRefreshesTriggers(t *testing.T) {
	triggerHandlersMu = sync.RWMutex{}
	scriptTriggers = map[string][]triggerHandler{}
	scriptDisplayNames = map[string]string{}
	scriptCategories = map[string]string{}
	scriptSubCategories = map[string]string{}
	triggersWin = nil
	triggersList = nil
	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	scriptMu = sync.RWMutex{}
	scriptDisabled = map[string]bool{}
	scriptInvalid = map[string]bool{}
	scriptEnabledFor = map[string]scriptScope{}
	scriptTerminators = map[string]func(){}
	t.Cleanup(func() {
		triggerHandlersMu = sync.RWMutex{}
		scriptTriggers = map[string][]triggerHandler{}
		scriptDisplayNames = map[string]string{}
		scriptCategories = map[string]string{}
		scriptSubCategories = map[string]string{}
		triggersWin = nil
		triggersList = nil
		shortcutMu = sync.RWMutex{}
		shortcutMaps = map[string]map[string]string{}
		scriptMu = sync.RWMutex{}
		scriptDisabled = map[string]bool{}
		scriptInvalid = map[string]bool{}
		scriptEnabledFor = map[string]scriptScope{}
		scriptTerminators = map[string]func(){}
	})

	makeTriggersWindow()
	scriptRegisterTriggers("plug", "", []string{"yo"}, func() {})
	if len(triggersList.Contents) != 2 {
		t.Fatalf("items not added to list: %d", len(triggersList.Contents))
	}
	triggersWin.Dirty = false
	disablescript("plug", "test")
	if len(triggersList.Contents) != 0 {
		t.Fatalf("triggers list not cleared: %d", len(triggersList.Contents))
	}
	if !triggersWin.Dirty {
		t.Fatalf("triggers window not refreshed")
	}
}
