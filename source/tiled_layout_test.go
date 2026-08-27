package main

import (
	"math"
	"testing"

	"gothoom/eui"
)

func TestCenteredTiledLayoutUsesCurrentThreeColumnWorkspace(t *testing.T) {
	original := gs
	originalWidth, originalHeight := eui.ScreenSize()
	t.Cleanup(func() {
		gs = original
		eui.SetScreenSize(originalWidth, originalHeight)
	})

	gs = gsdef
	gs.TiledWindows = true
	gs.MessagesToConsole = false
	eui.SetScreenSize(1920, 1080)
	applyTiledWindowStates()
	if !gs.GameWindow.Open || !gs.InventoryWindow.Open || !gs.PlayersWindow.Open || !gs.MessagesWindow.Open || !gs.ChatWindow.Open {
		t.Fatal("separate tiled workspace did not open every main pane")
	}

	assertWindowRect(t, gs.GameWindow, gs.TiledLeftWidth, 0, 1-gs.TiledLeftWidth-gs.TiledRightWidth, 1)
	assertWindowRect(t, gs.InventoryWindow, 0, 0, gs.TiledLeftWidth, 0.70)
	assertWindowRect(t, gs.PlayersWindow, 1-gs.TiledRightWidth, 0, gs.TiledRightWidth, 0.70)
	assertWindowRect(t, gs.MessagesWindow, 0, 0.70, gs.TiledLeftWidth, 0.30)
	assertWindowRect(t, gs.ChatWindow, 1-gs.TiledRightWidth, 0.70, gs.TiledRightWidth, 0.30)
	if gamePixels := gs.GameWindow.Size.X * 1920; math.Abs(gamePixels-1080) > 1e-9 {
		t.Fatalf("game pane width = %v pixels, want square 1080", gamePixels)
	}
}

func TestTiledGameWindowHidesAndRestoresTitleBar(t *testing.T) {
	originalGS := gs
	originalGameWin := gameWin
	originalTitleHeight := gameWindowFreeformTitleHeight
	t.Cleanup(func() {
		if gameWin != nil && gameWin != originalGameWin {
			gameWin.RemoveWindow()
		}
		gs = originalGS
		gameWin = originalGameWin
		gameWindowFreeformTitleHeight = originalTitleHeight
	})

	gameWin = eui.NewWindow()
	defaultTitleHeight := gameWin.GetRawTitleSize()
	gameWindowFreeformTitleHeight = defaultTitleHeight
	gs = gsdef
	gs.TiledWindows = true
	prepareTiledWorkspaceWindowChrome()
	if gameWin.GetRawTitleSize() != 0 {
		t.Fatalf("tiled game title height = %v, want 0", gameWin.GetRawTitleSize())
	}

	gs.TiledWindows = false
	prepareTiledWorkspaceWindowChrome()
	if gameWin.GetRawTitleSize() != defaultTitleHeight {
		t.Fatalf("freeform game title height = %v, want %v", gameWin.GetRawTitleSize(), defaultTitleHeight)
	}
}

func TestCenteredTiledLayoutSwapsBothSidePairs(t *testing.T) {
	original := gs
	t.Cleanup(func() { gs = original })

	gs = gsdef
	gs.TiledWindows = true
	gs.TiledKeepGameLarge = true
	gs.MessagesToConsole = false
	gs.TiledInventoryLeft = false
	gs.TiledConsoleLeft = false
	applyTiledWindowStates()

	assertWindowRect(t, gs.PlayersWindow, 0, 0, gs.TiledLeftWidth, 0.70)
	assertWindowRect(t, gs.InventoryWindow, 1-gs.TiledRightWidth, 0, gs.TiledRightWidth, 0.70)
	assertWindowRect(t, gs.ChatWindow, 0, 0.70, gs.TiledLeftWidth, 0.30)
	assertWindowRect(t, gs.MessagesWindow, 1-gs.TiledRightWidth, 0.70, gs.TiledRightWidth, 0.30)
}

