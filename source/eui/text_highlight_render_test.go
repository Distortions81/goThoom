package eui

import (
	"bytes"
	"fmt"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
)

// Called by the existing opt-in text editing render harness. Pixel equality
// checks exercise the real glyph renderer, including shaping and clipping.
func checkTextHighlightRendering() error {
	for _, scale := range []float32{1, 1.5, 2} {
		uiScale = scale
		area, _ := NewTextArea()
		area.Text = "\tAV café e\u0301 ffi\n\tβeta שלום second line with horizontal overflow\nthird line"
		area.Size = point{X: 220, Y: 80}
		area.Focused = true
		area.CursorPos = 4
		size := area.GetSize()
		clip := rect{X0: 9, Y0: 7, X1: 260, Y1: 130}
		n := utf8.RuneCountInString(area.Text)
		white, blue := ColorWhite, NewColor(35, 75, 130, 255)
		plain, colored := ebiten.NewImage(280, 150), ebiten.NewImage(280, 150)
		render := func(dst *ebiten.Image) []byte {
			dst.Clear()
			area.drawEditableText(dst, point{X: 9, Y: 7}, size, clip, white, blue)
			pixels := make([]byte, 280*150*4)
			dst.ReadPixels(pixels)
			return pixels
		}
		for _, scroll := range []point{{}, {X: 27.5, Y: 8.5}} {
			for _, selected := range []bool{false, true} {
				state := area.editor()
				state.followCaret, state.scroll = false, scroll
				area.SelectStart, area.SelectEnd = 0, 0
				if selected {
					area.SelectEnd = n
				}
				area.SetTextHighlighter(nil)
				before := render(plain)
				layout := area.editLayout()
				area.SetTextHighlighter(func(string) []TextColorSpan { return []TextColorSpan{{0, n, white}} })
				after := render(colored)
				if !bytes.Equal(before, after) || area.editLayout() != layout {
					return fmt.Errorf("highlight changed glyph geometry: scale=%g scroll=%v selected=%t", scale, scroll, selected)
				}
				area.SetTextHighlighter(func(string) []TextColorSpan { return []TextColorSpan{{0, n, NewColor(255, 100, 80, 255)}} })
				after = render(colored)
				if selected {
					if !bytes.Equal(before, after) {
						return fmt.Errorf("syntax colors overrode selection: scale=%g scroll=%v", scale, scroll)
					}
					continue
				}
				if bytes.Equal(before, after) {
					return fmt.Errorf("highlight produced no color: scale=%g scroll=%v", scale, scroll)
				}
				for i := 0; i < len(before); i += 4 {
					if before[i+3] != after[i+3] {
						return fmt.Errorf("highlight changed glyph coverage at pixel %d, scale=%g", i/4, scale)
					}
					if bytes.Equal(before[i:i+4], after[i:i+4]) {
						continue
					}
					viewport := intersectRect(clip, area.editViewport(point{X: 9, Y: 7}, size))
					x, y := float32((i/4)%280), float32((i/4)/280)
					if x < viewport.X0-1 || y < viewport.Y0-1 || x >= viewport.X1 || y >= viewport.Y1 {
						return fmt.Errorf("highlight escaped viewport at %g,%g", x, y)
					}
				}
			}
		}
		plain.Deallocate()
		colored.Deallocate()
	}
	return nil
}
