package main

import (
	"math"
	"testing"

	"gothoom/eui"
)

func near(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestDefaultWindowLayoutIsNormalizedAndGapless(t *testing.T) {
	if gsdef.WindowWidth != 1920 || gsdef.WindowHeight != 1080 {
		t.Fatalf("default application size = %dx%d", gsdef.WindowWidth, gsdef.WindowHeight)
	}
	if !normalizedWindowStateValid(gsdef.GameWindow, true) ||
		!normalizedWindowStateValid(gsdef.InventoryWindow, true) ||
		!normalizedWindowStateValid(gsdef.PlayersWindow, true) ||
		!normalizedWindowStateValid(gsdef.MessagesWindow, true) ||
		!normalizedWindowStateValid(gsdef.ChatWindow, true) ||
		!normalizedWindowStateValid(gsdef.ToolbarWindow, false) {
		t.Fatal("default resizable window layout is not normalized")
	}
	if gsdef.ToolbarPlacement != ToolbarInInventory {
		t.Fatalf("default toolbar placement = %v, want inventory", gsdef.ToolbarPlacement)
	}
	if gsdef.ToolbarInfoBar {
		t.Fatal("toolbar info bar should default off")
	}
	if !gsdef.TiledKeepGameLarge {
		t.Fatal("tiled game window should default to its largest size")
	}
	if !gsdef.TiledInventoryLeft || !gsdef.TiledConsoleLeft {
		t.Fatalf("default tiled sides = inventory left %t, console left %t; want both left", gsdef.TiledInventoryLeft, gsdef.TiledConsoleLeft)
	}
	if !gsdef.MessagesToConsole {
		t.Fatal("chat and console should be combined by default")
	}
	if !gsdef.AutoResizeWindows {
		t.Fatal("window layout auto-resize should default on")
	}

	leftEdge := gsdef.InventoryWindow.Position.X + gsdef.InventoryWindow.Size.X
	gameEdge := gsdef.GameWindow.Position.X + gsdef.GameWindow.Size.X
	rightEdge := gsdef.PlayersWindow.Position.X + gsdef.PlayersWindow.Size.X
	if !near(leftEdge, gsdef.GameWindow.Position.X) ||
		!near(gameEdge, gsdef.PlayersWindow.Position.X) ||
		!near(rightEdge, 1) {
		t.Fatalf("column edges are not gapless: left=%v game=%v right=%v", leftEdge, gameEdge, rightEdge)
	}

	leftBottom := gsdef.InventoryWindow.Position.Y + gsdef.InventoryWindow.Size.Y
	messageBottom := gsdef.MessagesWindow.Position.Y + gsdef.MessagesWindow.Size.Y
	rightTopBottom := gsdef.PlayersWindow.Position.Y + gsdef.PlayersWindow.Size.Y
	chatBottom := gsdef.ChatWindow.Position.Y + gsdef.ChatWindow.Size.Y
	if !near(leftBottom, gsdef.MessagesWindow.Position.Y) || !near(messageBottom, 1) ||
		!near(rightTopBottom, gsdef.ChatWindow.Position.Y) || !near(chatBottom, 1) {
		t.Fatalf("row edges are not gapless: inventory=%v messages=%v players=%v chat=%v", leftBottom, messageBottom, rightTopBottom, chatBottom)
	}
}

func TestAbsoluteWindowSettingsAreRejected(t *testing.T) {
	original := gs
	defer func() { gs = original }()

	gs = gsdef
	gs.GameWindow.Position.X = 468
	gs.GameWindow.Size = WindowPoint{X: 936, Y: 948}
	if normalizedWindowSettingsValid() {
		t.Fatal("absolute window settings were accepted as normalized")
	}
}

func TestApplyWindowStateScalesResizableWindow(t *testing.T) {
	originalScale := eui.UIScale()
	originalW, originalH := eui.ScreenSize()
	defer func() {
		eui.SetUIScale(originalScale)
		eui.SetScreenSize(originalW, originalH)
	}()

	eui.SetUIScale(1)
	eui.SetScreenSize(1000, 800)
	win := eui.NewWindow()
	win.Resizable = true
	state := WindowState{Position: WindowPoint{X: 0.2, Y: 0.25}, Size: WindowPoint{X: 0.4, Y: 0.5}}
	applyWindowState(win, &state)
	if got := win.GetPos(); got != (eui.Point{X: 200, Y: 200}) {
		t.Fatalf("initial position = %+v", got)
	}
	if got := win.GetSize(); got != (eui.Point{X: 400, Y: 400}) {
		t.Fatalf("initial size = %+v", got)
	}

	eui.SetScreenSize(500, 400)
	applyWindowState(win, &state)
	if got := win.GetPos(); got != (eui.Point{X: 100, Y: 100}) {
		t.Fatalf("scaled position = %+v", got)
	}
	if got := win.GetSize(); got != (eui.Point{X: 200, Y: 200}) {
		t.Fatalf("scaled size = %+v", got)
	}
	if syncWindow(win, &state) {
		t.Fatal("applying a scaled layout changed the saved fractions")
	}
	if state.Position != (WindowPoint{X: 0.2, Y: 0.25}) || state.Size != (WindowPoint{X: 0.4, Y: 0.5}) {
		t.Fatalf("saved fractions drifted: position=%+v size=%+v", state.Position, state.Size)
	}

	eui.SetScreenSize(1000, 800)
	applyWindowState(win, &state)
	if got := win.GetPos(); got != (eui.Point{X: 200, Y: 200}) {
		t.Fatalf("restored position = %+v", got)
	}
	if got := win.GetSize(); got != (eui.Point{X: 400, Y: 400}) {
		t.Fatalf("restored size = %+v", got)
	}
}

func TestApplyWindowStateDoesNotResizeFixedWindow(t *testing.T) {
	originalScale := eui.UIScale()
	originalW, originalH := eui.ScreenSize()
	defer func() {
		eui.SetUIScale(originalScale)
		eui.SetScreenSize(originalW, originalH)
	}()

	eui.SetUIScale(1)
	eui.SetScreenSize(1000, 800)
	win := eui.NewWindow()
	win.Resizable = false
	win.Size = eui.Point{X: 300, Y: 100}
	state := WindowState{Position: WindowPoint{X: 0.5, Y: 0.25}, Size: WindowPoint{X: 0.9, Y: 0.9}}
	applyWindowState(win, &state)
	if got := win.GetSize(); got != (eui.Point{X: 300, Y: 100}) {
		t.Fatalf("fixed window size changed to %+v", got)
	}
	if got := win.GetPos(); got != (eui.Point{X: 500, Y: 200}) {
		t.Fatalf("fixed window position = %+v", got)
	}

	eui.SetScreenSize(600, 400)
	applyWindowState(win, &state)
	if got := win.GetSize(); got != (eui.Point{X: 300, Y: 100}) {
		t.Fatalf("fixed window size changed after screen resize to %+v", got)
	}
	if got := win.GetPos(); got != (eui.Point{X: 300, Y: 100}) {
		t.Fatalf("scaled fixed window position = %+v", got)
	}
}

func TestResetSavedWindowSettings(t *testing.T) {
	original := gs
	defer func() { gs = original }()

	gs.GameWindow = WindowState{Open: false, Position: WindowPoint{X: 0.1, Y: 0.2}, Size: WindowPoint{X: 0.3, Y: 0.4}}
	gs.InventoryWindow = WindowState{Open: false}
	gs.PlayersWindow = WindowState{Open: false}
	gs.MessagesWindow = WindowState{Open: false}
	gs.ChatWindow = WindowState{Open: false}
	gs.MovieWindow = WindowState{Open: true, Position: WindowPoint{X: 0.5, Y: 0.6}}
	gs.ToolbarWindow = WindowState{Open: false, Position: WindowPoint{X: 0.7, Y: 0.8}}
	gs.ToolbarPlacement = ToolbarFloating
	gs.ToolbarInfoBar = true
	gs.TiledKeepGameLarge = false
	gs.TiledInventoryLeft = false
	gs.TiledConsoleLeft = false
	gs.MessagesToConsole = false

	resetSavedWindowSettings()

	if gs.GameWindow != gsdef.GameWindow || gs.InventoryWindow != gsdef.InventoryWindow ||
		gs.PlayersWindow != gsdef.PlayersWindow || gs.MessagesWindow != gsdef.MessagesWindow ||
		gs.ChatWindow != gsdef.ChatWindow || gs.MovieWindow != gsdef.MovieWindow ||
		gs.ToolbarWindow != gsdef.ToolbarWindow {
		t.Fatal("window states were not restored to defaults")
	}
	if gs.ToolbarPlacement != ToolbarInInventory {
		t.Fatalf("toolbar placement = %v, want inventory", gs.ToolbarPlacement)
	}
	if gs.ToolbarInfoBar {
		t.Fatal("toolbar info bar was not restored to its off default")
	}
	if !gs.TiledKeepGameLarge {
		t.Fatal("large tiled game mode was not restored to its default")
	}
	if !gs.TiledInventoryLeft || !gs.TiledConsoleLeft || !gs.MessagesToConsole {
		t.Fatalf("default tiled arrangement was not restored: inventory left %t, console left %t, combined %t", gs.TiledInventoryLeft, gs.TiledConsoleLeft, gs.MessagesToConsole)
	}
}
