// A standalone EUI application: no goThoom resources or settings are needed.
package main

import (
	"image/color"
	"log"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
)

type game struct{}

func (*game) Update() error { return eui.Update() }
func (*game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 24, G: 28, B: 36, A: 255})
	eui.Draw(screen)
}
func (*game) Layout(w, h int) (int, int) { return eui.Layout(w, h) }

func main() {
	if err := eui.Init(); err != nil {
		log.Fatal(err)
	}
	win := eui.NewWindow()
	win.Title = "EUI example"
	win.AutoSize, win.Movable, win.Closable = true, true, true
	label := eui.NewLabel("A UI with no external assets")
	button := eui.NewActionButton("Open dialog", func() {
		eui.ShowPopup("Hello", "This dialog belongs entirely to EUI.", []eui.PopupButton{{Text: "OK"}})
	})
	swatch := eui.NewColorSwatch("Accent", eui.AccentColor(), nil, func(c eui.Color) {
		label.Text = "Color selected"
		win.Refresh()
	})
	input, _ := eui.NewInput()
	input.Label = "Name"
	input.Text = "Select and replace this text"
	input.Size.X = 440
	area, _ := eui.NewTextArea()
	area.Label = "Text editing"
	area.Size = eui.Point{X: 440, Y: 220}
	area.AcceptTab = true
	area.Text = "// Try selecting, indenting, and undoing.\nfunc greet() {\n\tprint(\"Hello!\")\n}\n"
	win.AddItem(eui.NewColumn(label, input, area,
		eui.NewLabel("Ctrl/Command+Z: undo. Ctrl+Tab: leave the editor."), button, swatch))
	win.MarkOpen()
	ebiten.SetWindowSize(800, 600)
	ebiten.SetWindowTitle("EUI basic example")
	if err := ebiten.RunGame(&game{}); err != nil {
		log.Fatal(err)
	}
}
