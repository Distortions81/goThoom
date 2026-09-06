package main

import (
	"testing"

	"gothoom/eui"
)

func TestSimplePopupsDoNotRequireOuterScrollbars(t *testing.T) {
	initFont()
	originalScale := eui.UIScale()
	originalWidth, originalHeight := eui.ScreenSize()
	t.Cleanup(func() {
		eui.SetUIScale(originalScale)
		eui.SetScreenSize(originalWidth, originalHeight)
	})

	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		eui.SetUIScale(scale)
		eui.SetScreenSize(1200, 800)
		win := eui.ShowPopup(
			"Confirm Quit",
			"Are you sure you would like to quit?",
			[]eui.PopupButton{{Text: "Cancel"}, {Text: "Quit"}},
		)
		if !win.NoScroll {
			win.RemoveWindow()
			t.Fatalf("popup allowed an outer scrollbar at %.2fx", scale)
		}
		horizontal, vertical := win.RequiresScroll()
		win.RemoveWindow()
		if horizontal || vertical {
			t.Fatalf("popup requires scrollbars at %.2fx: horizontal=%t vertical=%t", scale, horizontal, vertical)
		}
	}
}
