package main

import (
	"gothoom/eui"
	"math"
)

var tiledLayoutNames = []string{
	"Game centered",
	"Game on a side",
	"Messages below game",
	"Messages above game",
	"Messages above and below",
	"Full-width messages below",
	"Full-width messages above",
}

func tiledFullWidthMessages() bool {
	return gs.TiledLayout == TiledLayoutFullMessagesBelow || gs.TiledLayout == TiledLayoutFullMessagesAbove
}

// Band heights are fractions of the workspace. Combining split messages
// removes Chat's band entirely and gives that space back to the game.
func tiledMessageBandHeights() (top, bottom float64) {
	switch gs.TiledLayout {
	case TiledLayoutMessagesAbove, TiledLayoutFullMessagesAbove:
		top = gs.TiledMessagesTopHeight
	case TiledLayoutMessagesBelow, TiledLayoutFullMessagesBelow:
		bottom = gs.TiledMessagesBottomHeight
	case TiledLayoutMessagesSplit:
		if !gs.MessagesToConsole || gs.TiledConsoleLeft {
			top = gs.TiledMessagesTopHeight
		}
		if !gs.MessagesToConsole || !gs.TiledConsoleLeft {
			bottom = gs.TiledMessagesBottomHeight
		}
	}
	return
}

func tiledMessageBandSpan() (x, width float64) {
	if gs.TiledLayout == TiledLayoutSide {
		if gs.TiledGameLeft {
			return gs.TiledSideGameWidth, 1 - gs.TiledSideGameWidth
		}
		return 0, 1 - gs.TiledSideGameWidth
	}
	if tiledFullWidthMessages() {
		return 0, 1
	}
	return gs.TiledLeftWidth, 1 - gs.TiledLeftWidth - gs.TiledRightWidth
}

func applyMessageBandTiledWindowStates() {
	top, bottom := tiledMessageBandHeights()
	left, right := gs.TiledLeftWidth, gs.TiledRightWidth
	tiledWindowState(&gs.GameWindow, left, top, 1-left-right, 1-top-bottom)

	listY, listHeight := 0.0, 1.0
	if tiledFullWidthMessages() {
		listY, listHeight = top, 1-top-bottom
	}
	first, second := &gs.InventoryWindow, &gs.PlayersWindow
	if !gs.TiledInventoryLeft {
		first, second = second, first
	}
	tiledWindowState(first, 0, listY, left, listHeight)
	tiledWindowState(second, 1-right, listY, right, listHeight)

	x, width := tiledMessageBandSpan()
	first, second = &gs.MessagesWindow, &gs.ChatWindow
	if !gs.TiledConsoleLeft {
		first, second = second, first
	}
	if gs.TiledLayout == TiledLayoutMessagesSplit {
		if top > 0 {
			tiledWindowState(first, x, 0, width, top)
		}
		if bottom > 0 {
			tiledWindowState(second, x, 1-bottom, width, bottom)
		}
		return
	}
	y, height := 0.0, top
	if bottom > 0 {
		y, height = 1-bottom, bottom
	}
	if gs.MessagesToConsole {
		tiledWindowState(&gs.MessagesWindow, x, y, width, height)
		return
	}
	applyTiledMessagePair(x, y, width, height)
}

func tiledMessageOrderDisabled() bool {
	return gs.MessagesToConsole && gs.TiledLayout != TiledLayoutCenter && gs.TiledLayout != TiledLayoutMessagesSplit
}

func tiledPairedMessages() bool {
	return gs.TiledLayout == TiledLayoutSide || (gs.TiledLayout >= TiledLayoutMessagesBelow && gs.TiledLayout != TiledLayoutMessagesSplit)
}

func tiledMessageBandVerticalSpan() (y, height float64) {
	if gs.TiledLayout == TiledLayoutSide {
		return 1 - gs.TiledRightBottom, gs.TiledRightBottom
	}
	top, bottom := tiledMessageBandHeights()
	if bottom > 0 {
		return 1 - bottom, bottom
	}
	return 0, top
}

func applyTiledMessagePair(x, y, width, height float64) {
	first, second := &gs.MessagesWindow, &gs.ChatWindow
	if !gs.TiledConsoleLeft {
		first, second = second, first
	}
	if gs.TiledMessagesStacked {
		split := height * gs.TiledMessagesSplit
		tiledWindowState(first, x, y, width, split)
		tiledWindowState(second, x, y+split, width, height-split)
	} else {
		split := width * gs.TiledMessagesSplit
		tiledWindowState(first, x, y, split, height)
		tiledWindowState(second, x+split, y, width-split, height)
	}
}

// Leave room for two native panes before computing the game size. Otherwise
// EUI's minimum window height could expand stacked panes into each other.
func clampTiledMessagePairHeight(height int) {
	if gs.MessagesToConsole || !tiledPairedMessages() || !gs.TiledMessagesStacked || height <= 0 {
		return
	}
	minimum := math.Min(0.60, 2*eui.MinWindowSize*float64(eui.UIScale())/float64(height))
	switch gs.TiledLayout {
	case TiledLayoutSide:
		gs.TiledRightBottom = math.Max(gs.TiledRightBottom, minimum)
	case TiledLayoutMessagesAbove, TiledLayoutFullMessagesAbove:
		gs.TiledMessagesTopHeight = math.Max(gs.TiledMessagesTopHeight, minimum)
	default:
		gs.TiledMessagesBottomHeight = math.Max(gs.TiledMessagesBottomHeight, minimum)
	}
}

func clampTiledMessagePairSplit(width, height int) {
	if gs.MessagesToConsole || !tiledPairedMessages() {
		return
	}
	_, fraction := tiledMessageBandSpan()
	extent := fraction * float64(width)
	if gs.TiledMessagesStacked {
		_, fraction = tiledMessageBandVerticalSpan()
		extent = fraction * float64(height)
	}
	if extent <= 0 {
		return
	}
	minimum := math.Min(0.5, eui.MinWindowSize*float64(eui.UIScale())/extent)
	gs.TiledMessagesSplit = math.Min(1-minimum, math.Max(minimum, gs.TiledMessagesSplit))
}
