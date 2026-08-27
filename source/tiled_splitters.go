package main

import (
	"math"

	"gothoom/eui"
)

// dockedToolbarMinimumWidth is the visible two-row toolbar's logical width,
// including the hand display, button spacing, and host-window padding.
const dockedToolbarMinimumWidth = 380.0

type tiledSplitter uint8

const (
	tiledSplitterNone tiledSplitter = iota
	tiledSplitterLeftBottom
	tiledSplitterRightBottom
	tiledSplitterLeftWidth
	tiledSplitterRightWidth
	tiledSplitterSideGame
	tiledSplitterSideTop
)

func updateTiledSplitter(splitter tiledSplitter, position, extent float64) bool {
	if splitter == tiledSplitterNone || extent <= 0 {
		return false
	}
	before := []float64{
		gs.TiledLeftBottom,
		gs.TiledRightBottom,
		gs.TiledLeftWidth,
		gs.TiledRightWidth,
		gs.TiledSideGameWidth,
		gs.TiledSideTopSplit,
	}
	beforeGamePosition := gs.TiledGamePosition
	switch splitter {
	case tiledSplitterLeftBottom:
		gs.TiledLeftBottom = 1 - position/extent
	case tiledSplitterRightBottom:
		gs.TiledRightBottom = 1 - position/extent
	case tiledSplitterLeftWidth:
		if gs.TiledKeepGameLarge && gs.TiledLayout == TiledLayoutCenter {
			moveFixedCenteredGame(position/extent, extent)
		} else {
			gs.TiledLeftWidth = position / extent
		}
	case tiledSplitterRightWidth:
		if gs.TiledKeepGameLarge && gs.TiledLayout == TiledLayoutCenter {
			gameWidth := 1 - gs.TiledLeftWidth - gs.TiledRightWidth
			moveFixedCenteredGame(position/extent-gameWidth, extent)
		} else {
			gs.TiledRightWidth = 1 - position/extent
		}
	case tiledSplitterSideGame:
		if gs.TiledGameLeft {
			gs.TiledSideGameWidth = position / extent
		} else {
			gs.TiledSideGameWidth = 1 - position/extent
		}
	case tiledSplitterSideTop:
		panelWidth := 1 - gs.TiledSideGameWidth
		panelStart := 0.0
		if gs.TiledGameLeft {
			panelStart = gs.TiledSideGameWidth
		}
		if panelWidth > 0 {
			gs.TiledSideTopSplit = (position/extent - panelStart) / panelWidth
		}
	}
	clampTiledLayoutSettings()
	after := []float64{
		gs.TiledLeftBottom,
		gs.TiledRightBottom,
		gs.TiledLeftWidth,
		gs.TiledRightWidth,
		gs.TiledSideGameWidth,
		gs.TiledSideTopSplit,
	}
	for i := range before {
		if before[i] != after[i] {
			return true
		}
	}
	return beforeGamePosition != gs.TiledGamePosition
}

func centeredSideMinimums(toolbarMinimum float64) (float64, float64) {
	const sideMinimum = 0.15
	leftMinimum := sideMinimum
	rightMinimum := sideMinimum
	if gs.ToolbarPlacement == ToolbarFloating {
		return leftMinimum, rightMinimum
	}
	toolbarMinimum = math.Min(math.Max(toolbarMinimum, sideMinimum), 0.55)
	if tiledToolbarIsInFirstPane() {
		leftMinimum = toolbarMinimum
	} else {
		rightMinimum = toolbarMinimum
	}
	return leftMinimum, rightMinimum
}

func moveFixedCenteredGame(desiredLeft, extent float64) {
	gameWidth := 1 - gs.TiledLeftWidth - gs.TiledRightWidth
	toolbarMinimum := 0.0
	if extent > 0 && gs.ToolbarPlacement != ToolbarFloating {
		toolbarMinimum = dockedToolbarMinimumWidth * float64(eui.UIScale()) / extent
	}
	leftMinimum, rightMinimum := centeredSideMinimums(toolbarMinimum)
	maxLeft := 1 - gameWidth - rightMinimum
	desiredLeft = math.Min(math.Max(desiredLeft, leftMinimum), maxLeft)
	gs.TiledLeftWidth = desiredLeft
	gs.TiledRightWidth = 1 - gameWidth - desiredLeft

	travel := 1 - gameWidth - leftMinimum - rightMinimum
	if travel <= 0 {
		gs.TiledGamePosition = 0
		return
	}
	gs.TiledGamePosition = math.Min(math.Max(2*(desiredLeft-leftMinimum)/travel-1, -1), 1)
}

// maximizeCenteredGameForWorkspace gives the full-height game pane the width
// of the largest square that fits between the two required side columns. Any
// extra horizontal room is shared evenly unless the toolbar's host needs more.
func maximizeCenteredGameForWorkspace(width, height int, toolbarMinimum float64) {
	if width <= 0 || height <= 0 {
		return
	}

	leftMinimum, rightMinimum := centeredSideMinimums(toolbarMinimum)

	gameWidth := math.Min(float64(height)/float64(width), 1-leftMinimum-rightMinimum)
	gameWidth = math.Max(gameWidth, 0.30)
	remaining := 1 - gameWidth
	travel := math.Max(0, remaining-leftMinimum-rightMinimum)
	position := math.Min(math.Max(gs.TiledGamePosition, -1), 1)
	gs.TiledLeftWidth = leftMinimum + travel*(position+1)/2
	gs.TiledRightWidth = remaining - gs.TiledLeftWidth
}

func tiledToolbarIsInFirstPane() bool {
	switch gs.ToolbarPlacement {
	case ToolbarInInventory:
		return gs.TiledInventoryLeft
	case ToolbarInPlayers:
		return !gs.TiledInventoryLeft
	default:
		return false
	}
}

