package main

import (
	"path/filepath"
	"testing"
)

func TestCollectKeybindingEntriesShowsScriptsAndLegacyMacros(t *testing.T) {
	dir := t.TempDir()
	program := parseLegacyMacroSources([]legacyMacroSource{
		{
			Name: "Default",
			Path: filepath.Join(dir, "Default"),
			Text: "control-f1 message \"one\"\n\"say\" \"not a key\"\nshift-click message \"two\"\n",
		},
	})
	registered := []Hotkey{
		{Combo: "F12", Name: "User hotkey"},
		{Combo: "F3", Script: "armor"},
		{Combo: "Shift-F4", Script: "dance", Disabled: true},
		{Combo: "F5", Script: "stopped"},
	}

	entries := collectKeybindingEntries(registered, program, func(owner string) string {
		return map[string]string{"armor": "Iron Armor", "dance": "Dance"}[owner]
	}, func(hotkey Hotkey) bool {
		return !hotkey.Disabled && hotkey.Script != "stopped"
	})
	if len(entries) != 3 {
		t.Fatalf("entries = %#v, want three active script or macro bindings", entries)
	}
	if entries[0] != (keybindingEntry{Binding: "control-f1", Owner: "Default", Kind: "macro", Line: 1}) {
		t.Fatalf("first entry = %#v", entries[0])
	}
	if entries[1] != (keybindingEntry{Binding: "F3", Owner: "Iron Armor", Kind: "script"}) {
		t.Fatalf("second entry = %#v", entries[1])
	}
	if entries[2] != (keybindingEntry{Binding: "shift-click", Owner: "Default", Kind: "macro", Line: 3}) {
		t.Fatalf("third entry = %#v", entries[2])
	}
}

func TestCollectKeybindingEntriesOmitsShadowedAndUnreachableLegacyBindings(t *testing.T) {
	dir := t.TempDir()
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "Default",
		Path: filepath.Join(dir, "Default"),
		Text: "f1 message \"first\"\nF1 message \"shadowed\"\nundo message \"unreachable\"\nclick6 message \"unreachable\"\n",
	}})

	entries := collectKeybindingEntries(nil, program, nil, nil)
	if len(entries) != 1 || entries[0].Binding != "f1" || entries[0].Line != 1 {
		t.Fatalf("entries = %#v, want only the active first f1 binding", entries)
	}
}

func TestCollectKeybindingEntriesHonorsLegacyOverride(t *testing.T) {
	dir := t.TempDir()
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "Default",
		Path: filepath.Join(dir, "Default"),
		Text: "control-f1 message \"legacy wins\"\ncontrol-f2\n{\n$no_override\nmessage \"both run\"\n}\n",
	}})
	registered := []Hotkey{
		{Combo: "Ctrl-F1", Script: "one"},
		{Combo: "Ctrl-F2", Script: "two"},
	}

	entries := collectKeybindingEntries(registered, program, nil, func(Hotkey) bool { return true })
	for _, entry := range entries {
		if entry.Kind == "script" && entry.Binding == "Ctrl-F1" {
			t.Fatal("script Ctrl-F1 was listed even though the legacy macro consumes it")
		}
	}
	foundF2Script := false
	for _, entry := range entries {
		if entry.Kind == "script" && entry.Binding == "Ctrl-F2" {
			foundF2Script = true
		}
	}
	if !foundF2Script {
		t.Fatal("script Ctrl-F2 was omitted even though the legacy macro allows default handling")
	}
}
