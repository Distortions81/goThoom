package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font/gofont/goregular"
	"gothoom/eui"
)

// Test that closing the hotkey editor clears the reference and allows reopening.
func TestOpenHotkeyEditorReopenAfterClose(t *testing.T) {
	hotkeyEditWin = nil

	// Open and close with OK
	openHotkeyEditor(-1)
	if hotkeyEditWin == nil {
		t.Fatalf("editor not opened")
	}
	finishHotkeyEdit(true)
	if hotkeyEditWin != nil {
		t.Fatalf("editor not cleared after OK")
	}

	// Reopen and close with Cancel
	openHotkeyEditor(-1)
	if hotkeyEditWin == nil {
		t.Fatalf("editor not reopened after OK")
	}
	finishHotkeyEdit(false)
	if hotkeyEditWin != nil {
		t.Fatalf("editor not cleared after Cancel")
	}

	// Reopen and close via 'X'
	openHotkeyEditor(-1)
	if hotkeyEditWin == nil {
		t.Fatalf("editor not reopened after Cancel")
	}
	hotkeyEditWin.Close()
	if hotkeyEditWin != nil {
		t.Fatalf("editor not cleared after Close")
	}

	// Final reopen to ensure no leftovers
	openHotkeyEditor(-1)
	if hotkeyEditWin == nil {
		t.Fatalf("editor not reopened after Close")
	}
	hotkeyEditWin.Close()
}

// Test that entering a command in the hotkey editor saves correctly.
func TestHotkeyCommandInput(t *testing.T) {
	hotkeys = nil
	dir := t.TempDir()
	origDir := dataDirPath
	dataDirPath = dir
	defer func() { dataDirPath = origDir }()

	openHotkeyEditor(-1)
	if len(hotkeyCmdInputs) == 0 {
		t.Fatalf("command input not initialized")
	}

	hotkeyComboText.Text = "Ctrl-A"
	hotkeyNameInput.Text = "Test"
	hotkeyCmdInputs[0].Text = "say hi"
	finishHotkeyEdit(true)

	if len(hotkeys) != 1 {
		t.Fatalf("hotkey not saved")
	}
	if hotkeys[0].Combo != "Ctrl-A" || hotkeys[0].Name != "Test" || len(hotkeys[0].Commands) != 1 || hotkeys[0].Commands[0].Command != "say hi" {
		t.Fatalf("unexpected hotkey data: %+v", hotkeys[0])
	}
}

// Test that a hotkey with an empty command saves correctly.
func TestHotkeyEmptyCommandSaved(t *testing.T) {
	hotkeys = nil
	openHotkeyEditor(-1)
	hotkeyComboText.Text = "Ctrl-E"
	finishHotkeyEdit(true)

	if len(hotkeys) != 1 {
		t.Fatalf("hotkey not saved")
	}
	if len(hotkeys[0].Commands) != 0 {
		t.Fatalf("expected no commands, got: %+v", hotkeys[0].Commands)
	}
	if hotkeyEditWin != nil {
		hotkeyEditWin.Close()
	}
}

// Test that attempting to bind a duplicate combo results in an error and no save.
func TestHotkeyDuplicateComboError(t *testing.T) {
	if err := eui.EnsureFontSource(goregular.TTF); err != nil {
		t.Fatalf("ensure font: %v", err)
	}
	hotkeys = []Hotkey{{Combo: "Ctrl-A"}}

	openHotkeyEditor(-1)
	hotkeyComboText.Text = "Ctrl-A"
	finishHotkeyEdit(true)

	if len(hotkeys) != 1 {
		t.Fatalf("duplicate hotkey saved")
	}
	if hotkeyEditWin == nil {
		t.Fatalf("editor closed despite duplicate combo")
	}
	hotkeyEditWin.Close()
}

// Test that editing a hotkey with no name still saves changes.
func TestHotkeyEditWithoutName(t *testing.T) {
	hotkeys = []Hotkey{{Combo: "Ctrl-A", Commands: []HotkeyCommand{{Command: "say hi"}}}}
	dir := t.TempDir()
	origDir := dataDirPath
	dataDirPath = dir
	defer func() { dataDirPath = origDir }()

	openHotkeyEditor(0)
	hotkeyCmdInputs[0].Text = "say bye"
	finishHotkeyEdit(true)

	if len(hotkeys) != 1 || hotkeys[0].Commands[0].Command != "say bye" {
		t.Fatalf("hotkey not updated without name: %+v", hotkeys)
	}
}

