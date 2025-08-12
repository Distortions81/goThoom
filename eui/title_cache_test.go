//go:build test

package eui

import (
	"bytes"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// helper to ensure font source for title rendering
func ensureFont(t *testing.T) {
	if mplusFaceSource == nil {
		s, err := text.NewGoTextFaceSource(bytes.NewReader(notoTTF))
		if err != nil {
			t.Fatalf("font init: %v", err)
		}
		mplusFaceSource = s
	}
}

func TestSetTitleUpdatesRender(t *testing.T) {
	ensureFont(t)

	win := *defaultTheme
	win.Theme = baseTheme
	win.Title = "before"
	win.Size = point{X: 100, Y: 100}
	windows = nil
	win.Open()

	screen := ebiten.NewImage(200, 200)
	win.Dirty = true
	win.Draw(screen)
	buf1 := make([]byte, 4*win.Render.Bounds().Dx()*win.Render.Bounds().Dy())
	win.Render.ReadPixels(buf1)

	win.SetTitle("after")
	if !win.Dirty {
		t.Fatalf("expected window marked dirty after SetTitle")
	}
	win.Draw(screen)
	buf2 := make([]byte, 4*win.Render.Bounds().Dx()*win.Render.Bounds().Dy())
	win.Render.ReadPixels(buf2)

	if bytes.Equal(buf1, buf2) {
		t.Fatalf("expected cached image to change after title update")
	}
}

func TestSetTitleSizeUpdatesRender(t *testing.T) {
	ensureFont(t)

	win := *defaultTheme
	win.Theme = baseTheme
	win.Title = "title"
	win.Size = point{X: 100, Y: 100}
	windows = nil
	win.Open()

	screen := ebiten.NewImage(200, 200)
	win.Dirty = true
	win.Draw(screen)
	buf1 := make([]byte, 4*win.Render.Bounds().Dx()*win.Render.Bounds().Dy())
	win.Render.ReadPixels(buf1)

	win.SetTitleSize(win.GetTitleSize() + 10)
	if !win.Dirty {
		t.Fatalf("expected window marked dirty after SetTitleSize")
	}
	win.Draw(screen)
	buf2 := make([]byte, 4*win.Render.Bounds().Dx()*win.Render.Bounds().Dy())
	win.Render.ReadPixels(buf2)

	if bytes.Equal(buf1, buf2) {
		t.Fatalf("expected cached image to change after title size update")
	}
}
