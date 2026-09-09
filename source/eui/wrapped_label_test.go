package eui

import (
	"strings"
	"testing"
)

func TestWrappedLabelReflowsForWidthScaleAndContent(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	originalScale := UIScale()
	t.Cleanup(func() { SetUIScale(originalScale) })
	label := NewWrappedLabel("Preferences apply to every character using this script.\nChanges save immediately.", 150)
	for _, scale := range []float32{1, 1.5, 2} {
		SetUIScale(scale)
		size := label.GetSize()
		face := textFace(label.FontSize*scale + 2)
		for _, line := range strings.Split(label.Text, "\n") {
			if width := MeasureTextWidth(line, face); width > float64(size.X) {
				t.Fatalf("line width %.1f exceeds label %.1f at %.1fx", width, size.X, scale)
			}
		}
		if size.Y <= 2*(label.FontSize*scale+2) {
			t.Fatal("wrapped lines did not increase content height")
		}
	}
	label.Size.X = 1000
	label.GetSize()
	if strings.Count(label.Text, "\n") != 1 {
		t.Fatal("widening kept automatic line breaks or lost original paragraph break")
	}
	label.SetWrappedText("Saved.")
	if label.Text != "Saved." {
		t.Fatal("content update retained old wrapped text")
	}
}
