package main

import (
	"path/filepath"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"gothoom/eui"
)

// Reference: ClanLordClient GameWin_cl.cp DoMouseDownEvent and Macros_cl.cp
// DoMacroClick at 6ba334cfb3fb779ecfe37e0b635fac476cb73a5e.
func TestLegacyMacroRightClickClassicDispatch(t *testing.T) {
	for _, test := range []struct {
		name, source                     string
		modifiers                        legacyMacroModifiers
		ground, playerList, allowDefault bool
		want                             []string
	}{
		{name: "right click takes precedence", source: "click2 message \"right\"\ncontrol-click message \"fallback\"\n", want: []string{"right"}},
		{name: "right click alias", source: "right-click message \"right\"\ncontrol-click message \"fallback\"\n", want: []string{"right"}},
		{name: "control click fallback", source: "control-click message \"fallback\"\n", want: []string{"fallback"}},
		{name: "preserves shift", modifiers: legacyMacroModShift, source: "shift-control-click message \"shift\"\ncontrol-click message \"wrong\"\n", want: []string{"shift"}},
		{name: "already holding control", modifiers: legacyMacroModControl, source: "control-click message \"fallback\"\n", want: []string{"fallback"}},
		{name: "no override runs both", source: "click2\n{\n$no_override\nmessage \"right\"\n}\ncontrol-click message \"fallback\"\n", want: []string{"right", "fallback"}},
		{name: "both pass through", source: "click2\n{\n$no_override\nmessage \"right\"\n}\ncontrol-click\n{\n$no_override\nmessage \"fallback\"\n}\n", want: []string{"right", "fallback"}, allowDefault: true},
		{name: "no macro", allowDefault: true},
		{name: "ground requires any click", ground: true, source: "click2 message \"wrong\"\ncontrol-click message \"wrong\"\n", allowDefault: true},
		{name: "ground fallback", ground: true, source: "click2 message \"wrong\"\ncontrol-click\n{\n$any_click\nmessage \"ground\"\n}\n", want: []string{"ground"}},
		{name: "ground right click", ground: true, source: "click2\n{\n$any_click\nmessage \"ground\"\n}\ncontrol-click message \"wrong\"\n", want: []string{"ground"}},
		{name: "button and chord context", source: "click2\n{\n$no_override\nmessage @click.button \"/\" @click.chord \"/\" @click.simple_name\n}\ncontrol-click message @click.button \"/\" @click.chord \"/\" @click.simple_name\n", want: []string{"2 / 3 / BobJones", "1 / 1 / BobJones"}},
		{name: "player list uses control click", playerList: true, source: "click2 message \"wrong\"\ncontrol-click message @click.simple_name\n", want: []string{"BobJones"}},
		{name: "player list ignores click2", playerList: true, source: "click2 message \"wrong\"\n", allowDefault: true},
		{name: "player list no override", playerList: true, source: "control-click\n{\n$no_override\nmessage \"fallback\"\n}\n", want: []string{"fallback"}, allowDefault: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			program := parseLegacyMacroSources([]legacyMacroSource{{Path: filepath.Join(t.TempDir(), "right.mac"), Text: test.source}})
			if len(program.Diagnostics) != 0 {
				t.Fatalf("fixture diagnostics: %v", program.Diagnostics)
			}
			var messages []string
			runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{Message: func(s string) { messages = append(messages, s) }})
			event := legacyMacroClickEvent{Name: "Bob Jones", HasName: true, OnPlayer: !test.ground, Button: 2, Chord: 3, HasButton: !test.playerList, HasChord: !test.playerList, Modifiers: test.modifiers}
			if test.ground {
				event.Name = ""
			}
			started, allowDefault := runtime.triggerRightClick(event, 0)
			if started != (len(test.want) > 0) || allowDefault != test.allowDefault || !equalStrings(messages, test.want) {
				t.Fatalf("dispatch=(%t,%t) messages=%v; want=(%t,%t) %v", started, allowDefault, messages, len(test.want) > 0, test.allowDefault, test.want)
			}
		})
	}
}

func TestPlayersRightClickMacroRunsBeforeContextMenu(t *testing.T) {
	initFont()
	oldWin, oldList, oldRefs := playersWin, playersList, playersRowRefs
	oldRuntime, oldSelected := legacyMacrosRuntime, selectedPlayerName
	legacyMacroInputState.Lock()
	oldMouse, oldConsumed := legacyMacroInputState.consumedMouse, legacyMacroInputState.consumed
	legacyMacroInputState.consumedMouse, legacyMacroInputState.consumed = nil, nil
	legacyMacroInputState.Unlock()
	t.Cleanup(func() {
		playersWin.RemoveWindow()
		playersWin, playersList, playersRowRefs = oldWin, oldList, oldRefs
		legacyMacrosRuntime, selectedPlayerName = oldRuntime, oldSelected
		legacyMacroInputState.Lock()
		legacyMacroInputState.consumedMouse, legacyMacroInputState.consumed = oldMouse, oldConsumed
		legacyMacroInputState.Unlock()
		eui.CloseContextMenus()
	})
	playersWin = eui.NewWindow()
	playersList = eui.NewColumn()
	row := eui.NewRow()
	playersList.AddItem(row)
	playersWin.AddItem(playersList)
	playersWin.MarkOpen()
	row.DrawRect = eui.Rect{X0: 10, Y0: 10, X1: 150, Y1: 40}
	playersRowRefs = map[*eui.ItemData]playerRef{row: {session: primarySessionID, name: "Bob Jones"}}
	selectedPlayerName = "Previous selection"
	program := parseLegacyMacroSources([]legacyMacroSource{{Path: filepath.Join(t.TempDir(), "right.mac"), Text: "control-click message @click.name\n"}})
	var messages []string
	legacyMacrosRuntime = newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{Message: func(s string) { messages = append(messages, s) }})
	eui.CloseContextMenus()
	if !handlePlayersContextClick(20, 20) || !equalStrings(messages, []string{"Bob Jones"}) {
		t.Fatalf("player-row right click did not reach the macro: %v", messages)
	}
	if eui.ContextMenusOpen() || selectedPlayerName != "Previous selection" || !legacyMacroMouseConsumed(ebiten.MouseButtonRight) {
		t.Fatal("consumed right click opened a menu, changed selection, or was not marked consumed")
	}
}
