package main

import (
	"strconv"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestTopRowDigitHotkeysUsePortableNames(t *testing.T) {
	if got := scriptKeyName(ebiten.KeyDigit1, nil, nil, false, false); got != "1" {
		t.Fatalf("top-row digit name = %q, want 1", got)
	}
	if !sameCombo("Ctrl-Digit1", "Ctrl-1") {
		t.Fatal("legacy Ebitengine digit name does not match Ctrl-1")
	}
}

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
		if !hotkey.BuiltIn || hotkey.Combo != wantCombo || len(hotkey.Commands) != 1 || hotkey.Commands[0].Command != wantCommand {
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

func TestDefaultSessionTabCycleHotkeys(t *testing.T) {
	list, added := addDefaultSessionTabCycleHotkeys(nil)
	if !added || len(list) != 2 {
		t.Fatalf("default cycle hotkeys = %d, added %v", len(list), added)
	}
	want := map[string]string{
		"Ctrl-Tab":       "/tab next",
		"Ctrl-Shift-Tab": "/tab previous",
	}
	for _, hotkey := range list {
		if !hotkey.BuiltIn || len(hotkey.Commands) != 1 || hotkey.Commands[0].Command != want[hotkey.Combo] {
			t.Fatalf("cycle hotkey = %+v", hotkey)
		}
	}
}

func TestRetiredSessionTabCycleHotkeysAreRemoved(t *testing.T) {
	list := []Hotkey{
		{Name: "Previous Session Tab (alternate)", Combo: "Alt-Comma", Commands: []HotkeyCommand{{Command: "/tab previous"}}},
		{Name: "Renamed", Combo: "Ctrl-Period", Commands: []HotkeyCommand{{Command: "/tab next"}}, BuiltIn: true},
		{Name: "My tab shortcut", Combo: "Ctrl-Comma", Commands: []HotkeyCommand{{Command: "/tab next"}}},
	}
	filtered, removed := removeRetiredSessionTabHotkeys(list)
	if !removed || len(filtered) != 1 || filtered[0].Name != "My tab shortcut" {
		t.Fatalf("retired cycle migration = %+v, removed=%v", filtered, removed)
	}
}

func TestDefaultSessionTabCycleHotkeysPreserveOccupiedCombo(t *testing.T) {
	existing := Hotkey{Name: "Existing", Combo: "Control-Tab", Commands: []HotkeyCommand{{Command: "/other"}}}
	list, _ := addDefaultSessionTabCycleHotkeys([]Hotkey{existing})
	for _, hotkey := range list[1:] {
		if sameCombo(hotkey.Combo, existing.Combo) {
			t.Fatalf("occupied combo was duplicated: %+v", hotkey)
		}
	}
}

func TestSessionTabTooltipFindsCurrentShortcut(t *testing.T) {
	oldHotkeys := hotkeys
	t.Cleanup(func() { hotkeys = oldHotkeys })
	hotkeys = []Hotkey{
		{Combo: "Alt-2", Commands: []HotkeyCommand{{Command: "/tab 2"}}},
		{Combo: "Ctrl-2", Disabled: true, Commands: []HotkeyCommand{{Command: "/tab 2"}}},
	}
	if got := hotkeyComboForCommand("/TAB 2"); got != "Alt-2" {
		t.Fatalf("tab tooltip combo = %q, want Alt-2", got)
	}
}

func TestDefaultClientHotkeysCoverClientActions(t *testing.T) {
	list, added := addDefaultClientHotkeys(nil)
	if !added || len(list) != 11 {
		t.Fatalf("default client hotkeys = %d, added %v", len(list), added)
	}
	wantCounts := map[string]int{
		"/fullscreen": 1,
		"/palette":    1,
		"/move left":  2,
		"/move right": 2,
		"/move up":    2,
		"/move down":  2,
		"/move run":   1,
	}
	gotCounts := map[string]int{}
	for _, hotkey := range list {
		if !hotkey.BuiltIn || len(hotkey.Commands) != 1 {
			t.Fatalf("client hotkey = %+v", hotkey)
		}
		gotCounts[hotkey.Commands[0].Command]++
	}
	for command, want := range wantCounts {
		if got := gotCounts[command]; got != want {
			t.Errorf("%s bindings = %d, want %d", command, got, want)
		}
	}
}

func TestKnownBuiltInHotkeyMigrationDoesNotClaimCustomBindings(t *testing.T) {
	list := []Hotkey{
		{Name: "Walk Left", Combo: "Alt-J", Commands: []HotkeyCommand{{Command: "/move left"}}},
		{Name: "My Walk Left", Combo: "J", Commands: []HotkeyCommand{{Command: "/move left"}}},
		{Name: "Walk Left", Combo: "K", Commands: []HotkeyCommand{{Command: "/wave"}}},
	}
	if !markKnownBuiltInHotkeys(list) {
		t.Fatal("known built-in hotkey was not migrated")
	}
	if !list[0].BuiltIn {
		t.Fatal("renamed key for a known built-in action lost its identity")
	}
	if list[1].BuiltIn || list[2].BuiltIn {
		t.Fatalf("custom hotkeys were claimed as built-ins: %+v", list)
	}
}

func TestDefaultClientHotkeysPreserveCustomCommandAndOccupiedKey(t *testing.T) {
	existing := []Hotkey{
		{Combo: "Alt-F", Commands: []HotkeyCommand{{Command: "/fullscreen"}}},
		{Combo: "A", Commands: []HotkeyCommand{{Command: "/wave"}}},
	}
	list, _ := addDefaultClientHotkeys(existing)
	fullscreen := 0
	walkLeftA := 0
	for _, hotkey := range list {
		for _, command := range hotkey.Commands {
			if command.Command == "/fullscreen" {
				fullscreen++
			}
			if command.Command == "/move left" && sameCombo(hotkey.Combo, "A") {
				walkLeftA++
			}
		}
	}
	if fullscreen != 1 || walkLeftA != 0 {
		t.Fatalf("custom bindings changed: fullscreen=%d walk-left-A=%d", fullscreen, walkLeftA)
	}
}

func TestCompileClientHotkey(t *testing.T) {
	tests := []struct {
		combo     string
		key       ebiten.Key
		modifiers uint8
	}{
		{"Ctrl-Shift-P", ebiten.KeyP, clientHotkeyModCtrl | clientHotkeyModShift},
		{"1", ebiten.KeyDigit1, 0},
		{"Shift", ebiten.KeyShift, 0},
		{"ArrowLeft", ebiten.KeyArrowLeft, 0},
	}
	for _, test := range tests {
		got, ok := compileClientHotkey(test.combo)
		if !ok || got.key != test.key || got.modifiers != test.modifiers {
			t.Errorf("compile %q = %+v, %v", test.combo, got, ok)
		}
	}
	if _, ok := compileClientHotkey("Ctrl-WheelUp"); ok {
		t.Fatal("wheel binding compiled as a held keyboard action")
	}
}

func TestRebuildClientHotkeysUsesEditableGlobalBindings(t *testing.T) {
	oldBindings := clientHotkeyBindings.Load()
	t.Cleanup(func() { clientHotkeyBindings.Store(oldBindings) })
	list := []Hotkey{
		{Combo: "Alt-J", Commands: []HotkeyCommand{{Command: "/move left"}}},
		{Combo: "K", Disabled: true, Commands: []HotkeyCommand{{Command: "/move right"}}},
		{Combo: "L", Script: "example", Commands: []HotkeyCommand{{Command: "/move down"}}},
	}
	rebuildClientHotkeyBindings(list)
	compiled := clientHotkeyBindings.Load()
	if compiled == nil || len(compiled[clientHotkeyMoveLeft]) != 1 {
		t.Fatalf("compiled movement bindings = %+v", compiled)
	}
	left := compiled[clientHotkeyMoveLeft][0]
	if left.key != ebiten.KeyJ || left.modifiers != clientHotkeyModAlt {
		t.Fatalf("compiled custom binding = %+v", left)
	}
	if len(compiled[clientHotkeyMoveRight]) != 0 || len(compiled[clientHotkeyMoveDown]) != 0 {
		t.Fatal("disabled or script binding entered client movement cache")
	}
}
