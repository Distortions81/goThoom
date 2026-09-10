package main

import (
	"testing"
	"time"

	"gothoom/eui"
)

func TestScriptMobileTintLifecycleAndResolution(t *testing.T) {
	overlayMu.Lock()
	original := scriptMobileTints
	originalOutlines := scriptMobileOutlines
	originalFlashes := scriptMobileFlashes
	scriptMobileTints = map[string]map[uint16]scriptMobileTint{}
	scriptMobileOutlines = map[string]map[uint16]scriptMobileTint{}
	scriptMobileFlashes = map[string]map[uint8]scriptMobileFlash{}
	overlayMu.Unlock()
	t.Cleanup(func() {
		overlayMu.Lock()
		scriptMobileTints = original
		scriptMobileOutlines = originalOutlines
		scriptMobileFlashes = originalFlashes
		overlayMu.Unlock()
	})

	scriptSetMobileTint("alpha", 42, 255, 128, 128, 255)
	scriptSetMobileTint("zeta", 42, 64, 255, 64, 200)
	if tint, ok := scriptMobileTintForPict(42); !ok || tint != (scriptMobileTint{r: 64, g: 255, b: 64, a: 200}) {
		t.Fatalf("resolved tint = %+v, %v", tint, ok)
	}
	scriptClearMobileTint("zeta", 42)
	if tint, ok := scriptMobileTintForPict(42); !ok || tint != (scriptMobileTint{r: 255, g: 128, b: 128, a: 255}) {
		t.Fatalf("fallback tint = %+v, %v", tint, ok)
	}
	scriptClearMobileTints("alpha")
	if _, ok := scriptMobileTintForPict(42); ok {
		t.Fatal("cleared tint remained active")
	}
	scriptSetMobileTint("alpha", 0, 1, 2, 3, 4)
	scriptSetMobileTint("alpha", 0xffff, 1, 2, 3, 4)
	if len(scriptMobileTints["alpha"]) != 0 {
		t.Fatal("invalid pict ID received a tint")
	}
	scriptSetMobileOutline("alpha", 42, 255, 64, 64, 255)
	if outline, ok := scriptMobileOutlineForPict(42); !ok || outline != (scriptMobileTint{r: 255, g: 64, b: 64, a: 255}) {
		t.Fatalf("mobile outline = %+v, %v", outline, ok)
	}
	scriptClearMobileOutlines("alpha")
	if _, ok := scriptMobileOutlineForPict(42); ok {
		t.Fatal("cleared outline remained active")
	}
}

func TestScriptMobileFlashLifecycleAndResolution(t *testing.T) {
	overlayMu.Lock()
	original := scriptMobileFlashes
	scriptMobileFlashes = map[string]map[uint8]scriptMobileFlash{}
	overlayMu.Unlock()
	t.Cleanup(func() {
		overlayMu.Lock()
		scriptMobileFlashes = original
		overlayMu.Unlock()
	})

	scriptFlashMobile("alpha", 7, 255, 48, 48, 255, time.Second)
	scriptFlashMobile("zeta", 7, 64, 255, 64, 200, time.Second)
	if tint, ok := scriptMobileFlashForIndex(7); !ok || tint != (scriptMobileTint{r: 64, g: 255, b: 64, a: 200}) {
		t.Fatalf("resolved flash = %+v, %v", tint, ok)
	}
	scriptFlashMobile("alpha", 8, 255, 48, 48, 255, time.Nanosecond)
	time.Sleep(time.Millisecond)
	if _, ok := scriptMobileFlashForIndex(8); ok {
		t.Fatal("expired flash remained active")
	}
}

func TestMarkLastiesAltClickProof(t *testing.T) {
	initFont()
	const owner = "mark_lasties_proof"
	sim := activateBundledProofScript(t, owner, "mark_beasts.go")
	click := makeScriptInputEvent("Alt-LeftClick")
	click.OnMobile = true
	click.Mobile.PictID = 777
	if sim.input(t, click) {
		t.Fatal("Alt-click should not also move or attack")
	}
	sim.barrier(t)
	overlayMu.RLock()
	tint, marked := scriptMobileOutlines[owner][777]
	overlayMu.RUnlock()
	if len(scriptMobileTints[owner]) != 0 {
		t.Fatal("new marks should use outline only")
	}
	if !marked || tint != (scriptMobileTint{r: 255, g: 255, b: 0, a: 255}) {
		t.Fatalf("Alt-click mark = %+v, %v", tint, marked)
	}
	if sim.input(t, click) {
		t.Fatal("Alt-click removal should still consume input")
	}
	sim.barrier(t)
	overlayMu.RLock()
	_, marked = scriptMobileOutlines[owner][777]
	overlayMu.RUnlock()
	if marked {
		t.Fatal("second Alt-click did not remove mark")
	}
}

