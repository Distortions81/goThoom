//go:build integration

package main

import (
	"sync"
	"testing"

	"gothoom/eui"
	scriptapi "gt"
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
	registerScriptConfigTestOption(t, "plug", "enabled", "Enabled", "", scriptapi.ScopeGlobal, "bool", true, func(v bool) { changed = v }, nil, nil, 0, 0, 0)
	registerScriptConfigTestOption(t, "plug", "count", "Count", "", scriptapi.ScopeGlobal, "int", 42, nil, nil, nil, 0, 100, 1)
	registerScriptConfigTestOption(t, "plug", "ratio", "Ratio", "", scriptapi.ScopeGlobal, "float", 0.5, nil, nil, nil, 0, 1, 0.1)
	registerScriptConfigTestOption(t, "plug", "greeting", "Greeting", "", scriptapi.ScopeGlobal, "text", "hello", nil, nil, nil, 0, 0, 0)
	registerScriptConfigTestOption(t, "plug", "mode", "Mode", "", scriptapi.ScopeGlobal, "choice", "one", nil, nil, []string{"one", "two"}, 0, 0, 0)
	registerScriptConfigTestOption(t, "plug", "binding", "Binding", "", scriptapi.ScopeGlobal, "key", "Ctrl-K", nil, nil, nil, 0, 0, 0)
	registerScriptConfigTestOption(t, "plug", "item", "Item", "", scriptapi.ScopeGlobal, "item", "Sword", nil, nil, nil, 0, 0, 0)

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
	if len(root.Contents) != 7 {
		t.Fatalf("config row count = %d, want 7", len(root.Contents))
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
	decimal := root.Contents[2].Contents[1]
	if decimal.Value != 0.5 || decimal.IntOnly {
		t.Fatalf("decimal slider value/int mode = %v/%v", decimal.Value, decimal.IntOnly)
	}
	input := root.Contents[3].Contents[1]
	if input.Text != "hello" {
		t.Fatalf("text input value = %q", input.Text)
	}
	choice := root.Contents[4].Contents[1]
	if len(choice.Options) != 2 || choice.Options[choice.Selected] != "one" {
		t.Fatalf("choice value/options = %d/%v", choice.Selected, choice.Options)
	}
	keyInput := root.Contents[5].Contents[1]
	if keyInput.Text != "Ctrl-K" {
		t.Fatalf("key binding input = %q", keyInput.Text)
	}
	dropdown := root.Contents[6].Contents[1]
	if len(dropdown.Options) == 0 || dropdown.Options[dropdown.Selected] != "Sword" {
		t.Fatalf("item selector value/options = %d/%v", dropdown.Selected, dropdown.Options)
	}
}