func TestCombinedMessagesCollapseChatIntoCenteredConsoleTile(t *testing.T) {
	original := gs
	t.Cleanup(func() { gs = original })

	gs = gsdef
	gs.TiledWindows = true
	gs.MessagesToConsole = true
	gs.ChatWindow.Open = true
	applyTiledWindowStates()

	assertWindowRect(t, gs.GameWindow, gs.TiledLeftWidth, 0, 1-gs.TiledLeftWidth-gs.TiledRightWidth, 1)
	assertWindowRect(t, gs.InventoryWindow, 0, 0, gs.TiledLeftWidth, 0.70)
	assertWindowRect(t, gs.PlayersWindow, 1-gs.TiledRightWidth, 0, gs.TiledRightWidth, 1)
	assertWindowRect(t, gs.MessagesWindow, 0, 0.70, gs.TiledLeftWidth, 0.30)
	assertWindowRect(t, gs.ChatWindow, gsdef.ChatWindow.Position.X, gsdef.ChatWindow.Position.Y, gsdef.ChatWindow.Size.X, gsdef.ChatWindow.Size.Y)
	if gs.ChatWindow.Open {
		t.Fatal("chat window remains open in combined-message layout")
	}

	gs.TiledConsoleLeft = false
	applyTiledWindowStates()
	assertWindowRect(t, gs.InventoryWindow, 0, 0, gs.TiledLeftWidth, 1)
	assertWindowRect(t, gs.PlayersWindow, 1-gs.TiledRightWidth, 0, gs.TiledRightWidth, 0.70)
	assertWindowRect(t, gs.MessagesWindow, 1-gs.TiledRightWidth, 0.70, gs.TiledRightWidth, 0.30)

	gs.MessagesToConsole = false
	applyTiledWindowStates()
	if !gs.ChatWindow.Open || gs.ChatWindow.Size.X == 0 || gs.ChatWindow.Size.Y == 0 {
		t.Fatal("chat tile did not return after disabling combined messages")
	}
}

func TestSideTiledLayoutKeepsGameOnSelectedSide(t *testing.T) {
	original := gs
	t.Cleanup(func() { gs = original })

	gs = gsdef
	gs.TiledWindows = true
	gs.TiledLayout = TiledLayoutSide
	gs.MessagesToConsole = false
	applyTiledWindowStates()
	if !gs.MessagesToConsole {
		t.Fatal("side tiled layout should combine chat and console")
	}

	assertWindowRect(t, gs.GameWindow, 0, 0, gs.TiledSideGameWidth, 1)
	assertWindowRect(t, gs.InventoryWindow, gs.TiledSideGameWidth, 0, (1-gs.TiledSideGameWidth)*gs.TiledSideTopSplit, 0.70)
	assertWindowRect(t, gs.PlayersWindow, gs.TiledSideGameWidth+(1-gs.TiledSideGameWidth)*gs.TiledSideTopSplit, 0, (1-gs.TiledSideGameWidth)*(1-gs.TiledSideTopSplit), 0.70)
	assertWindowRect(t, gs.MessagesWindow, gs.TiledSideGameWidth, 0.70, 1-gs.TiledSideGameWidth, 0.30)

	gs.TiledGameLeft = false
	applyTiledWindowStates()
	assertWindowRect(t, gs.GameWindow, 1-gs.TiledSideGameWidth, 0, gs.TiledSideGameWidth, 1)
	assertWindowRect(t, gs.InventoryWindow, 0, 0, (1-gs.TiledSideGameWidth)*gs.TiledSideTopSplit, 0.70)
}

func TestTiledSplittersPersistEveryWorkspaceDivision(t *testing.T) {
	original := gs
	t.Cleanup(func() {
		gs = original
	})

	gs = gsdef
	gs.TiledWindows = true
	gs.TiledKeepGameLarge = false
	if !updateTiledSplitter(tiledSplitterLeftBottom, 480, 800) {
		t.Fatal("left splitter did not update its pane fraction")
	}
	if math.Abs(gs.TiledLeftBottom-0.40) > 1e-9 {
		t.Fatalf("left bottom fraction = %v, want 0.40", gs.TiledLeftBottom)
	}
	if !updateTiledSplitter(tiledSplitterLeftWidth, 300, 1000) || math.Abs(gs.TiledLeftWidth-0.30) > 1e-9 {
		t.Fatalf("left width = %v, want 0.30", gs.TiledLeftWidth)
	}
	if !updateTiledSplitter(tiledSplitterRightWidth, 800, 1000) || math.Abs(gs.TiledRightWidth-0.20) > 1e-9 {
		t.Fatalf("right width = %v, want 0.20", gs.TiledRightWidth)
	}
	gs.TiledLayout = TiledLayoutSide
	if !updateTiledSplitter(tiledSplitterSideGame, 700, 1000) || math.Abs(gs.TiledSideGameWidth-0.70) > 1e-9 {
		t.Fatalf("side game width = %v, want 0.70", gs.TiledSideGameWidth)
	}
	if !updateTiledSplitter(tiledSplitterSideTop, 880, 1000) || math.Abs(gs.TiledSideTopSplit-0.60) > 1e-9 {
		t.Fatalf("side top split = %v, want 0.60", gs.TiledSideTopSplit)
	}
}