// Test that a hotkey without a name still saves and refreshes.
func TestHotkeySavedWithoutName(t *testing.T) {
	hotkeys = nil
	openHotkeyEditor(-1)
	hotkeyComboText.Text = "Ctrl-C"
	hotkeyCmdInputs[0].Text = "say hi"
	finishHotkeyEdit(true)
	if len(hotkeys) != 1 || hotkeys[0].Name != "" {
		t.Fatalf("hotkey not saved or name unexpectedly set: %+v", hotkeys)
	}
	if hotkeyEditWin != nil {
		hotkeyEditWin.Close()
	}
}

// Test that adding a hotkey without a name updates the window list.
func TestHotkeyListUpdatesForNamelessHotkey(t *testing.T) {
	hotkeys = nil
	hotkeysWin = nil
	hotkeysList = nil

	makeHotkeysWindow()
	if hotkeysList == nil {
		t.Fatalf("hotkeys window not initialized")
	}
	if len(hotkeysList.Contents) != 0 {
		t.Fatalf("expected empty list")
	}

	openHotkeyEditor(-1)
	hotkeyComboText.Text = "Ctrl-X"
	hotkeyCmdInputs[0].Text = "say hi"
	finishHotkeyEdit(true)

	if len(hotkeysList.Contents) != 1 {
		t.Fatalf("hotkeys list not refreshed: %d", len(hotkeysList.Contents))
	}
	row := hotkeysList.Contents[0]
	if row == nil || len(row.Contents) == 0 {
		t.Fatalf("hotkey row malformed")
	}
	if got := row.Contents[0].Text; got != "Ctrl-X -> say hi" {
		t.Fatalf("unexpected hotkey text: %q", got)
	}
}

// Test that loading hotkeys from disk refreshes the hotkeys window list.
func TestLoadHotkeysShowsEntriesInWindow(t *testing.T) {
	hotkeys = nil
	hotkeysWin = nil
	hotkeysList = nil
	pluginHotkeyEnabled = map[string]map[string]bool{}

	dir := t.TempDir()
	origDir := dataDirPath
	dataDirPath = dir
	defer func() { dataDirPath = origDir }()

	// Create the hotkeys window initially with no entries.
	makeHotkeysWindow()
	if hotkeysList == nil {
		t.Fatalf("hotkeys window not initialized")
	}
	if len(hotkeysList.Contents) != 0 {
		t.Fatalf("expected empty hotkeys list")
	}

	// Write a hotkey entry to disk and load it.
	hk := []Hotkey{{Combo: "Ctrl-B", Name: "Bye", Commands: []HotkeyCommand{{Command: "say bye"}}}}
	data, err := json.Marshal(hk)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	err = os.WriteFile(filepath.Join(dir, hotkeysFile), data, 0o644)
	if err != nil {
		t.Fatalf("write hotkeys: %v", err)
	}

	loadHotkeys()

	if len(hotkeysList.Contents) != 2 {
		t.Fatalf("hotkeys list not refreshed: %d", len(hotkeysList.Contents))
	}
	row := hotkeysList.Contents[0]
	if row == nil || len(row.Contents) == 0 {
		t.Fatalf("hotkey row malformed")
	}
	if got := row.Contents[0].Text; got != "Bye : Ctrl-B -> say bye" {
		t.Fatalf("unexpected hotkey text: %q", got)
	}
}

// Test that long input lines wrap and cause the window to grow.
func TestHotkeyEditorWrapsAndResizes(t *testing.T) {
	hotkeyEditWin = nil
	openHotkeyEditor(-1)
	if hotkeyEditWin == nil {
		t.Fatalf("editor not opened")
	}
	base := hotkeyEditWin.Size.Y
	long := "this is a very long command line that should wrap across multiple lines for testing"
	hotkeyCmdInputs[0].Text = long
	wrapHotkeyInputs()
	if !strings.Contains(hotkeyCmdInputs[0].Text, "\n") || hotkeyCmdInputs[0].Size.Y <= 20 {
		t.Fatalf("command input did not wrap or grow: %q size %v", hotkeyCmdInputs[0].Text, hotkeyCmdInputs[0].Size.Y)
	}
	if hotkeyEditWin.Size.Y <= base {
		t.Fatalf("window did not resize: %v <= %v", hotkeyEditWin.Size.Y, base)
	}
	hotkeyEditWin.Close()
}