// clampTiledLayoutForToolbar preserves the toolbar's two-row layout by
// preventing its host pane from becoming narrower than the toolbar. The
// fraction is relative to the full workspace width.
func clampTiledLayoutForToolbar(minWidthFraction float64) {
	if gs.ToolbarPlacement == ToolbarFloating || minWidthFraction <= 0 {
		return
	}
	if gs.TiledLayout == TiledLayoutSide {
		// The alternate layout can devote at most 65% of the workspace to the
		// panel side and 85% of that area to the toolbar's top pane.
		minWidthFraction = math.Min(minWidthFraction, 0.65*0.85)
		panelWidth := 1 - gs.TiledSideGameWidth
		requiredPanelWidth := minWidthFraction / 0.85
		if panelWidth < requiredPanelWidth {
			panelWidth = requiredPanelWidth
			gs.TiledSideGameWidth = 1 - panelWidth
		}
		if panelWidth <= 0 {
			return
		}
		requiredShare := minWidthFraction / panelWidth
		if tiledToolbarIsInFirstPane() {
			gs.TiledSideTopSplit = math.Max(gs.TiledSideTopSplit, requiredShare)
			gs.TiledSideTopSplit = math.Min(gs.TiledSideTopSplit, 0.85)
		} else {
			gs.TiledSideTopSplit = math.Min(gs.TiledSideTopSplit, 1-requiredShare)
			gs.TiledSideTopSplit = math.Max(gs.TiledSideTopSplit, 0.15)
		}
		return
	}

	// Keep at least 30% for the game and 15% for the opposite side. The
	// toolbar side may grow beyond the normal side-column maximum when the UI
	// scale requires it.
	minWidthFraction = math.Min(math.Max(minWidthFraction, 0.15), 0.55)
	if tiledToolbarIsInFirstPane() {
		gs.TiledLeftWidth = math.Max(gs.TiledLeftWidth, minWidthFraction)
		if gs.TiledLeftWidth+gs.TiledRightWidth > 0.70 {
			gs.TiledRightWidth = math.Max(0.15, 0.70-gs.TiledLeftWidth)
		}
		return
	}
	gs.TiledRightWidth = math.Max(gs.TiledRightWidth, minWidthFraction)
	if gs.TiledLeftWidth+gs.TiledRightWidth > 0.70 {
		gs.TiledLeftWidth = math.Max(0.15, 0.70-gs.TiledRightWidth)
	}
}

func configureTiledWorkspaceDividers() {
	if !gs.TiledWindows {
		eui.SetTileDividers(nil)
		return
	}
	width, height := eui.ScreenSize()
	if width <= 0 || height <= 0 {
		eui.SetTileDividers(nil)
		return
	}
	w := float32(width)
	h := float32(height)
	scale := eui.UIScale()
	dividers := make([]eui.TileDivider, 0, 5)
	add := func(orientation eui.TileDividerOrientation, position, start, end float32, splitter tiledSplitter, extent float64) {
		dividers = append(dividers, eui.TileDivider{
			Orientation: orientation,
			Position:    position,
			Start:       start,
			End:         end,
			Thickness:   5 * scale,
			HitSize:     16 * scale,
			OnDrag: func(next float32) {
				if updateTiledSplitter(splitter, float64(next), extent) {
					applyManagedWindowLayout()
					settingsDirty = true
				}
			},
		})
	}

	if gs.TiledLayout == TiledLayoutSide {
		panelWidth := 1 - gs.TiledSideGameWidth
		panelStart := 0.0
		major := panelWidth
		if gs.TiledGameLeft {
			panelStart = gs.TiledSideGameWidth
			major = gs.TiledSideGameWidth
		}
		topEnd := float32((1 - gs.TiledRightBottom) * float64(height))
		add(eui.TileDividerVertical, float32(major*float64(width)), 0, h, tiledSplitterSideGame, float64(width))
		topSplit := panelStart + panelWidth*gs.TiledSideTopSplit
		add(eui.TileDividerVertical, float32(topSplit*float64(width)), 0, topEnd, tiledSplitterSideTop, float64(width))
		add(eui.TileDividerHorizontal, topEnd, float32(panelStart*float64(width)), float32((panelStart+panelWidth)*float64(width)), tiledSplitterRightBottom, float64(height))
		eui.SetTileDividers(dividers)
		return
	}

	leftEnd := float32(gs.TiledLeftWidth * float64(width))
	rightStart := float32((1 - gs.TiledRightWidth) * float64(width))
	add(eui.TileDividerVertical, leftEnd, 0, h, tiledSplitterLeftWidth, float64(width))
	add(eui.TileDividerVertical, rightStart, 0, h, tiledSplitterRightWidth, float64(width))
	if gs.MessagesToConsole {
		if gs.TiledConsoleLeft {
			add(eui.TileDividerHorizontal, float32((1-gs.TiledLeftBottom)*float64(height)), 0, leftEnd, tiledSplitterLeftBottom, float64(height))
		} else {
			add(eui.TileDividerHorizontal, float32((1-gs.TiledRightBottom)*float64(height)), rightStart, w, tiledSplitterRightBottom, float64(height))
		}
	} else {
		add(eui.TileDividerHorizontal, float32((1-gs.TiledLeftBottom)*float64(height)), 0, leftEnd, tiledSplitterLeftBottom, float64(height))
		add(eui.TileDividerHorizontal, float32((1-gs.TiledRightBottom)*float64(height)), rightStart, w, tiledSplitterRightBottom, float64(height))
	}
	eui.SetTileDividers(dividers)
}
