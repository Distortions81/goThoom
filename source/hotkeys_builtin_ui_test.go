package main

import "testing"

func TestBuiltInHotkeyHasNoDeleteAndCanBeDisabledInEditor(t *testing.T) {
	initFont()
	originalHotkeys := hotkeys
	originalHotkeysWin := hotkeysWin
	originalHotkeysList := hotkeysList
	originalEditWin := hotkeyEditWin
	originalEnabledCB := hotkeyEnabledCB
	originalDataDir := dataDirPath
	hotkeys = []Hotkey{
		{Name: "Walk Left", Combo: "A", Commands: []HotkeyCommand{{Command: "/move left"}}, BuiltIn: true},
		{Name: "Wave", Combo: "Ctrl-W", Commands: []HotkeyCommand{{Command: "/wave"}}},
	}
	hotkeysWin = nil
	hotkeysList = nil
	hotkeyEditWin = nil
	hotkeyEnabledCB = nil
	dataDirPath = t.TempDir()
	t.Cleanup(func() {
		if hotkeyEditWin != nil && hotkeyEditWin != originalEditWin {
			hotkeyEditWin.Close()
		}
		if hotkeysWin != nil && hotkeysWin != originalHotkeysWin {
			hotkeysWin.RemoveWindow()
		}
		hotkeys = originalHotkeys
		hotkeysWin = originalHotkeysWin
		hotkeysList = originalHotkeysList
		hotkeyEditWin = originalEditWin
		hotkeyEnabledCB = originalEnabledCB
		dataDirPath = originalDataDir
	})

	makeHotkeysWindow()
	if len(hotkeysList.Contents) != 2 {
		t.Fatalf("hotkey rows = %d, want 2", len(hotkeysList.Contents))
	}
	if got := len(hotkeysList.Contents[0].Contents); got != 1 {
		t.Fatalf("built-in row controls = %d, want edit only", got)
	}
	if got := len(hotkeysList.Contents[1].Contents); got != 2 {
		t.Fatalf("custom row controls = %d, want edit and delete", got)
	}

	openHotkeyEditor(0)
	if hotkeyEnabledCB == nil || hotkeyEnabledCB.Text != "Enabled" || !hotkeyEnabledCB.Checked {
		t.Fatal("built-in editor does not offer its enabled state")
	}
	hotkeyEnabledCB.Checked = false
	finishHotkeyEdit(true)
	if !hotkeys[0].BuiltIn || !hotkeys[0].Disabled {
		t.Fatalf("disabled built-in identity was not preserved: %+v", hotkeys[0])
	}
}