// Test that @right.clicked in commands expands to the last right-clicked mobile name.
func TestApplyHotkeyVars(t *testing.T) {
	// Populate lastClickByButton for right-click
	lastClickByButtonMu.Lock()
	lastClickByButton[ebiten.MouseButtonRight] = ClickInfo{OnMobile: true, Mobile: Mobile{Name: "Target"}}
	lastClickByButtonMu.Unlock()
	got, ok := applyHotkeyVars("/use @right.clicked")
	if !ok || got != "/use Target" {
		t.Fatalf("got %q, ok %v", got, ok)
	}
}

// Test that @hovered in commands expands to the currently hovered mobile name.
func TestApplyHotkeyVarsHovered(t *testing.T) {
	lastHoverMu.Lock()
	lastHover = ClickInfo{OnMobile: true, Mobile: Mobile{Name: "Hover"}}
	lastHoverMu.Unlock()
	got, ok := applyHotkeyVars("/inspect @hovered")
	if !ok || got != "/inspect Hover" {
		t.Fatalf("got %q, ok %v", got, ok)
	}
}

// Test that commands referencing @right.clicked don't fire without a target.
func TestApplyHotkeyVarsNoClicked(t *testing.T) {
	lastClickByButtonMu.Lock()
	delete(lastClickByButton, ebiten.MouseButtonRight)
	lastClickByButtonMu.Unlock()
	if got, ok := applyHotkeyVars("/use @right.clicked"); ok || got != "" {
		t.Fatalf("got %q, ok %v", got, ok)
	}
}

// Test that commands referencing @hovered don't fire without a target.
func TestApplyHotkeyVarsNoHovered(t *testing.T) {
	lastHoverMu.Lock()
	lastHover = ClickInfo{}
	lastHoverMu.Unlock()
	if got, ok := applyHotkeyVars("/inspect @hovered"); ok || got != "" {
		t.Fatalf("got %q, ok %v", got, ok)
	}
}

// Test that hotkey equip commands skip already equipped items.
func TestHotkeyEquipAlreadyEquipped(t *testing.T) {
	resetInventory()
	addInventoryItem(100, -1, "Sword", true)
	consoleLog = messageLog{max: maxMessages}
	if !hotkeyEquipAlreadyEquipped("/equip 100") {
		t.Fatalf("expected command to be skipped")
	}
	msgs := getConsoleMessages()
	if len(msgs) == 0 || msgs[len(msgs)-1] != "Sword already equipped, skipping" {
		t.Fatalf("unexpected console messages %v", msgs)
	}
}

// Test that plugin hotkeys are rendered with a valid font size.
func TestPluginHotkeysFontSize(t *testing.T) {
	hotkeys = []Hotkey{{Combo: "Ctrl-P", Plugin: "plug", Commands: []HotkeyCommand{{Command: "say hi"}}}}
	hotkeysWin = nil
	hotkeysList = nil
	pluginDisplayNames = map[string]string{"plug": "Plugin"}
	pluginCategories = map[string]string{"plug": ""}
	pluginSubCategories = map[string]string{"plug": ""}
	pluginHotkeyEnabled = map[string]map[string]bool{}

	makeHotkeysWindow()

	if len(hotkeysList.Contents) != 2 {
		t.Fatalf("expected plugin header and row, got %d", len(hotkeysList.Contents))
	}
	header := hotkeysList.Contents[0]
	if header.FontSize == 0 {
		t.Fatalf("plugin header font size not set")
	}
	row := hotkeysList.Contents[1]
	if len(row.Contents) != 2 {
		t.Fatalf("plugin row malformed")
	}
	lbl := row.Contents[1]
	if lbl.FontSize == 0 {
		t.Fatalf("plugin hotkey label font size not set")
	}
}

