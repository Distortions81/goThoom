package main

import (
	"fmt"
	"math"
	"testing"

	"gothoom/eui"
)

func isolateTiledGameSize(t *testing.T) {
	t.Helper()
	original, scale := gs, eui.UIScale()
	w, h := eui.ScreenSize()
	t.Cleanup(func() { gs = original; eui.SetUIScale(scale); eui.SetScreenSize(w, h) })
	gs = gsdef
	gs.TiledWindows = true
	gs.TiledKeepGameLarge = false
	gs.TiledLeftWidth, gs.TiledRightWidth = 0.32, 0.28
	eui.SetUIScale(1)
	eui.SetScreenSize(1920, 1080)
}

func TestKeepGameLargeOffKeepsSizeAndUnlocksDividers(t *testing.T) {
	for layout := TiledLayoutCenter; layout <= TiledLayoutFullMessagesAbove; layout++ {
		if layout == TiledLayoutSide {
			continue
		}
		for _, combined := range []bool{false, true} {
			t.Run(fmt.Sprintf("layout=%d/combined=%t", layout, combined), func(t *testing.T) {
				isolateTiledGameSize(t)
				gs.TiledLayout, gs.MessagesToConsole = layout, combined
				applyTiledWindowStates()
				for i := 0; i < 3; i++ {
					updateTiledKeepGameLarge(true)
					applyTiledWindowStates()
					left, right, gameWidth := gs.TiledLeftWidth, gs.TiledRightWidth, gs.GameWindow.Size.X
					updateTiledKeepGameLarge(false)
					applyTiledWindowStates()
					if !tiledSizeNear(gs.TiledLeftWidth, left) || !tiledSizeNear(gs.TiledRightWidth, right) {
						t.Fatal("turning automatic sizing off moved the dividers")
					}
					if !updateTiledSplitter(tiledSplitterLeftWidth, (left-0.02)*1920, 1920) {
						t.Fatal("turning automatic sizing off did not unlock the divider")
					}
					applyTiledWindowStates()
					if tiledSizeNear(gs.GameWindow.Size.X, gameWidth) || !tiledSizeNear(gs.TiledRightWidth, right) {
						t.Fatal("unlocked divider moved the game instead of resizing it")
					}
				}
			})
		}
	}
}

func TestKeepGameLargeRemembersAutomaticPositionAcrossToggle(t *testing.T) {
	isolateTiledGameSize(t)
	updateTiledKeepGameLarge(true)
	applyTiledWindowStates()
	if !updateTiledSplitter(tiledSplitterLeftWidth, 1920*0.26, 1920) {
		t.Fatal("automatic game divider did not move")
	}
	applyTiledWindowStates()
	left, right, position := gs.TiledLeftWidth, gs.TiledRightWidth, gs.TiledGamePosition
	updateTiledKeepGameLarge(false)
	applyTiledWindowStates()
	updateTiledKeepGameLarge(true)
	applyTiledWindowStates()
	if !tiledSizeNear(gs.TiledLeftWidth, left) || !tiledSizeNear(gs.TiledRightWidth, right) || !tiledSizeNear(gs.TiledGamePosition, position) {
		t.Fatal("off/on without manual edits lost automatic game placement")
	}

	updateTiledKeepGameLarge(false)
	applyTiledWindowStates()
	updateTiledSplitter(tiledSplitterLeftWidth, 1920*0.40, 1920)
	applyTiledWindowStates()
	updateTiledKeepGameLarge(true)
	applyTiledWindowStates()
	if tiledSizeNear(gs.TiledGamePosition, position) {
		t.Fatal("enabling automatic sizing ignored the new manual divider placement")
	}
}

func TestUnlockedGameSizeSurvivesReload(t *testing.T) {
	isolateTiledGameSize(t)
	updateTiledKeepGameLarge(true)
	applyTiledWindowStates()
	updateTiledKeepGameLarge(false)
	applyTiledWindowStates()
	left, right := gs.TiledLeftWidth, gs.TiledRightWidth
	data, err := marshalSettingsDocument(gs)
	if err != nil {
		t.Fatal(err)
	}
	gs, err = unmarshalSettingsDocument(data, gsdef)
	if err != nil {
		t.Fatal(err)
	}
	applyTiledWindowStates()
	if gs.TiledKeepGameLarge || !tiledSizeNear(gs.TiledLeftWidth, left) || !tiledSizeNear(gs.TiledRightWidth, right) {
		t.Fatal("reloading moved or relocked the unlocked game pane")
	}
}

func TestKeepGameLargeControlsStaySynchronized(t *testing.T) {
	isolateTiledGameSize(t)
	oldTile, oldWizard := tileKeepGameLargeCB, wizardKeepGameLargeCB
	t.Cleanup(func() { tileKeepGameLargeCB, wizardKeepGameLargeCB = oldTile, oldWizard })
	tileKeepGameLargeCB, _ = eui.NewCheckbox()
	wizardKeepGameLargeCB, _ = eui.NewCheckbox()
	for _, layout := range []TiledLayout{TiledLayoutCenter, TiledLayoutSide, TiledLayoutMessagesBelow} {
		gs.TiledLayout = layout
		for _, enabled := range []bool{true, false, true} {
			gs.TiledKeepGameLarge = enabled
			refreshWindowSettingsControls()
			for _, item := range []*eui.ItemData{tileKeepGameLargeCB, wizardKeepGameLargeCB} {
				if item.Checked != enabled || item.Disabled != (layout == TiledLayoutSide) {
					t.Fatal("Auto-size side panels control does not reflect the current setting/layout")
				}
			}
		}
	}
}

func tiledSizeNear(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestKeepGameLargeRefitsAfterWorkspaceChanges(t *testing.T) {
	isolateTiledGameSize(t)
	updateTiledKeepGameLarge(true)
	applyTiledWindowStates()
	updateTiledKeepGameLarge(false)
	left, right := gs.TiledLeftWidth, gs.TiledRightWidth
	eui.SetScreenSize(2560, 1080)
	gs.TiledLayout = TiledLayoutMessagesAbove
	gs.TiledMessagesTopHeight = 0.25
	applyTiledWindowStates()
	if !tiledSizeNear(gs.TiledLeftWidth, left) || !tiledSizeNear(gs.TiledRightWidth, right) {
		t.Fatal("unlocked game widths changed with the workspace")
	}
	updateTiledKeepGameLarge(true)
	applyTiledWindowStates()
	want := float64(1080) * 0.75 * float64(gameAreaSizeX) / float64(gameAreaSizeY) / 2560
	if !tiledSizeNear(gs.GameWindow.Size.X, want) {
		t.Fatalf("re-enabled automatic width = %g, want %g for the new game height", gs.GameWindow.Size.X, want)
	}
	if !updateTiledSplitter(tiledSplitterMessagesTop, 0.20*1080, 1080) {
		t.Fatal("message row divider did not move")
	}
	applyTiledWindowStates()
	want = float64(1080) * 0.80 * float64(gameAreaSizeX) / float64(gameAreaSizeY) / 2560
	if !tiledSizeNear(gs.GameWindow.Size.X, want) {
		t.Fatal("automatic width did not follow the changed message row height")
	}
}
