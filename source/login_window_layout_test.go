package main

import (
	"math"
	"testing"

	"gothoom/eui"
)

func TestLoginWindowStartsCentered(t *testing.T) {
	initFont()
	originalWindow := loginWin
	originalWidth, originalHeight := eui.ScreenSize()
	loginWin = nil
	eui.SetScreenSize(1200, 800)
	t.Cleanup(func() {
		if loginWin != nil {
			loginWin.RemoveWindow()
		}
		loginWin = originalWindow
		eui.SetScreenSize(originalWidth, originalHeight)
	})

	makeLoginWindow()
	pos := loginWin.GetPos()
	size := loginWin.GetSize()
	centerX := pos.X + size.X/2
	centerY := pos.Y + size.Y/2
	if math.Abs(float64(centerX-600)) > 0.5 || math.Abs(float64(centerY-400)) > 0.5 {
		t.Fatalf("login center = (%.1f, %.1f), want (600, 400)", centerX, centerY)
	}
}
