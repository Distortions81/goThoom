package main

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"

	"gothoom/eui"
)

func TestCustomTiledMovesPreserveEveryPaneAndFillWorkspace(t *testing.T) {
	for layout := TiledLayoutCenter; layout <= TiledLayoutSideColumns; layout++ {
		s := gsdef
		s.TiledLayout = layout
		root := tiledTemplateTree(s)
		original, _ := json.Marshal(root)
		for _, source := range tiledPaneNames {
			for _, target := range tiledPaneNames {
				if source == target {
					continue
				}
				for _, action := range []string{"Swap", "Left of", "Right of", "Above", "Below"} {
					t.Run(fmt.Sprintf("%d/%s/%s/%s", layout, source, action, target), func(t *testing.T) {
						next := editTiledTree(root, source, target, action)
						if !validTiledTree(next) {
							t.Fatal("edit lost or duplicated panes")
						}
						for _, combined := range []bool{false, true} {
							for _, scale := range []float64{1, 2} {
								panes, dividers := tiledTreeGeometry(next, combined, 1920/scale, 1080/scale, "Inventory")
								want := 5
								if combined {
									want = 4
								}
								if len(panes) != want || len(dividers) != want-1 {
									t.Fatal("wrong visible pane or divider count")
								}
								area := 0.0
								for name, a := range panes {
									if a.x < 0 || a.y < 0 || a.x+a.w > 1+1e-9 || a.y+a.h > 1+1e-9 {
										t.Fatal("pane outside workspace")
									}
									if a.w*1920/scale < eui.MinWindowSize-1e-9 || a.h*1080/scale < eui.MinWindowSize-1e-9 {
										t.Fatal("pane below native minimum")
									}
									if name == "Inventory" && a.w*1920/scale < dockedToolbarMinimumWidth-1e-9 {
										t.Fatal("toolbar host too narrow")
									}
									area += a.w * a.h
									for other, b := range panes {
										if name == other {
											continue
										}
										if math.Min(a.x+a.w, b.x+b.w)-max(a.x, b.x) > 1e-9 && math.Min(a.y+a.h, b.y+b.h)-max(a.y, b.y) > 1e-9 {
											t.Fatal("panes overlap")
										}
									}
								}
								if math.Abs(area-1) > 1e-9 {
									t.Fatal("workspace contains gaps")
								}
							}
						}
						after, _ := json.Marshal(root)
						if string(after) != string(original) {
							t.Fatal("edit mutated the original layout")
						}
					})
				}
			}
		}
	}
}

func TestTiledEditorImportsSavedLayoutsWithoutMovingPanes(t *testing.T) {
	original, scale := gs, eui.UIScale()
	w, h := eui.ScreenSize()
	t.Cleanup(func() { gs = original; eui.SetUIScale(scale); eui.SetScreenSize(w, h) })
	eui.SetScreenSize(1920, 1080)
	for _, scale := range []float32{1, 2} {
		eui.SetUIScale(scale)
		for layout := TiledLayoutCenter; layout <= TiledLayoutSideColumns; layout++ {
			for flags := 0; flags < 32; flags++ {
				gs = gsdef
				gs.TiledLayout = layout
				gs.MessagesToConsole, gs.TiledConsoleLeft = flags&1 != 0, flags&2 != 0
				gs.TiledInventoryLeft, gs.TiledGameLeft, gs.TiledMessagesStacked = flags&4 != 0, flags&8 != 0, flags&16 != 0
				applyTiledWindowStates()
				panes, _ := currentTiledGeometry()
				for name, state := range map[string]WindowState{"Game": gs.GameWindow, "Inventory": gs.InventoryWindow, "Players": gs.PlayersWindow, "Console": gs.MessagesWindow, "Chat": gs.ChatWindow} {
					if !state.Open {
						continue
					}
					r := panes[name]
					if math.Abs(r.x-state.Position.X) > 1e-9 || math.Abs(r.y-state.Position.Y) > 1e-9 || math.Abs(r.w-state.Size.X) > 1e-9 || math.Abs(r.h-state.Size.Y) > 1e-9 {
						t.Fatalf("layout=%d flags=%d scale=%g: %s preview %+v differs from saved pane %+v", layout, flags, scale, name, r, state)
					}
				}
			}
		}
	}
}