func TestMarkLastiesInlineEditsAutosave(t *testing.T) {
	initFont()
	const owner = "mark_lasties_rows_proof"
	sim := activateBundledProofScript(t, owner, "mark_beasts.go")
	window := func() Window {
		value, err := currentScriptEventQueue(owner).interpreter.Eval("lastiesWindow")
		if err != nil {
			t.Fatal(err)
		}
		return value.Interface().(Window)
	}
	panel := window()
	click := func(id string) {
		t.Helper()
		button := panel.state.buttons[id]
		if button == nil {
			t.Fatalf("missing button %s", id)
		}
		button.Handler.Emit(eui.UIEvent{Type: eui.EventClick})
		sim.barrier(t)
	}
	edit := func(id, text string) {
		t.Helper()
		panel.state.controls[id].item.Handler.Emit(eui.UIEvent{Type: eui.EventInputChanged, Text: text})
		sim.barrier(t)
	}
	click("add")
	if panel.state.controls["entry-1-id"].item.Text != "" || len(scriptMobileTints[owner]) != 0 {
		t.Fatal("Add should create an empty row without marking a sprite")
	}
	edit("entry-1-id", "22")
	edit("entry-1-note", "Leave for Sam — needs last hits")
	if _, ok := scriptMobileOutlines[owner][22]; !ok || panel.state.controls["entry-1-image"].option.Image != 22 {
		t.Fatal("inline ID edit did not mark sprite 22 and update its preview")
	}
	panel.state.controls["entry-1-tint-enabled"].item.Handler.Emit(eui.UIEvent{Type: eui.EventCheckboxChanged, Checked: true})
	sim.barrier(t)
	for id, value := range map[string]uint32{"entry-1-tint": 0x80ff80ff, "entry-1-outline": 0x4080ffff} {
		control := panel.state.controls[id]
		panel.state.colorPicked(control, value, control.revision)
		sim.barrier(t)
		if packScriptWindowColor(control.item.WheelColor) != value {
			t.Fatal("swatch did not update")
		}
	}
	edit("entry-1-id", "71")
	if _, ok := scriptMobileTints[owner][22]; ok {
		t.Fatal("old sprite ID remained marked")
	}
	if scriptMobileTints[owner][71] != (scriptMobileTint{r: 128, g: 255, b: 128, a: 255}) || scriptMobileOutlines[owner][71] != (scriptMobileTint{r: 64, g: 128, b: 255, a: 255}) {
		t.Fatal("row colors did not follow the edited sprite ID")
	}
	click("add")
	edit("entry-1-note", "Sam’s last hits — priority")
	edit("entry-2-note", "Identify this sprite on the next hunt")
	// Reload from persisted script data: notes, edited colors, and blank rows survive.
	src, err := scriptScripts.ReadFile(bundledScriptDir + "/mark_beasts.go")
	if err != nil {
		t.Fatal(err)
	}
	if !loadscriptSource(owner, "Mark Beasts", "mark_beasts.go", src, restrictedStdlib()) {
		t.Fatal("reload failed")
	}
	sim.barrier(t)
	panel = window()
	if panel.state.controls["entry-1-id"].option.Text != "71" || panel.state.controls["entry-2-id"].option.Text != "" || panel.state.controls["entry-1-outline"].option.Color != 0x4080ffff {
		t.Fatal("automatic save did not retain edited and blank rows across reload")
	}
	if panel.state.controls["entry-1-note"].option.Text != "Sam’s last hits — priority" || panel.state.controls["entry-2-note"].option.Text != "Identify this sprite on the next hunt" {
		t.Fatal("notes did not autosave on existing and blank entries")
	}
	edit("entry-2-id", "71")
	if scriptMobileTints[owner][71].r != 128 {
		t.Fatal("duplicate row overwrote the first row")
	}
	edit("entry-2-id", "invalid")
	if len(scriptMobileTints[owner]) != 1 {
		t.Fatal("invalid ID created a mark")
	}
	oldInput := panel.state.controls["entry-1-id"].item
	click("entry-1-delete")
	if panel.state.controls["entry-1-id"] != nil || panel.state.controls["entry-2-id"] == nil {
		t.Fatal("delete targeted the wrong row")
	}
	click("entry-2-delete")
	oldInput.Handler.Emit(eui.UIEvent{Type: eui.EventInputChanged, Text: "22"})
	sim.barrier(t)
	if len(scriptMobileTints[owner]) != 0 || len(scriptMobileOutlines[owner]) != 0 || len(panel.state.rowIDs) != 0 {
		t.Fatal("delete failed to remove row, tint, or outline, or a stale callback restored it")
	}
}

