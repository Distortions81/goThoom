package eui

import (
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"testing"
)

func TestSliderConstrainedWidth(t *testing.T) {
	uiScale = 1
	item := &itemData{
		ItemType: ITEM_SLIDER,
		MinValue: 0,
		MaxValue: 1,
		Value:    0,
		AuxSize:  point{X: 10, Y: 10},
		FontSize: 12,
	}
	item.DrawRect = rect{X0: 0, Y0: 0, X1: 30, Y1: 20}

	textSize := (item.FontSize * uiScale) + 2
	face := textFace(textSize)
	maxW, _ := text.Measure(sliderMaxLabel, face, 0)
	gap := currentStyle.SliderValueGap
	knobW := item.AuxSize.X * uiScale

	trackWidth := item.DrawRect.X1 - item.DrawRect.X0 - knobW - gap - float32(maxW)
	showValue := true
	if trackWidth < knobW {
		trackWidth = item.DrawRect.X1 - item.DrawRect.X0 - knobW
		showValue = false
		if trackWidth < 0 {
			trackWidth = 0
		}
	}
	if trackWidth <= 0 {
		t.Fatalf("trackWidth <= 0: %f", trackWidth)
	}
	if showValue {
		t.Fatalf("expected value label hidden for narrow slider")
	}

	mpos := point{X: item.DrawRect.X0 + knobW + trackWidth, Y: 0}
	item.setSliderValue(mpos)
	if item.Value != item.MaxValue {
		t.Fatalf("expected value to be MaxValue, got %f", item.Value)
	}
}
