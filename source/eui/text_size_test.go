package eui

import (
	"math"
	"testing"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
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

func TestUnconstrainedTextGrowsBeyondThemeDefault(t *testing.T) {
	if err := EnsureFontSource(goregular.TTF); err != nil {
		t.Fatal(err)
	}
	previousScale := uiScale
	uiScale = 1
	t.Cleanup(func() { uiScale = previousScale })

	item := &itemData{
		ItemType: ITEM_TEXT,
		FontSize: 12,
		Size:     point{X: 128, Y: 24},
		Text:     "Incorrect password. Please try again.",
	}
	width, _ := text.Measure(item.Text, itemFace(item, item.FontSize*uiScale+2), 0)
	if got := item.GetSize().X; got < float32(math.Ceil(width)) {
		t.Fatalf("unconstrained text width = %v, want at least measured width %v", got, width)
	}

	item.ConstrainToSize = true
	if got := item.GetSize().X; got != 128 {
		t.Fatalf("constrained text width = %v, want declared width 128", got)
	}
}

func TestItemFaceHandlesTypedNilFace(t *testing.T) {
	if err := EnsureFontSource(goregular.TTF); err != nil {
		t.Fatal(err)
	}
	item := &itemData{Face: (*text.GoTextFace)(nil)}
	if face := itemFace(item, 14); face == nil {
		t.Fatal("typed nil item face did not fall back to the font cache")
	}
}
