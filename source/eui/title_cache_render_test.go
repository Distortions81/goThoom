package eui

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font/gofont/goregular"
)

func TestRenderWindowTitleCache(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_TITLE_CACHE") == "" {
		t.Skip("set GOTHOOM_RENDER_TITLE_CACHE=1")
	}
	if err := EnsureFontSource(goregular.TTF); err != nil {
		t.Fatal(err)
	}
	game := &windowTitleCacheGame{}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

type windowTitleCacheGame struct {
	done bool
	err  error
}

func (*windowTitleCacheGame) Layout(_, _ int) (int, int) { return 360, 120 }

func (game *windowTitleCacheGame) Update() error {
	if game.done {
		return ebiten.Termination
	}
	return nil
}

func (game *windowTitleCacheGame) Draw(_ *ebiten.Image) {
	if game.done {
		return
	}
	game.done = true

	win := NewWindow()
	defer win.deallocate()
	win.Title = "Players"
	win.Position = point{X: 17, Y: 11}
	win.Size = point{X: 320, Y: 100}
	win.TitleHeight = 28
	win.Closable = true
	win.Maximizable = true
	win.Searchable = true
	win.Movable = true
	win.ShowDragbar = true

	first := ebiten.NewImage(360, 120)
	second := ebiten.NewImage(360, 120)
	defer first.Deallocate()
	defer second.Deallocate()
	win.drawWinTitle(first)
	if win.titleRenderCount != 1 {
		game.err = fmt.Errorf("first title render count = %d, want 1", win.titleRenderCount)
		return
	}

	// A content repaint must composite the existing title texture instead of
	// rebuilding its text, buttons, and dragbar marks.
	win.markDirty()
	win.drawWinTitle(second)
	if win.titleRenderCount != 1 {
		game.err = fmt.Errorf("content repaint title render count = %d, want 1", win.titleRenderCount)
		return
	}
	firstPixels := make([]byte, 360*120*4)
	secondPixels := make([]byte, len(firstPixels))
	first.ReadPixels(firstPixels)
	second.ReadPixels(secondPixels)
	if !bytes.Equal(firstPixels, secondPixels) {
		game.err = fmt.Errorf("cached title pixels changed during content repaint")
		return
	}

	win.Title = "Players (32)"
	win.drawWinTitle(second)
	if win.titleRenderCount != 2 {
		game.err = fmt.Errorf("changed title render count = %d, want 2", win.titleRenderCount)
	}
}
