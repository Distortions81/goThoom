package eui

import (
	"runtime"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"golang.org/x/time/rate"
)

var touchIDs []ebiten.TouchID

var (
	isWasm         = runtime.GOOS == "js" && runtime.GOARCH == "wasm"
	touchScrolling bool
	prevTouchAvg   = point{}
	wheelLimiter   = rate.NewLimiter(rate.Every(125*time.Millisecond), 1)
)

const touchScrollScale = 0.05

// pointerPosition returns the current pointer position.
// If a touch is active, the first touch is used. Otherwise the mouse cursor position is returned.
func pointerPosition() (int, int) {
	touchIDs = ebiten.AppendTouchIDs(touchIDs[:0])
	var x, y int
	if len(touchIDs) > 0 {
		x, y = ebiten.TouchPosition(touchIDs[0])
	} else {
		x, y = ebiten.CursorPosition()
	}
	return x, y
}

// pointerWheel returns the wheel delta for mouse or two-finger touch scrolling.
func pointerWheel() (float64, float64) {
	touchIDs = ebiten.AppendTouchIDs(touchIDs[:0])
	if len(touchIDs) >= 2 {
		// Average the first two touches to emulate wheel scrolling.
		x0, y0 := ebiten.TouchPosition(touchIDs[0])
		x1, y1 := ebiten.TouchPosition(touchIDs[1])
		avgX := float64(x0+x1) / 2
		avgY := float64(y0+y1) / 2

		if !touchScrolling {
			touchScrolling = true
			prevTouchAvg = point{X: float32(avgX), Y: float32(avgY)}
			return 0, 0
		}

		// Reverse the scroll direction so dragging two fingers up moves
		// content up just like a mouse wheel. This provides a more
		// natural feel on touch devices.
		dx := (avgX - float64(prevTouchAvg.X)) * touchScrollScale
		dy := (avgY - float64(prevTouchAvg.Y)) * touchScrollScale
		prevTouchAvg = point{X: float32(avgX), Y: float32(avgY)}
		return dx, dy
	}

	touchScrolling = false

	wx, wy := ebiten.Wheel()
	if isWasm {
		if !wheelLimiter.Allow() {
			return 0, 0
		}

		// Limit scroll events to +/-3 for a consistent feel in browsers
		if wx > 0 {
			wx = 3
		} else if wx < 0 {
			wx = -3
		}
		if wy > 0 {
			wy = 3
		} else if wy < 0 {
			wy = -3
		}
	}
	return wx, wy
}

// pointerJustPressed reports whether the primary pointer was just pressed.
func pointerJustPressed() bool {
	touchIDs = ebiten.AppendTouchIDs(touchIDs[:0])
	if len(touchIDs) > 1 {
		return false
	}
	if len(inpututil.AppendJustPressedTouchIDs(nil)) > 0 {
		return true
	}
	return inpututil.IsMouseButtonJustPressed(ebiten.MouseButton0)
}

// pointerPressed reports whether the primary pointer is currently pressed.
func pointerPressed() bool {
	touchIDs = ebiten.AppendTouchIDs(touchIDs[:0])
	if len(touchIDs) > 1 {
		return false
	}
	if len(touchIDs) == 1 {
		return true
	}
	return ebiten.IsMouseButtonPressed(ebiten.MouseButton0)
}

// pointerPressDuration returns how long the primary pointer has been pressed.
func pointerPressDuration() int {
	touchIDs = ebiten.AppendTouchIDs(touchIDs[:0])
	if len(touchIDs) > 1 {
		return 0
	}
	if len(touchIDs) == 1 {
		return inpututil.TouchPressDuration(touchIDs[0])
	}
	return inpututil.MouseButtonPressDuration(ebiten.MouseButton0)
}
