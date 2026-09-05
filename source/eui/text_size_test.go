package eui

import (
	"testing"

	"golang.org/x/image/font/gofont/goregular"
)

func TestAutosizedTextPreservesBlankLinesAndRefreshesMetrics(t *testing.T) {
	if err := EnsureFontSource(goregular.TTF); err != nil {
		t.Fatal(err)
	}
	previousScale := uiScale
	t.Cleanup(func() { uiScale = previousScale })
	for _, scale := range []float32{1, 1.5, 2} {
		uiScale = scale
		item := &itemData{ItemType: ITEM_TEXT, FontSize: 12, Text: "Wide text"}
		line := item.GetSize()
		item.Text = "Wide text\n\n"
		if got := item.GetSize(); got.X != line.X || got.Y != 3*line.Y {
			t.Fatalf("scale %v: trailing blank lines changed dimensions: %v, single line %v", scale, got, line)
		}
		item.Text = ""
		if got := item.GetSize(); got.Y != line.Y || got.X <= 0 {
			t.Fatalf("scale %v: empty text lost its line: %v", scale, got)
		}
		item.Text = "Wide text"
		item.FontSize = 24
		if got := item.GetSize(); got.X <= line.X || got.Y <= line.Y {
			t.Fatalf("scale %v: font size change retained stale metrics: %v", scale, got)
		}
	}
}
