package main

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestJoystickClickButtonUsesPortableFaceButtons(t *testing.T) {
	tests := map[string]ebiten.StandardGamepadButton{
		"click1": ebiten.StandardGamepadButtonRightBottom,
		"click2": ebiten.StandardGamepadButtonRightRight,
		"click3": ebiten.StandardGamepadButtonRightLeft,
	}
	for action, want := range tests {
		got, ok := joystickClickButton(action)
		if !ok || got != want {
			t.Errorf("joystickClickButton(%q) = %v, %v; want %v, true", action, got, ok, want)
		}
	}
	if _, ok := joystickClickButton("unknown"); ok {
		t.Fatal("unknown action unexpectedly has a standard button")
	}
}