func TestMarkLastiesMigratesExistingMarks(t *testing.T) {
	initFont()
	const owner = "mark_lasties_migration"
	resetScriptCallbackTestState(t, owner)
	t.Cleanup(func() { disablescript(owner, "test cleanup"); drainScriptDispatcher() })
	scriptStorageSet(owner, "marks", map[uint16]map[string]uint8{22: {
		"R": 120, "G": 200, "B": 255, "A": 255,
		"OutlineR": 20, "OutlineG": 240, "OutlineB": 60, "OutlineA": 255,
	}})
	src, err := scriptScripts.ReadFile(bundledScriptDir + "/mark_beasts.go")
	if err != nil {
		t.Fatal(err)
	}
	if !loadscriptSource(owner, "Mark Beasts", "mark_beasts.go", src, restrictedStdlib()) {
		t.Fatal("load failed")
	}
	scriptEventSimulator{owner: owner}.barrier(t)
	if scriptStorageGet(owner, "entries") == nil || scriptMobileTints[owner][22].r != 120 || scriptMobileOutlines[owner][22].g != 240 {
		t.Fatal("existing marked sprite and colors did not migrate to inline entries")
	}
}

func TestMarkBeastsPerEntryEffectsAndDefaults(t *testing.T) {
	initFont()
	const owner = "mark_beasts_defaults"
	sim := activateBundledProofScript(t, owner, "mark_beasts.go")
	window := func(name string) Window {
		t.Helper()
		value, err := currentScriptEventQueue(owner).interpreter.Eval(name)
		if err != nil {
			t.Fatal(err)
		}
		return value.Interface().(Window)
	}
	check := func(panel Window, id string, enabled bool) {
		t.Helper()
		control := panel.state.controls[id]
		if control == nil {
			t.Fatalf("missing checkbox %s", id)
		}
		control.item.Handler.Emit(eui.UIEvent{Type: eui.EventCheckboxChanged, Checked: enabled})
		sim.barrier(t)
	}
	mark := func(id uint16) {
		click := makeScriptInputEvent("Alt-LeftClick")
		click.OnMobile, click.Mobile.PictID = true, id
		sim.input(t, click)
		sim.barrier(t)
	}
	mark(22)
	panel := window("lastiesWindow")
	panel.state.buttons["settings"].Handler.Emit(eui.UIEvent{Type: eui.EventClick})
	sim.barrier(t)
	if scriptConfigWin == nil || scriptConfigOwner != owner || !scriptConfigWin.Open {
		t.Fatal("settings gear did not open native script settings")
	}
	if scriptConfigEntries[owner][0].Value.(bool) || !scriptConfigEntries[owner][2].Value.(bool) {
		t.Fatal("fresh defaults should enable only outlines")
	}
	if !scriptSetConfigValue(owner, "default-tint", true) ||
		!scriptSetConfigValue(owner, "default-outline", false) ||
		!scriptSetConfigValue(owner, "default-tint-color", uint32(0x90b0d0ff)) {
		t.Fatal("could not update native Mark Beasts preferences")
	}
	if len(scriptMobileTints[owner]) != 0 || len(scriptMobileOutlines[owner]) != 1 {
		t.Fatal("editing defaults changed an existing entry")
	}
	mark(71)
	if scriptMobileTints[owner][71] != (scriptMobileTint{r: 144, g: 176, b: 208, a: 255}) {
		t.Fatal("new entry did not inherit tint default and color")
	}
	if _, ok := scriptMobileOutlines[owner][71]; ok {
		t.Fatal("new entry ignored outline default")
	}
	check(panel, "entry-2-outline-enabled", true)
	if len(scriptMobileTints[owner]) != 1 || len(scriptMobileOutlines[owner]) != 2 {
		t.Fatal("could not enable tint and outline together")
	}
	check(panel, "entry-2-tint-enabled", false)
	check(panel, "entry-2-outline-enabled", false)
	if len(scriptMobileTints[owner]) != 0 || len(scriptMobileOutlines[owner]) != 1 {
		t.Fatal("disabling both effects did not clear just that entry")
	}
	src, err := scriptScripts.ReadFile(bundledScriptDir + "/mark_beasts.go")
	if err != nil {
		t.Fatal(err)
	}
	if !loadscriptSource(owner, "Mark Beasts", "mark_beasts.go", src, restrictedStdlib()) {
		t.Fatal("reload failed")
	}
	sim.barrier(t)
	panel = window("lastiesWindow")
	if panel.state.controls["entry-2-tint-enabled"].option.Checked || panel.state.controls["entry-2-outline-enabled"].option.Checked || panel.state.controls["entry-2-tint"].option.Color != 0x90b0d0ff {
		t.Fatal("disabled flags or saved color did not survive reload")
	}
	mark(82)
	if _, ok := scriptMobileOutlines[owner][82]; ok || scriptMobileTints[owner][82].b != 208 {
		t.Fatal("new-entry defaults did not survive reload")
	}
	check(panel, "entry-2-tint-enabled", true)
	if scriptMobileTints[owner][71].b != 208 {
		t.Fatal("reenabling tint did not restore its saved color")
	}
}

