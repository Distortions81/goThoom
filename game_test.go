package main

import (
	"math"
	"testing"
	"time"

	"gothoom/eui"
)

func TestStopWalkIfOutside(t *testing.T) {
	old := gs.ClickToToggle
	gs.ClickToToggle = true
	walkToggled = true
	stopWalkIfOutside(true, false)
	if walkToggled {
		t.Fatalf("walkToggled should be false after outside click")
	}

	walkToggled = true
	stopWalkIfOutside(true, true)
	if !walkToggled {
		t.Fatalf("walkToggled should remain true when clicking inside game")
	}

	walkToggled = true
	stopWalkIfOutside(false, false)
	if !walkToggled {
		t.Fatalf("walkToggled should remain true when not clicking")
	}

	gs.ClickToToggle = old
}

func TestContinueHeldWalk(t *testing.T) {
	prev := inputState{mouseDown: true}
	if !continueHeldWalk(prev, false, true, 0, false) {
		t.Fatalf("walk should continue when mouse is held outside")
	}
	if continueHeldWalk(prev, false, false, 0, false) {
		t.Fatalf("walk should stop when mouse button is released")
	}
	if !continueHeldWalk(inputState{}, true, true, 2, false) {
		t.Fatalf("walk should start when mouse is held inside game")
	}
}

func TestWorldDrawInfoSubtractsTitleHeight(t *testing.T) {
	oldGameWin := gameWin
	oldScale := gs.GameScale
	defer func() {
		gameWin = oldGameWin
		gs.GameScale = oldScale
	}()

	gs.GameScale = 1
	gameWin = eui.NewWindow()
	gameWin.NoScale = true
	gameWin.Position = eui.Point{}
	gameWin.Size = eui.Point{X: gameAreaSizeX + 4, Y: gameAreaSizeY + 4 + 20}
	gameWin.Margin = 0
	gameWin.Border = 0
	gameWin.BorderPad = 0
	gameWin.Padding = 0
	gameWin.TitleHeight = 20

	x, y, scale := worldDrawInfo()
	if x != 2 || y != 22 {
		t.Fatalf("world origin got (%d,%d), want (2,22)", x, y)
	}
	wantScale := float64(gameAreaSizeX-1) / float64(gameAreaSizeX)
	if math.Abs(scale-wantScale) > 0.000001 {
		t.Fatalf("world scale got %v, want %v", scale, wantScale)
	}
}

func TestUpdateGameImageSizeKeepsVisibleImageExactWhenShrinking(t *testing.T) {
	oldGameWin := gameWin
	oldGameImageItem := gameImageItem
	oldGameImage := gameImage
	oldGameImageBacking := gameImageBacking
	oldScale := eui.UIScale()
	defer func() {
		gameWin = oldGameWin
		gameImageItem = oldGameImageItem
		gameImage = oldGameImage
		gameImageBacking = oldGameImageBacking
		eui.SetUIScale(oldScale)
	}()

	eui.SetUIScale(1)
	gameImageItem = nil
	gameImage = nil
	gameImageBacking = nil
	gameWin = eui.NewWindow()
	gameWin.NoScale = true
	gameWin.Padding = 0
	gameWin.Border = 0
	gameWin.BorderPad = 0
	gameWin.Margin = 0
	gameWin.TitleHeight = 0

	gameWin.Size = eui.Point{X: 104, Y: 84}
	updateGameImageSize()
	if gameImage == nil {
		t.Fatalf("initial visible image is nil")
	}
	if gameImage.Bounds().Dx() != 100 || gameImage.Bounds().Dy() != 80 {
		t.Fatalf("initial visible image got %v, want 100x80", gameImage.Bounds())
	}
	backing := gameImageBacking

	gameWin.Size = eui.Point{X: 84, Y: 64}
	updateGameImageSize()
	if gameImageBacking != backing {
		t.Fatalf("backing image was reallocated while shrinking")
	}
	if gameImage == nil {
		t.Fatalf("shrunk visible image is nil")
	}
	if gameImage.Bounds().Dx() != 80 || gameImage.Bounds().Dy() != 60 {
		t.Fatalf("shrunk visible image got %v, want 80x60", gameImage.Bounds())
	}
	if gameImageItem.Image != gameImage {
		t.Fatalf("game image item does not use current visible subimage")
	}
}

func TestAltNetDelay(t *testing.T) {
	full := 100 * time.Millisecond
	var start time.Time

	if d, s := altNetDelay(1, start, time.Now(), full); d != 0 || !s.IsZero() {
		t.Fatalf("frame 1 got delay %v start %v", d, s)
	}

	now := time.Now()
	if d, s := altNetDelay(3, start, now, full); d != 0 || s.IsZero() {
		t.Fatalf("frame 3 got delay %v start %v", d, s)
	} else {
		start = s
	}

	half := start.Add(1500 * time.Millisecond)
	if d, _ := altNetDelay(4, start, half, full); d < 49*time.Millisecond || d > 51*time.Millisecond {
		t.Fatalf("half ramp got %v", d)
	}

	end := start.Add(3 * time.Second)
	if d, _ := altNetDelay(10, start, end, full); d != full {
		t.Fatalf("end ramp got %v want %v", d, full)
	}
}