func TestCenteredVerticalSplitterMovesFixedWidthGame(t *testing.T) {
	original := gs
	originalScale := eui.UIScale()
	t.Cleanup(func() {
		gs = original
		eui.SetUIScale(originalScale)
	})

	eui.SetUIScale(1)
	gs = gsdef
	gs.TiledLayout = TiledLayoutCenter
	gs.TiledKeepGameLarge = true
	gs.ToolbarPlacement = ToolbarFloating
	maximizeCenteredGameForWorkspace(1920, 1080, 0)
	gameWidth := 1 - gs.TiledLeftWidth - gs.TiledRightWidth
	originalRight := gs.TiledRightWidth

	if !updateTiledSplitter(tiledSplitterLeftWidth, 480, 1920) {
		t.Fatal("left divider did not move fixed-width game pane")
	}
	if math.Abs(gs.TiledLeftWidth-0.25) > 1e-9 {
		t.Fatalf("left width = %v, want 0.25", gs.TiledLeftWidth)
	}
	if math.Abs((1-gs.TiledLeftWidth-gs.TiledRightWidth)-gameWidth) > 1e-9 {
		t.Fatal("moving left divider changed fixed game width")
	}
	if gs.TiledRightWidth == originalRight {
		t.Fatal("opposite divider did not move with fixed-width game")
	}

	position := gs.TiledGamePosition
	maximizeCenteredGameForWorkspace(1920, 1080, 0)
	if math.Abs(gs.TiledGamePosition-position) > 1e-9 || math.Abs(gs.TiledLeftWidth-0.25) > 1e-9 {
		t.Fatal("fixed game position was not preserved when layout reapplied")
	}
}

func TestMaximizeCenteredGameHonorsToolbarPaneMinimum(t *testing.T) {
	original := gs
	t.Cleanup(func() { gs = original })

	gs = gsdef
	gs.ToolbarPlacement = ToolbarInInventory
	gs.TiledInventoryLeft = true
	maximizeCenteredGameForWorkspace(1920, 1080, 0.30)
	if math.Abs(gs.TiledLeftWidth-0.30) > 1e-9 {
		t.Fatalf("toolbar side width = %v, want 0.30", gs.TiledLeftWidth)
	}
	if math.Abs(gs.TiledRightWidth-0.15) > 1e-9 {
		t.Fatalf("opposite side width = %v, want 0.15", gs.TiledRightWidth)
	}
	if gameWidth := 1 - gs.TiledLeftWidth - gs.TiledRightWidth; math.Abs(gameWidth-0.55) > 1e-9 {
		t.Fatalf("game width = %v, want largest feasible 0.55", gameWidth)
	}
}

func TestTiledToolbarPaneCannotShrinkBelowToolbarWidth(t *testing.T) {
	original := gs
	t.Cleanup(func() { gs = original })

	gs = gsdef
	gs.TiledWindows = true
	gs.TiledLeftWidth = 0.15
	gs.TiledRightWidth = 0.30
	gs.ToolbarPlacement = ToolbarInInventory
	gs.TiledInventoryLeft = true
	clampTiledLayoutForToolbar(0.28)
	if math.Abs(gs.TiledLeftWidth-0.28) > 1e-9 {
		t.Fatalf("toolbar pane width = %v, want 0.28", gs.TiledLeftWidth)
	}

	gs.TiledLayout = TiledLayoutSide
	gs.TiledSideGameWidth = 0.70
	gs.TiledSideTopSplit = 0.50
	clampTiledLayoutForToolbar(0.34)
	panelWidth := 1 - gs.TiledSideGameWidth
	if got := panelWidth * gs.TiledSideTopSplit; got+1e-9 < 0.34 {
		t.Fatalf("alternate toolbar pane width = %v, want at least 0.34", got)
	}
}

func assertWindowRect(t *testing.T, state WindowState, x, y, width, height float64) {
	t.Helper()
	if math.Abs(state.Position.X-x) > 1e-9 || math.Abs(state.Position.Y-y) > 1e-9 || math.Abs(state.Size.X-width) > 1e-9 || math.Abs(state.Size.Y-height) > 1e-9 {
		t.Fatalf("window state = position:%+v size:%+v, want position:(%.3f, %.3f) size:(%.3f, %.3f)", state.Position, state.Size, x, y, width, height)
	}
}
