package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestClassicMacroModifierInventory(t *testing.T) {
	// Macros_cl.cp gModNameMap, ClanLordClient 6ba334cfb3fb779ecfe37e0b635fac476cb73a5e.
	for _, modifier := range []struct {
		name string
		flag legacyMacroModifiers
	}{
		{"command", legacyMacroModCommand},
		{"control", legacyMacroModControl},
		{"numpad", legacyMacroModNumpad},
		{"option", legacyMacroModOption},
		{"shift", legacyMacroModShift},
	} {
		t.Run(modifier.name, func(t *testing.T) {
			combo := strings.ToUpper(modifier.name) + "-F6"
			program := parseLegacyMacroSources([]legacyMacroSource{{Path: filepath.Join(t.TempDir(), "classic.mac"), Text: combo + " message \"ran\"\n"}})
			var messages []string
			runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{Message: func(s string) { messages = append(messages, s) }})
			if started, _ := runtime.triggerKey("f6", modifier.flag, 0); !started || !equalStrings(messages, []string{"ran"}) {
				t.Fatalf("classic modifier did not dispatch: %v", messages)
			}
		})
	}
	for _, invented := range []string{"apple", "openapple", "open-apple", "open_apple", "cmd", "opt", "ctl", "⌘", "⌥", "⌃", "⇧", "capslock", "banana", ""} {
		if _, _, ok := parseLegacyMacroKeyBinding(invented + "-f6"); ok {
			t.Errorf("unsupported modifier accepted: %q", invented)
		}
	}
}

func TestClassicMacroNamedKeyInventory(t *testing.T) {
	// Macros_cl.cp gKeyNameMap, ClanLordClient 6ba334cfb3fb779ecfe37e0b635fac476cb73a5e.
	keys := "escape f1 f2 f3 f4 f5 f6 f7 f8 f9 f10 f11 f12 f13 f14 f15 f16 minus delete tab return space help home pageup del end pagedown up down left right clear enter click click2 right-click click3 click4 click5 click6 click7 click8 wheelup wheeldown wheelleft wheelright"
	if _, _, ok := parseLegacyMacroKeyBinding("undo"); ok {
		t.Fatal("undo is not a classic key name")
	}
	for _, name := range strings.Fields(keys) {
		if _, _, ok := parseLegacyMacroKeyBinding("command-option-" + name); !ok {
			t.Errorf("missing classic named key %q", name)
		}
	}
}

func TestClassicDeleteKeysStayDistinct(t *testing.T) {
	for _, key := range []ebiten.Key{ebiten.KeyBackspace, ebiten.KeyDelete} {
		name, _, _ := legacyMacroKeyName(key)
		for _, trigger := range []string{"delete", "del"} {
			_, binding, _ := parseLegacyMacroKeyBinding(trigger)
			decl := legacyMacroDeclaration{Kind: legacyMacroKey, Key: binding}
			if legacyMacroKeyMatches(decl, name, 0) != (name == trigger) {
				t.Errorf("%s wrongly matched %s", trigger, key)
			}
		}
		legacyMacroBeginInputFrame()
		legacyMacroMarkKeyConsumed(key, name)
		if legacyMacroHotkeySuppressed("Backspace") != (key == ebiten.KeyBackspace) || legacyMacroHotkeySuppressed("Delete") != (key == ebiten.KeyDelete) {
			t.Errorf("%s suppressed the other delete key", key)
		}
	}
	legacyMacroBeginInputFrame()
}

func TestLegacyTriggerEditorRejectsClassicModifierConflict(t *testing.T) {
	doc := triggerEditorFixture(t, []byte("command-f6 message \"one\"\nf7 message \"two\"\n"), true)
	if err := doc.save([]string{"command-f6", "COMMAND-f6"}); err == nil {
		t.Fatal("editor allowed equivalent command bindings")
	}
}

func TestClassicReturnAndKeypadEnter(t *testing.T) {
	for _, key := range []ebiten.Key{ebiten.KeyEnter, ebiten.KeyNumpadEnter} {
		name, numpad, _ := legacyMacroKeyName(key)
		if numpad {
			t.Fatal("classic Return and Enter do not carry the numpad modifier")
		}
		for _, trigger := range []string{"return", "enter", "numpad-enter"} {
			_, binding, _ := parseLegacyMacroKeyBinding(trigger)
			decl := legacyMacroDeclaration{Kind: legacyMacroKey, Key: binding}
			want := key == ebiten.KeyEnter && trigger == "return" || key == ebiten.KeyNumpadEnter && trigger != "return"
			if legacyMacroKeyMatches(decl, name, 0) != want {
				t.Errorf("%s wrongly matched %s", trigger, key)
			}
		}
		legacyMacroBeginInputFrame()
		legacyMacroMarkKeyConsumed(key, name)
		if legacyMacroHotkeySuppressed("Enter") != (key == ebiten.KeyEnter) || legacyMacroHotkeySuppressed("NumpadEnter") != (key == ebiten.KeyNumpadEnter) {
			t.Errorf("%s suppressed the other enter key", key)
		}
	}
	legacyMacroBeginInputFrame()
}

func TestClassicKeypadClear(t *testing.T) {
	name, numpad, ok := legacyMacroKeyName(ebiten.KeyNumLock)
	if !ok || !numpad || name != "clear" {
		t.Fatal("Mac keypad Clear must map to numpad-clear")
	}
	for _, trigger := range []string{"numpad-clear", "numpad-escape"} {
		_, binding, _ := parseLegacyMacroKeyBinding(trigger)
		if !legacyMacroKeyMatches(legacyMacroDeclaration{Key: binding}, name, legacyMacroModNumpad) {
			t.Errorf("Clear did not match %s", trigger)
		}
	}
	legacyMacroBeginInputFrame()
	legacyMacroMarkKeyConsumed(ebiten.KeyNumLock, name)
	if !legacyMacroHotkeySuppressed("NumLock") || legacyMacroHotkeySuppressed("Escape") {
		t.Fatal("Clear suppressed the wrong hotkey")
	}
	legacyMacroBeginInputFrame()
}
