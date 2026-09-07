// Package inputkeys keeps desktop editing shortcuts consistent across the UI.
package inputkeys

import (
	"runtime"

	"github.com/hajimehoshi/ebiten/v2"
)

type Modifiers struct {
	Control, Alt, Meta bool
	Mac                bool
}

func Current() Modifiers {
	return Modifiers{
		Control: ebiten.IsKeyPressed(ebiten.KeyControl),
		Alt:     ebiten.IsKeyPressed(ebiten.KeyAlt),
		Meta:    ebiten.IsKeyPressed(ebiten.KeyMeta),
		Mac:     runtime.GOOS == "darwin",
	}
}

// Shortcut retains existing Control shortcuts and adds Command on macOS.
func (m Modifiers) Shortcut() bool { return !m.Alt && (m.Control || m.Mac && m.Meta) }
func (m Modifiers) Word() bool     { return m.Control || m.Mac && m.Alt }
func (m Modifiers) Line() bool     { return m.Mac && m.Meta }

// Option is a text-producing modifier on Mac keyboards.
func (m Modifiers) SuppressText() bool { return m.Control && !m.Alt || m.Meta }

func ShortcutLabel() string {
	if runtime.GOOS == "darwin" {
		return "Cmd"
	}
	return "Ctrl"
}
