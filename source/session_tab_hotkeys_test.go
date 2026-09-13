package main

import (
	"strconv"
	"testing"
)

func TestDefaultSessionTabHotkeysCoverTenTabs(t *testing.T) {
	list, added := addDefaultSessionTabHotkeys(nil)
	if !added || len(list) != maxSessions {
		t.Fatalf("default tab hotkeys = %d, added %v", len(list), added)
	}
	for index, hotkey := range list {
		position := index + 1
		key := position
		if position == 10 {
			key = 0
		}
		wantCombo := "Ctrl-" + strconv.Itoa(key)
		wantCommand := "/tab " + strconv.Itoa(position)
		if hotkey.Combo != wantCombo || len(hotkey.Commands) != 1 || hotkey.Commands[0].Command != wantCommand {
			t.Fatalf("tab %d hotkey = %+v, want %s", position, hotkey, wantCombo)
		}
	}
}

func TestDefaultSessionTabHotkeysPreserveCustomizedBinding(t *testing.T) {
	existing := Hotkey{Combo: "Alt-4", Commands: []HotkeyCommand{{Command: "/tab 4"}}}
	list, _ := addDefaultSessionTabHotkeys([]Hotkey{existing})
	count := 0
	for _, hotkey := range list {
		if len(hotkey.Commands) == 1 && hotkey.Commands[0].Command == "/tab 4" {
			count++
			if hotkey.Combo != "Alt-4" {
				t.Fatalf("custom tab binding changed to %q", hotkey.Combo)
			}
		}
	}
	if count != 1 {
		t.Fatalf("found %d tab-4 bindings, want one", count)
	}
}