func TestCustomTiledLayoutPersistsAndRestoresHiddenChat(t *testing.T) {
	original := gs
	t.Cleanup(func() { gs = original })
	gs = gsdef
	gs.TiledWindows, gs.MessagesToConsole = true, false
	gs.TiledCustomLayout = editTiledTree(tiledTemplateTree(gs), "Chat", "Console", "Above")
	applyTiledWindowStates()
	want := []WindowState{gs.GameWindow, gs.InventoryWindow, gs.PlayersWindow, gs.MessagesWindow, gs.ChatWindow}
	gs.MessagesToConsole = true
	applyTiledWindowStates()
	if gs.ChatWindow.Open {
		t.Fatal("combined Chat is still open")
	}
	data, err := marshalSettingsDocument(gs)
	if err != nil {
		t.Fatal(err)
	}
	gs, err = unmarshalSettingsDocument(data, gsdef)
	if err != nil {
		t.Fatal(err)
	}
	if !validTiledTree(gs.TiledCustomLayout) {
		t.Fatal("custom arrangement was not saved")
	}
	gs.MessagesToConsole = false
	applyTiledWindowStates()
	got := []WindowState{gs.GameWindow, gs.InventoryWindow, gs.PlayersWindow, gs.MessagesWindow, gs.ChatWindow}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("separating messages did not restore the saved arrangement")
	}
	resetSavedWindowSettings()
	if gs.TiledCustomLayout != nil {
		t.Fatal("Reset Windows retained the custom arrangement")
	}
}

func TestCustomTiledInvalidSavedTreeFallsBack(t *testing.T) {
	original := gs
	t.Cleanup(func() { gs = original })
	for _, tree := range []*TiledNode{
		{}, tiledLeaf("Chat"), tiledBranch("z", .5, tiledLeaf("Game"), tiledLeaf("Chat")),
		tiledBranch("x", math.NaN(), tiledLeaf("Game"), tiledLeaf("Chat")),
	} {
		gs = gsdef
		gs.TiledCustomLayout = tree
		clampTiledLayoutSettings()
		if gs.TiledCustomLayout != nil {
			t.Fatal("invalid custom tree was retained")
		}
	}
}

func TestCustomTiledDividerTargetsSavedSplitInCombinedMode(t *testing.T) {
	original, scale := gs, eui.UIScale()
	w, h := eui.ScreenSize()
	t.Cleanup(func() { gs = original; eui.SetUIScale(scale); eui.SetScreenSize(w, h) })
	gs = gsdef
	gs.MessagesToConsole = true
	gs.TiledCustomLayout = tiledTemplateTree(gs)
	eui.SetScreenSize(1920, 1080)
	eui.SetUIScale(1)
	before := cloneTiledNode(gs.TiledCustomLayout)
	_, dividers := currentTiledGeometry()
	for _, divider := range dividers {
		setCustomTiledRatio(divider.path, .6)
	}
	if reflect.DeepEqual(before, gs.TiledCustomLayout) {
		t.Fatal("divider edits were not saved")
	}
	if !validTiledTree(gs.TiledCustomLayout) {
		t.Fatal("divider edited a hidden leaf instead of a split")
	}
	if !reflect.DeepEqual(before, tiledTemplateTree(gs)) {
		t.Fatal("divider mutated the source layout")
	}
}

func TestTiledEditorSwapMoveAndUndo(t *testing.T) {
	initFont()
	original := gs
	t.Cleanup(func() { gs = original })
	gs = gsdef
	gs.MessagesToConsole = false
	ed := newTiledWorkspaceEditor(540)
	applyTiledWindowStates()
	before := captureTiledWorkspace()
	ed.clickPane("Inventory")
	if !ed.panes["Inventory"].Checked {
		t.Fatal("selected pane is not highlighted")
	}
	ed.clickPane("Chat")
	if gs.TiledCustomLayout == nil || ed.undo.Disabled {
		t.Fatal("swap did not produce an undoable custom layout")
	}
	ed.undo.Handler.Emit(eui.UIEvent{Type: eui.EventClick})
	if !reflect.DeepEqual(captureTiledWorkspace(), before) {
		t.Fatal("Undo did not restore the original layout preferences")
	}
	ed.action.Selected = 3 // Above
	ed.clickPane("Chat")
	ed.clickPane("Console")
	panes, _ := currentTiledGeometry()
	if panes["Chat"].x != panes["Console"].x || panes["Chat"].y >= panes["Console"].y {
		t.Fatal("Above did not stack Chat over Console")
	}
}
