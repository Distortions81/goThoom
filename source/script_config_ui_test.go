//go:build integration

package main

import (
	"sync"
	"testing"

	"gothoom/eui"
)

func TestScriptConfigWindowUsesCurrentValuesAndCallbacks(t *testing.T) {
	initFont()
	origDataDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = origDataDir })

	scriptMu = sync.RWMutex{}
	scriptDisplayNames = map[string]string{"plug": "Plug"}
	scriptAuthors = map[string]string{"plug": "Test"}
	scriptConfigMu = sync.RWMutex{}
	scriptConfigEntries = map[string][]scriptConfigEntry{}
	scriptStoreMu = sync.Mutex{}
	scriptStores = map[string]*scriptStore{}
	scriptsList = nil
	resetInventory()
	addInventoryItem(1, 0, "Sword", false)

	changed := true
	scriptAddConfig("plug", "Enabled", "bool", true, func(v bool) { changed = v })
	scriptAddConfig("plug", "Count", "int", 42)
	scriptAddConfig("plug", "Greeting", "string", "hello")
	scriptAddConfig("plug", "Item", "item", "Sword")

	openscriptConfigWindow("plug")
	if scriptConfigWin == nil || len(scriptConfigWin.Contents) != 1 {
		t.Fatal("configuration window was not created")
	}
	t.Cleanup(func() {
		if scriptConfigWin != nil {
			scriptConfigWin.Close()
			scriptConfigWin = nil
			scriptConfigOwner = ""
		}
	})
	root := scriptConfigWin.Contents[0]
	if len(root.Contents) != 4 {
		t.Fatalf("config row count = %d, want 4", len(root.Contents))
	}

	checkbox := root.Contents[0].Contents[1]
	if !checkbox.Checked {
		t.Fatal("checkbox did not use current value")
	}
	checkbox.Handler.Emit(eui.UIEvent{Item: checkbox, Type: eui.EventCheckboxChanged, Checked: false})
	if changed {
		t.Fatal("checkbox change callback did not receive false")
	}

	slider := root.Contents[1].Contents[1]
	if slider.Value != 42 || !slider.IntOnly {
		t.Fatalf("integer slider value/int mode = %v/%v", slider.Value, slider.IntOnly)
	}
	input := root.Contents[2].Contents[1]
	if input.Text != "hello" {
		t.Fatalf("text input value = %q", input.Text)
	}
	dropdown := root.Contents[3].Contents[1]
	if len(dropdown.Options) == 0 || dropdown.Options[dropdown.Selected] != "Sword" {
		t.Fatalf("item selector value/options = %d/%v", dropdown.Selected, dropdown.Options)
	}
}
