package main

import (
	"image/color"
	"math"
	"testing"
	"time"

	"gothoom/climg"
	"gothoom/eui"
)

func TestPlayfieldBackgroundColorUsesGammaCorrection(t *testing.T) {
	oldImages := clImages
	images := &climg.CLImages{}
	clImages = images
	t.Cleanup(func() { clImages = oldImages })

	if got, want := playfieldBackgroundColor(), (color.RGBA{R: 0x88, G: 0x88, B: 0x88, A: 0xff}); got != want {
		t.Fatalf("uncorrected background got %v, want %v", got, want)
	}

	images.SetGammaCorrection(true, 1.8, 2.2)
	if got, want := playfieldBackgroundColor(), (color.RGBA{R: 0x98, G: 0x98, B: 0x98, A: 0xff}); got != want {
		t.Fatalf("corrected background got %v, want %v", got, want)
	}
}

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
	oldTiledWindows := gs.TiledWindows
	defer func() {
		gameWin = oldGameWin
		gs.GameScale = oldScale
		gs.TiledWindows = oldTiledWindows
	}()

	gs.GameScale = 1
	gs.TiledWindows = false
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

func TestSpeechBubbleScaleDoesNotFollowSupersamplingDefault(t *testing.T) {
	if got := speechBubbleWindowScale(3); got != 1 {
		t.Fatalf("3x physical scale got bubble scale %v, want 1", got)
	}
	if got := speechBubbleWindowScale(4); math.Abs(got-4.0/3.0) > 1e-9 {
		t.Fatalf("4x physical scale got bubble scale %v, want %v", got, 4.0/3.0)
	}
	if got := speechBubbleWindowScale(0); got != 1 {
		t.Fatalf("invalid physical scale got bubble scale %v, want 1", got)
	}
}

func TestUpdateGameImageSizeKeepsVisibleImageExactWhenShrinking(t *testing.T) {
	oldGameWin := gameWin
	oldGameImageItem := gameImageItem
	oldGameImage := gameImage
	oldGameImageBacking := gameImageBacking
	oldScale := eui.UIScale()
	oldTiledWindows := gs.TiledWindows
	defer func() {
		gameWin = oldGameWin
		gameImageItem = oldGameImageItem
		gameImage = oldGameImage
		gameImageBacking = oldGameImageBacking
		eui.SetUIScale(oldScale)
		gs.TiledWindows = oldTiledWindows
	}()

	eui.SetUIScale(1)
	gs.TiledWindows = false
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

func TestTiledGameResizeKeepsManagedWindowSize(t *testing.T) {
	oldGameWin := gameWin
	oldGameImageItem := gameImageItem
	oldGameImage := gameImage
	oldGameImageBacking := gameImageBacking
	oldTiledWindows := gs.TiledWindows
	oldInAspectResize := inAspectResize
	defer func() {
		gameWin = oldGameWin
		gameImageItem = oldGameImageItem
		gameImage = oldGameImage
		gameImageBacking = oldGameImageBacking
		gs.TiledWindows = oldTiledWindows
		inAspectResize = oldInAspectResize
	}()

	gameImageItem = nil
	gameImage = nil
	gameImageBacking = nil
	inAspectResize = false
	gs.TiledWindows = true
	gameWin = eui.NewWindow()
	gameWin.NoScale = true
	gameWin.Padding = 0
	gameWin.Border = 0
	gameWin.BorderPad = 0
	gameWin.Margin = 0
	gameWin.TitleHeight = 0
	gameWin.Size = eui.Point{X: 600, Y: 800}

	onGameWindowResize()

	if got, want := gameWin.GetSize(), (eui.Point{X: 600, Y: 800}); got != want {
		t.Fatalf("tiled game window size got %v, want %v", got, want)
	}
}

func TestTiledGameImageFillsEveryManagedPixel(t *testing.T) {
	oldGameWin := gameWin
	oldGameImageItem := gameImageItem
	oldGameImage := gameImage
	oldGameImageBacking := gameImageBacking
	oldTiledWindows := gs.TiledWindows
	defer func() {
		gameWin = oldGameWin
		gameImageItem = oldGameImageItem
		gameImage = oldGameImage
		gameImageBacking = oldGameImageBacking
		gs.TiledWindows = oldTiledWindows
	}()

	gameImageItem = nil
	gameImage = nil
	gameImageBacking = nil
	gs.TiledWindows = true
	gameWin = eui.NewWindow()
	gameWin.NoScale = true
	gameWin.Padding = 0
	gameWin.TitleHeight = 0
	gameWin.Size = eui.Point{X: 601, Y: 799}

	updateGameImageSize()

	if got := gameImage.Bounds().Size(); got.X != 601 || got.Y != 799 {
		t.Fatalf("tiled game image size = %v, want 601x799", got)
	}
	if got := gameImageItem.Position; got != (eui.Point{}) {
		t.Fatalf("tiled game image position = %v, want zero", got)
	}
}

func TestNetworkAdjustmentDelay(t *testing.T) {
	const frame = 200 * time.Millisecond

	tests := []struct {
		name              string
		replyTime, jitter time.Duration
		offset            int
		want              time.Duration
	}{
		{name: "disabled offset", offset: 0, want: 0},
		{name: "no reply measurement keeps offset", offset: 100, want: 100 * time.Millisecond},
		{name: "room before next frame keeps offset", replyTime: 40 * time.Millisecond, jitter: 10 * time.Millisecond, offset: 100, want: 100 * time.Millisecond},
		{name: "reply time advances send", replyTime: 100 * time.Millisecond, jitter: 20 * time.Millisecond, offset: 100, want: 80 * time.Millisecond},
		{name: "late reply sends immediately", replyTime: 190 * time.Millisecond, jitter: 10 * time.Millisecond, offset: 100, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := networkAdjustmentDelay(frame, tt.replyTime, tt.jitter, tt.offset); got != tt.want {
				t.Fatalf("networkAdjustmentDelay() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestP99Duration(t *testing.T) {
	if got := p99Duration(nil); got != 0 {
		t.Fatalf("p99Duration(nil) = %v, want 0", got)
	}
	if got := p99Duration([]time.Duration{2 * time.Millisecond, 7 * time.Millisecond, time.Millisecond}); got != 7*time.Millisecond {
		t.Fatalf("p99Duration() = %v, want 7ms while the sample window is warming up", got)
	}
	samples := make([]time.Duration, 100)
	for i := range samples {
		samples[i] = time.Duration(i+1) * time.Millisecond
	}
	if got := p99Duration(samples); got != 99*time.Millisecond {
		t.Fatalf("p99Duration() = %v, want 99ms", got)
	}
}
