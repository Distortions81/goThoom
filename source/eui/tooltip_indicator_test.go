package eui

import "testing"

func TestSliderTooltipIndicatorRequiresLabel(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	win := NewWindow()
	win.ShowTooltipIndicators = true
	slider, _ := NewSlider()
	slider.Size = Point{X: 180, Y: 24}
	slider.SetTooltip("Adjust the enhancement strength.")
	win.AddItem(slider)
	if got := slider.tooltipIndicatorRect(Point{}, slider.GetSize()); got != (rect{}) {
		t.Fatalf("unlabeled slider has an indicator over its track: %v", got)
	}
	if !shouldDrawTooltip(slider, nil, nil) {
		t.Fatal("unlabeled slider lost its hover help")
	}
	slider.Label = "Enhancement strength"
	if got := slider.tooltipIndicatorRect(Point{}, slider.GetSize()); got.X1 <= got.X0 {
		t.Fatal("labeled slider lost its caption indicator")
	}
}
