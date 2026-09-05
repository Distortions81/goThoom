package eui

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font/gofont/goregular"
)

func BenchmarkUIDrawClippedTextList(b *testing.B) {
	if err := EnsureFontSource(goregular.TTF); err != nil {
		b.Fatal(err)
	}
	win := NewWindow()
	flow := &itemData{ItemType: ITEM_FLOW, FlowType: FLOW_VERTICAL, Size: point{X: 300, Y: 200}}
	for range 1000 {
		flow.Contents = append(flow.Contents, &itemData{
			ItemType: ITEM_TEXT, Text: "Player status\nAdditional information", FontSize: 12,
			Position: point{X: 1000},
		})
	}
	screen := ebiten.NewImage(300, 200)
	b.Cleanup(func() { screen.Deallocate() })
	var dropdowns []openDropdown
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		flow.drawFlows(win, flow, point{}, point{}, rect{X1: 300, Y1: 200}, screen, &dropdowns)
	}
}
