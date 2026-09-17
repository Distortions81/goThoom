package eui

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// Run alone; Ebitengine permits only one RunGame per process.
func TestRenderTextEditing(t *testing.T) {
	path := os.Getenv("GOTHOOM_TEXT_EDIT_CAPTURE")
	if path == "" {
		t.Skip("set GOTHOOM_TEXT_EDIT_CAPTURE to a PNG path")
	}
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	g := &textEditingRenderGame{path: path}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type textEditingRenderGame struct {
	done bool
	path string
	err  error
}

func (g *textEditingRenderGame) Layout(_, _ int) (int, int) { return 840, 640 }
func (g *textEditingRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *textEditingRenderGame) Draw(screen *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	g.err = g.render(screen)
}
func (g *textEditingRenderGame) render(screen *ebiten.Image) error {
	updateNow = time.Now()
	screen.Fill(color.RGBA{R: 24, G: 28, B: 36, A: 255})
	for i, scale := range []float32{1, 1.5, 2} {
		uiScale = scale
		baseY := float32(15 + i*205)
		input, _ := NewInput()
		input.Text = "Name: café and selected text"
		input.Label = fmt.Sprintf("Single line / %.1fx", scale)
		input.Size = point{X: 380 / scale, Y: 25}
		input.Focused = true
		input.editSelect(6, 10)
		var dropdowns []openDropdown
		input.drawItem(nil, point{X: 15, Y: baseY}, point{}, rect{X1: 840, Y1: 640}, screen, &dropdowns)
		area, _ := NewTextArea()
		area.Text = "func greet() {\n\tprint(\"Hello!\")\n\n\t// Another line\n}\n"
		area.Size = point{X: 400 / scale, Y: 185 / scale}
		area.Focused = true
		area.editMove(0, false)
		baseline := ebiten.NewImageFromImage(screen)
		area.drawItem(nil, point{X: 425, Y: baseY}, point{}, rect{X1: 840, Y1: 640}, screen, &dropdowns)
		// Multiline editing keeps a neutral document background while focused.
		background := area.Color
		if background == (Color{}) {
			background = area.themeStyle().Color
		}
		r, g, b, _ := screen.At(805, int(baseY+170)).RGBA()
		if uint8(r>>8) != background.R || uint8(g>>8) != background.G || uint8(b>>8) != background.B {
			return fmt.Errorf("scale %v: focused text area changed document background", scale)
		}
		// Caret drawing must invalidate pixels only within the text viewport.
		before := make([]byte, 840*640*4)
		screen.ReadPixels(before)
		area.editor().caretOn = false
		screen.Clear()
		screen.DrawImage(baseline, nil)
		area.drawItem(nil, point{X: 425, Y: baseY}, point{}, rect{X1: 840, Y1: 640}, screen, &dropdowns)
		after := make([]byte, len(before))
		screen.ReadPixels(after)
		if bytes.Equal(before, after) {
			return fmt.Errorf("scale %v: caret was not drawn", scale)
		}
		viewport, _ := area.editGeometry()
		for p := 0; p < len(before); p += 4 {
			if bytes.Equal(before[p:p+4], after[p:p+4]) {
				continue
			}
			x, y := (p/4)%840, (p/4)/840
			if float32(x) < viewport.X0-1 || float32(x) > viewport.X1 || float32(y) < viewport.Y0-1 || float32(y) > viewport.Y1 {
				return fmt.Errorf("caret escaped viewport at %d,%d", x, y)
			}
		}
		area.editor().caretOn = true
		area.editSelect(15, 31)
		screen.Clear()
		screen.DrawImage(baseline, nil)
		baseline.Deallocate()
		area.drawItem(nil, point{X: 425, Y: baseY}, point{}, rect{X1: 840, Y1: 640}, screen, &dropdowns)
		long, _ := NewInput()
		long.Text = "A long field that scrolls horizontally to keep this caret visible"
		long.Size = point{X: 380 / scale, Y: 25}
		long.Focused = true
		long.editMove(len([]rune(long.Text)), false)
		long.drawItem(nil, point{X: 15, Y: baseY + 95}, point{}, rect{X1: 840, Y1: 640}, screen, &dropdowns)
		if long.editor().scroll.X <= 0 {
			return fmt.Errorf("long field did not scroll")
		}
		_, origin := long.editGeometry()
		_, cx := long.editLayout().caret(long.CursorPos)
		if got := long.editCursorAt(point{X: origin.X + cx, Y: origin.Y + 3}); got != long.CursorPos {
			return fmt.Errorf("scrolled caret hit %d, want %d", got, long.CursorPos)
		}
	}
	f, err := os.Create(g.path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, screen)
}
