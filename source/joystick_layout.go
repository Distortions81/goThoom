package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// joystickUsesStandardLayout keeps common controllers portable while retaining
// the existing raw controls for hardware without an SDL/Web-standard mapping.
func joystickUsesStandardLayout(id ebiten.GamepadID) bool {
	return gs.JoystickUseStandardLayout && ebiten.IsStandardGamepadLayoutAvailable(id)
}

func joystickClickButton(action string) (ebiten.StandardGamepadButton, bool) {
	switch action {
	case "click1":
		return ebiten.StandardGamepadButtonRightBottom, true
	case "click2":
		return ebiten.StandardGamepadButtonRightRight, true
	case "click3":
		return ebiten.StandardGamepadButtonRightLeft, true
	default:
		return 0, false
	}
}

func joystickClickJustPressed(id ebiten.GamepadID, action string) bool {
	if joystickUsesStandardLayout(id) {
		if button, ok := joystickClickButton(action); ok {
			return inpututil.IsStandardGamepadButtonJustPressed(id, button)
		}
	}
	button, ok := gs.JoystickBindings[action]
	return ok && inpututil.IsGamepadButtonJustPressed(id, button)
}

// joystickStickValues returns standard left/right sticks first. For an
// unmapped controller, it preserves the old paired-raw-axis configuration.
func joystickStickValues(id ebiten.GamepadID, stick int) (float64, float64, bool) {
	if joystickUsesStandardLayout(id) {
		switch stick {
		case 0:
			return ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal), ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical), true
		case 1:
			return ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisRightStickHorizontal), ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisRightStickVertical), true
		default:
			return 0, 0, false
		}
	}
	axis := stick * 2
	if stick < 0 || axis+1 >= ebiten.GamepadAxisCount(id) {
		return 0, 0, false
	}
	return ebiten.GamepadAxisValue(id, axis), ebiten.GamepadAxisValue(id, axis+1), true
}
