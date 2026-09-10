package main

import (
	"testing"

	"gothoom/eui"
)

func TestNamedMobileEffectsResolveAndClear(t *testing.T) {
	resetScriptCallbackTestState(t, "named_effects")
	for _, outline := range []bool{false, true} {
		base := scriptMobileTint{r: 255, a: 255}
		named := scriptMobileTint{g: 255, a: 255}
		winner := scriptMobileTint{b: 255, a: 255}
		if outline {
			scriptSetMobileOutline("zeta", 22, base.r, base.g, base.b, base.a)
		} else {
			scriptSetMobileTint("zeta", 22, base.r, base.g, base.b, base.a)
		}
		scriptSetNamedMobileEffect("alpha", " Sam ", named, outline)
		for _, id := range []uint16{22, 71} {
			if got, ok := scriptMobileEffectForMobile(id, "SAM", outline); !ok || got != named {
				t.Fatalf("name must override sprite, outline=%v id=%d: %+v, %v", outline, id, got, ok)
			}
		}
		if got, _ := scriptMobileEffectForMobile(22, "Samantha", outline); got != base {
			t.Fatal("name match leaked to another player")
		}
		scriptSetNamedMobileEffect("zeta", "sam", winner, outline)
		if got, _ := scriptMobileEffectForMobile(22, "Sam", outline); got != winner {
			t.Fatal("named effects did not resolve deterministically")
		}
		scriptClearNamedMobileEffect("zeta", " SAM ", outline)
		if got, _ := scriptMobileEffectForMobile(22, "Sam", outline); got != named {
			t.Fatal("clearing a name removed another script's effect")
		}
		if outline {
			scriptClearMobileOutlines("alpha")
		} else {
			scriptClearMobileTints("alpha")
		}
		if got, _ := scriptMobileEffectForMobile(22, "Sam", outline); got != base {
			t.Fatal("clear-all did not release named effect")
		}
		scriptSetNamedMobileEffect("alpha", "  ", named, outline)
		if _, ok := scriptMobileEffectForMobile(71, "", outline); ok {
			t.Fatal("empty name matched unnamed mobiles")
		}
	}
}

func TestMarkBeastsNamedEntry(t *testing.T) {
	initFont()
	const owner = "mark_beasts_named"
	sim := activateBundledProofScript(t, owner, "mark_beasts.go")
	click := makeScriptInputEvent("Alt-LeftClick")
	click.OnMobile = true
	click.Mobile.PictID, click.Mobile.Name, click.Mobile.Player = 22, "Sam", true
	if sim.input(t, click) {
		t.Fatal("Alt-click should consume input")
	}
	sim.barrier(t)
	window := func() Window {
		value, err := currentScriptEventQueue(owner).interpreter.Eval("lastiesWindow")
		if err != nil {
			t.Fatal(err)
		}
		return value.Interface().(Window)
	}
	panel := window()
	if panel.state.controls["entry-1-name"].option.Text != "Sam" || panel.state.controls["entry-1-id"].option.Text != "22" {
		t.Fatal("Alt-click did not populate the name and preview ID")
	}
	if _, ok := scriptMobileEffectForMobile(71, "sam", false); ok {
		t.Fatal("new named mark should not tint")
	}
	if _, ok := scriptMobileEffectForMobile(71, "sam", true); !ok {
		t.Fatal("new named mark should outline")
	}
	panel.state.controls["entry-1-tint-enabled"].item.Handler.Emit(eui.UIEvent{Type: eui.EventCheckboxChanged, Checked: true})
	sim.barrier(t)
	for _, outline := range []bool{false, true} {
		if _, ok := scriptMobileEffectForMobile(71, "sam", outline); !ok {
			t.Fatal("named player lost their mark after a sprite change")
		}
		if _, ok := scriptMobileEffectForMobile(22, "Someone Else", outline); ok {
			t.Fatal("another player with the same sprite received the named mark")
		}
	}
	edit := func(id, value string) {
		panel.state.controls[id].item.Handler.Emit(eui.UIEvent{Type: eui.EventInputChanged, Text: value})
		sim.barrier(t)
	}
	edit("entry-1-note", "Great healer; needs lastie X")
	edit("entry-1-id", "") // A name match needs no sprite ID.
	edit("entry-1-name", " Ann ")
	if _, ok := scriptMobileEffectForMobile(22, "Sam", false); ok {
		t.Fatal("old name stayed marked after editing")
	}
	if _, ok := scriptMobileEffectForMobile(71, "ann", false); !ok {
		t.Fatal("name-only entry did not match")
	}
	src, err := scriptScripts.ReadFile(bundledScriptDir + "/mark_beasts.go")
	if err != nil {
		t.Fatal(err)
	}
	if !loadscriptSource(owner, "Mark Beasts", "mark_beasts.go", src, restrictedStdlib()) {
		t.Fatal("reload failed")
	}
	sim.barrier(t)
	panel = window()
	if panel.state.controls["entry-1-name"].option.Text != " Ann " || panel.state.controls["entry-1-note"].option.Text != "Great healer; needs lastie X" {
		t.Fatal("name or note did not survive reload")
	}
	edit("entry-1-id", "22")
	edit("entry-1-name", "")
	if _, ok := scriptMobileEffectForMobile(22, "Other", false); !ok {
		t.Fatal("blank name did not restore sprite-ID matching")
	}
	edit("entry-1-name", "Ann")
	click.Mobile.Name, click.Mobile.PictID = "ANN", 71
	sim.input(t, click)
	sim.barrier(t)
	if len(panel.state.rowIDs) != 0 || len(scriptNamedMobileTints[owner]) != 0 || len(scriptNamedMobileOutlines[owner]) != 0 {
		t.Fatal("second Alt-click did not remove the named entry and its effects")
	}
	sim.input(t, click)
	sim.barrier(t)
	disablescript(owner, "test cleanup")
	drainScriptDispatcher()
	if len(scriptNamedMobileTints[owner]) != 0 || len(scriptNamedMobileOutlines[owner]) != 0 {
		t.Fatal("stopping the script did not release named effects")
	}
}