// Test that enabling a plugin hotkey persists only its state and not the
// command details.
func TestPluginHotkeyStatePersisted(t *testing.T) {
	hotkeys = nil
	pluginHotkeyEnabled = map[string]map[string]bool{}
	dir := t.TempDir()
	origDir := dataDirPath
	dataDirPath = dir
	defer func() { dataDirPath = origDir }()

	// Add plugin hotkey and enable it.
	pluginAddHotkey("plug", "Ctrl-P", "say hi")
	if len(hotkeys) != 1 {
		t.Fatalf("expected one plugin hotkey")
	}
	hotkeysMu.Lock()
	hotkeys[0].Disabled = false
	hotkeysMu.Unlock()
	if pluginHotkeyEnabled["plug"] == nil {
		pluginHotkeyEnabled["plug"] = map[string]bool{}
	}
	pluginHotkeyEnabled["plug"]["Ctrl-P"] = true
	saveHotkeys()

	// File should not contain command text.
	data, err := os.ReadFile(filepath.Join(dir, hotkeysFile))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(data), "say hi") {
		t.Fatalf("plugin command persisted: %s", data)
	}

	// Simulate restart.
	hotkeys = nil
	pluginHotkeyEnabled = map[string]map[string]bool{}
	loadHotkeys()
	if !pluginHotkeyEnabled["plug"]["Ctrl-P"] {
		t.Fatalf("expected enabled state, got disabled")
	}

	pluginAddHotkey("plug", "Ctrl-P", "say hi")
	found := false
	hotkeysMu.RLock()
	for _, hk := range hotkeys {
		if hk.Plugin == "plug" {
			found = true
			if hk.Disabled {
				t.Fatalf("hotkey disabled after reload")
			}
		}
	}
	hotkeysMu.RUnlock()
	if !found {
		t.Fatalf("plugin hotkey not re-added")
	}
}

// Test that removing a plugin hotkey clears it from all state and UI.
func TestPluginRemoveHotkeyClearsState(t *testing.T) {
	origHotkeys := hotkeys
	hotkeys = nil
	t.Cleanup(func() { hotkeys = origHotkeys })

	origEnabled := pluginHotkeyEnabled
	pluginHotkeyEnabled = map[string]map[string]bool{"plug": {"Ctrl-P": true}}
	t.Cleanup(func() { pluginHotkeyEnabled = origEnabled })

	origWin := hotkeysWin
	origList := hotkeysList
	hotkeysWin = nil
	hotkeysList = nil
	t.Cleanup(func() {
		hotkeysWin = origWin
		hotkeysList = origList
	})

	origDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = origDir })

	origDisabled := pluginDisabled
	pluginDisabled = map[string]bool{}
	origInvalid := pluginInvalid
	pluginInvalid = map[string]bool{}
	t.Cleanup(func() {
		pluginDisabled = origDisabled
		pluginInvalid = origInvalid
	})
	origEnabledPlugins := pluginEnabledFor
	pluginEnabledFor = map[string]pluginScope{}
	t.Cleanup(func() { pluginEnabledFor = origEnabledPlugins })

	makeHotkeysWindow()

	pluginAddHotkey("plug", "Ctrl-P", "say hi")

	hotkeysMu.RLock()
	if len(hotkeys) != 1 {
		hotkeysMu.RUnlock()
		t.Fatalf("expected hotkey added, got %d", len(hotkeys))
	}
	hotkeysMu.RUnlock()
	if m := pluginHotkeyEnabled["plug"]; m == nil || !m["Ctrl-P"] {
		t.Fatalf("expected pluginHotkeyEnabled entry before removal")
	}
	if len(hotkeysList.Contents) != 2 {
		t.Fatalf("expected hotkey list to have plugin header and row before removal, got %d", len(hotkeysList.Contents))
	}

	pluginRemoveHotkey("plug", "Ctrl-P")

	hotkeysMu.RLock()
	for _, hk := range hotkeys {
		if hk.Plugin == "plug" && hk.Combo == "Ctrl-P" {
			hotkeysMu.RUnlock()
			t.Fatalf("plugin hotkey not removed")
		}
	}
	hotkeysMu.RUnlock()

	if _, ok := pluginHotkeyEnabled["plug"]; ok {
		t.Fatalf("pluginHotkeyEnabled entry remains after removal")
	}

	if len(hotkeysList.Contents) != 0 {
		t.Fatalf("expected empty hotkeys list after removal, got %d", len(hotkeysList.Contents))
	}
}
