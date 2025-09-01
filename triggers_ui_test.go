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
	pluginTriggers = map[string][]triggerHandler{}
	pluginDisplayNames = map[string]string{}
	pluginCategories = map[string]string{}
	pluginSubCategories = map[string]string{}
	triggersWin = nil
	triggersList = nil
	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	t.Cleanup(func() {
		triggerHandlersMu = sync.RWMutex{}
		pluginTriggers = map[string][]triggerHandler{}
		pluginDisplayNames = map[string]string{}
		pluginCategories = map[string]string{}
		pluginSubCategories = map[string]string{}
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

	pluginRegisterTriggers("tester", "", []string{"hi"}, func() {})
	if len(triggersList.Contents) != 2 {
		t.Fatalf("items not added to list: %d", len(triggersList.Contents))
	}
	if got := triggersList.Contents[0].Text; got != "tester:" {
		t.Fatalf("unexpected plugin text: %q", got)
	}
	if got := triggersList.Contents[1].Text; got != "  hi" {
		t.Fatalf("unexpected trigger text: %q", got)
	}
}

// Test that disabling a plugin refreshes and clears triggers.
func TestDisablePluginRefreshesTriggers(t *testing.T) {
	triggerHandlersMu = sync.RWMutex{}
	pluginTriggers = map[string][]triggerHandler{}
	pluginDisplayNames = map[string]string{}
	pluginCategories = map[string]string{}
	pluginSubCategories = map[string]string{}
	triggersWin = nil
	triggersList = nil
	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	pluginMu = sync.RWMutex{}
	pluginDisabled = map[string]bool{}
	pluginInvalid = map[string]bool{}
	pluginEnabledFor = map[string]pluginScope{}
	pluginTerminators = map[string]func(){}
	t.Cleanup(func() {
		triggerHandlersMu = sync.RWMutex{}
		pluginTriggers = map[string][]triggerHandler{}
		pluginDisplayNames = map[string]string{}
		pluginCategories = map[string]string{}
		pluginSubCategories = map[string]string{}
		triggersWin = nil
		triggersList = nil
		shortcutMu = sync.RWMutex{}
		shortcutMaps = map[string]map[string]string{}
		pluginMu = sync.RWMutex{}
		pluginDisabled = map[string]bool{}
		pluginInvalid = map[string]bool{}
		pluginEnabledFor = map[string]pluginScope{}
		pluginTerminators = map[string]func(){}
	})

	makeTriggersWindow()
	pluginRegisterTriggers("plug", "", []string{"yo"}, func() {})
	if len(triggersList.Contents) != 2 {
		t.Fatalf("items not added to list: %d", len(triggersList.Contents))
	}
	triggersWin.Dirty = false
	disablePlugin("plug", "test")
	if len(triggersList.Contents) != 0 {
		t.Fatalf("triggers list not cleared: %d", len(triggersList.Contents))
	}
	if !triggersWin.Dirty {
		t.Fatalf("triggers window not refreshed")
	}
}
