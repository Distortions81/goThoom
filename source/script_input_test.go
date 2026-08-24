package main

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestScriptKeyNameUsesPrintableModifiedCharacter(t *testing.T) {
	tests := []struct {
		name      string
		key       ebiten.Key
		pressed   []ebiten.Key
		typed     []rune
		shifted   bool
		alternate bool
		want      string
	}{
		{name: "plain letter", key: ebiten.KeyA, pressed: []ebiten.Key{ebiten.KeyA}, typed: []rune{'a'}, want: "A"},
		{name: "shifted digit", key: ebiten.KeyDigit3, pressed: []ebiten.Key{ebiten.KeyDigit3}, typed: []rune{'#'}, shifted: true, want: "#"},
		{name: "shifted fallback", key: ebiten.KeyDigit3, pressed: []ebiten.Key{ebiten.KeyDigit3}, shifted: true, want: "#"},
		{name: "alternate layout", key: ebiten.KeyM, pressed: []ebiten.Key{ebiten.KeyM}, typed: []rune{'µ'}, alternate: true, want: "µ"},
		{name: "space keeps key name", key: ebiten.KeySpace, pressed: []ebiten.Key{ebiten.KeySpace}, typed: []rune{' '}, shifted: true, want: "Space"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := scriptKeyName(test.key, test.pressed, test.typed, test.shifted, test.alternate); got != test.want {
				t.Fatalf("scriptKeyName() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestScriptMouseButtonNames(t *testing.T) {
	tests := map[ebiten.MouseButton]string{
		ebiten.MouseButtonLeft:   "LeftClick",
		ebiten.MouseButtonMiddle: "MiddleClick",
		ebiten.MouseButtonRight:  "RightClick",
		ebiten.MouseButton3:      "Mouse4",
		ebiten.MouseButton4:      "Mouse5",
	}
	for button, want := range tests {
		if got := mouseButtonName(button); got != want {
			t.Errorf("mouseButtonName(%d) = %q, want %q", button, got, want)
		}
	}
}

func TestStructuredMouseChord(t *testing.T) {
	event := makeScriptInputEvent("Meta-Ctrl-K-Mouse4")
	if event.Chord != "Meta-Ctrl-K-Mouse4" || event.Key != "K" || event.Button != "Mouse4" {
		t.Fatalf("unexpected chord fields: %+v", event)
	}
	if !event.Meta || !event.Ctrl || event.Alt || event.Shift {
		t.Fatalf("unexpected modifiers: %+v", event)
	}
	if !event.Continues() {
		t.Fatal("input should pass through by default")
	}
	event.Consume()
	if event.Continues() || !scriptInputConsumesButton(event, "Mouse4") {
		t.Fatal("Consume did not suppress the matching mouse button")
	}
	event.Pass()
	if !event.Continues() {
		t.Fatal("Pass did not restore normal input")
	}
}

func TestPrintableBindingsAreReservedOnlyForOtherTextFields(t *testing.T) {
	if !shouldBlockPrintableCombo([]string{"A"}) || !shouldBlockPrintableCombo([]string{"#"}) {
		t.Fatal("unmodified printable bindings should not fire in another text field")
	}
	if shouldBlockPrintableCombo([]string{"Ctrl", "A"}) || shouldBlockPrintableCombo([]string{"F12"}) {
		t.Fatal("modified or non-printable bindings should remain available")
	}
}
