package main

import (
	"gothoom/eui"
	"testing"
)

func TestPointInUIOverlay(t *testing.T) {
	gameWin = eui.NewWindow()
	gameWin.MarkOpen()
	btn, _ := eui.NewButton()
	btn.DrawRect.X0 = 10
	btn.DrawRect.Y0 = 10
	btn.DrawRect.X1 = 20
	btn.DrawRect.Y1 = 20
	gameWin.AddItem(btn)
	if !pointInUI(15, 15) {
		t.Fatalf("pointInUI should detect overlay item")
	}
}

func TestUIOwnsPointerPressAfterWindowCloses(t *testing.T) {
	oldGameWin := gameWin
	defer func() { gameWin = oldGameWin }()

	gameWin = eui.NewWindow()
	closedWin := eui.NewWindow()
	closedWin.MarkOpen()
	closedWin.Close()

	if !uiOwnsPointerPress(false, closedWin, false) {
		t.Fatalf("a press begun on a UI window should remain owned after it closes")
	}
	if uiOwnsPointerPress(false, gameWin, false) {
		t.Fatalf("the playable game window should pass an unhandled press to the world")
	}
	if !uiOwnsPointerPress(false, gameWin, true) {
		t.Fatalf("an EUI-handled press should not pass through the game window")
	}
}

func TestPointInGameWindow(t *testing.T) {
	gameWin = eui.NewWindow()
	gameWin.MarkOpen()
	_ = gameWin.SetPos(eui.Point{X: 10, Y: 10})
	_ = gameWin.SetSize(eui.Point{X: 100, Y: 100})
	gameWin.Margin = 0
	gameWin.Border = 0
	gameWin.BorderPad = 0
	gameWin.Padding = 0
	gameWin.TitleHeight = 0

	if !pointInGameWindow(50, 50) {
		t.Fatalf("pointInGameWindow should detect interior point")
	}
	if pointInGameWindow(5, 5) {
		t.Fatalf("pointInGameWindow should ignore exterior point")
	}
}

func TestPointInGameWindowNoScaleUsesWindowScale(t *testing.T) {
	oldGameWin := gameWin
	oldW, oldH := eui.ScreenSize()
	defer func() {
		gameWin = oldGameWin
		eui.SetUIScale(1)
		eui.SetScreenSize(oldW, oldH)
	}()

	eui.SetUIScale(2)
	gameWin = eui.NewWindow()
	gameWin.NoScale = true
	gameWin.MarkOpen()
	_ = gameWin.SetPos(eui.Point{X: 10, Y: 10})
	_ = gameWin.SetSize(eui.Point{X: 100, Y: 100})
	gameWin.Margin = 2
	gameWin.Border = 1
	gameWin.BorderPad = 1
	gameWin.Padding = 1
	gameWin.TitleHeight = 0

	if !pointInGameWindow(16, 50) {
		t.Fatalf("NoScale game window hit test should not multiply frame by UI scale")
	}
}

func TestPointInAppScreenUsesEUIScreenSize(t *testing.T) {
	oldW, oldH := eui.ScreenSize()
	defer eui.SetScreenSize(oldW, oldH)

	eui.SetScreenSize(1500, 1000)
	if !pointInAppScreen(1400, 900) {
		t.Fatalf("pointInAppScreen should use EUI screen size")
	}
	if pointInAppScreen(1500, 900) {
		t.Fatalf("pointInAppScreen should reject coordinates beyond EUI screen size")
	}
}

func TestTypingInUI(t *testing.T) {
	for _, w := range eui.Windows() {
		w.Close()
	}

	consoleWin = eui.NewWindow()
	inputFlow = &eui.ItemData{}
	consoleWin.AddItem(inputFlow)
	inp, _ := eui.NewText()
	inputFlow.AddItem(inp)
	consoleWin.MarkOpen()

	inp.Focused = true
	if typingInUI() {
		t.Fatalf("typingInUI should ignore console input")
	}

	inp.Focused = false
	win := eui.NewWindow()
	other, _ := eui.NewInput()
	other.Focused = true
	win.AddItem(other)
	win.MarkOpen()

	if !typingInUI() {
		t.Fatalf("typingInUI should detect focused input")
	}
}

func TestTypingInUISearch(t *testing.T) {
	for _, w := range eui.Windows() {
		w.Close()
	}
	win := eui.NewWindow()
	win.Searchable = true
	win.MarkOpen()
	eui.SetActiveSearchForTest(win)
	if !typingInUI() {
		t.Fatalf("typingInUI should detect active search")
	}
	eui.SetActiveSearchForTest(nil)
}
