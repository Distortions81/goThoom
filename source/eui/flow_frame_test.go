package eui

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestFramedFlowClipsScrollingRowsInsideBorder(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	oldScale := UIScale()
	t.Cleanup(func() { SetUIScale(oldScale) })
	for _, scale := range []float32{1, 2} {
		SetUIScale(scale)
		row := NewColumn()
		row.Fixed, row.Filled = true, true
		row.Size = point{X: 100, Y: 100}
		flow := NewColumn(row)
		flow.Fixed, flow.Scrollable, flow.Outlined = true, true, true
		flow.Size, flow.Scroll = point{X: 100, Y: 40}, point{Y: 20 * scale}
		flow.Border, flow.OutlineColor = 2, ColorGray
		win := NewWindow()
		win.AddItem(flow)
		screen := ebiten.NewImage(300, 300)
		flow.drawFlows(win, nil, point{X: 10, Y: 20}, point{}, rect{X1: 300, Y1: 300}, screen, new([]openDropdown))
		border := 2 * scale
		if row.DrawRect.X0 != 10+border || row.DrawRect.Y0 != 20+border || row.DrawRect.Y1 != 20+40*scale-border {
			t.Fatalf("%gx scrolling row draws or receives input over its frame: %+v", scale, row.DrawRect)
		}
		screen.Deallocate()
		win.RemoveWindow()
	}
}