func TestMarkBeastsMigratesInlineEntriesAndDisabledOutlines(t *testing.T) {
	initFont()
	const owner = "mark_beasts_entry_migration"
	resetScriptCallbackTestState(t, owner)
	t.Cleanup(func() { disablescript(owner, "test cleanup"); drainScriptDispatcher() })
	scriptStorageSet(owner, "entries", []map[string]any{
		{"ID": "22", "Name": "Sam", "Note": "Save last hit", "Mark": map[string]uint8{"R": 120, "G": 200, "B": 255, "A": 255, "OutlineR": 20, "OutlineG": 240, "OutlineB": 60, "OutlineA": 255}},
		{"ID": "", "Note": "Blank row note"},
	})
	scriptStorageSet(owner, "show-outline", false)
	scriptStorageSet(owner, "hit-flash", map[string]any{"Enabled": false, "AllMobiles": true, "Color": map[string]uint8{"R": 12, "G": 34, "B": 56, "A": 78}})
	src, err := scriptScripts.ReadFile(bundledScriptDir + "/mark_beasts.go")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if !loadscriptSource(owner, "Mark Beasts", "mark_beasts.go", src, restrictedStdlib()) {
			t.Fatal("load failed")
		}
		scriptEventSimulator{owner: owner}.barrier(t)
		if scriptNamedMobileTints[owner]["sam"].r != 120 || len(scriptNamedMobileOutlines[owner]) != 0 {
			t.Fatal("migration lost tint or reenabled disabled outline")
		}
		value, err := currentScriptEventQueue(owner).interpreter.Eval("lastiesWindow")
		if err != nil {
			t.Fatal(err)
		}
		panel := value.Interface().(Window)
		if panel.state.controls["entry-1-note"].option.Text != "Save last hit" || panel.state.controls["entry-2-note"].option.Text != "Blank row note" || panel.state.controls["entry-1-outline"].option.Color != 0x14f03cff {
			t.Fatal("migration lost notes, blank row, or disabled outline color")
		}
		config := map[string]any{}
		for _, entry := range scriptConfigEntries[owner] {
			config[entry.Key] = entry.Value
		}
		if config["flash-hits"] != false || config["flash-all"] != true || config["flash-color"] != uint32(0x0c22384e) {
			t.Fatal("migration changed hit-flash settings")
		}
	}
}
