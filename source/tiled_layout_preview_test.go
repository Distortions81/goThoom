package main

import (
	"fmt"
	"math"
	"testing"

	"gothoom/eui"
)

func TestTiledPreviewsMatchWorkspaceTopology(t *testing.T) {
	original := gs
	t.Cleanup(func() { gs = original })
	for layout := TiledLayoutCenter; layout <= TiledLayoutFullMessagesAbove; layout++ {
		for flags := 0; flags < 32; flags++ {
			t.Run(fmt.Sprintf("layout=%d/flags=%d", layout, flags), func(t *testing.T) {
				gs = gsdef
				gs.TiledLayout = layout
				gs.MessagesToConsole = flags&1 != 0
				gs.TiledMessagesStacked = flags&2 != 0
				gs.TiledInventoryLeft = flags&4 != 0
				gs.TiledConsoleLeft = flags&8 != 0
				gs.TiledGameLeft = flags&16 != 0
				gs.TiledLeftWidth, gs.TiledRightWidth = .25, .25
				gs.TiledSideGameWidth, gs.TiledSideTopSplit = .5, .5
				gs.TiledLeftBottom, gs.TiledRightBottom = 1.0/3, 1.0/3
				gs.TiledMessagesTopHeight, gs.TiledMessagesBottomHeight = 1.0/3, 1.0/3
				gs.TiledMessagesSplit = .5
				before := gs
				panes := tiledPreviewPanes(layout, gs.MessagesToConsole, gs.TiledMessagesStacked, gs.TiledInventoryLeft, gs.TiledConsoleLeft, gs.TiledGameLeft)
				if gs.TiledLayout != before.TiledLayout || gs.GameWindow != before.GameWindow {
					t.Fatal("preview changed live workspace")
				}
				switch layout {
				case TiledLayoutCenter:
					applyCenteredTiledWindowStates()
				case TiledLayoutSide:
					applySideTiledWindowStates()
				default:
					applyMessageBandTiledWindowStates()
				}
				expected := map[string]WindowState{"Game": gs.GameWindow, "Inv": gs.InventoryWindow, "Players": gs.PlayersWindow, "Console": gs.MessagesWindow}
				if gs.MessagesToConsole {
					delete(expected, "Console")
					expected["Chat +\nConsole"] = gs.MessagesWindow
				} else {
					expected["Chat"] = gs.ChatWindow
				}
				if len(panes) != len(expected) {
					t.Fatalf("got %d panes, want %d", len(panes), len(expected))
				}
				for _, pane := range panes {
					want, ok := expected[pane.name]
					if !ok {
						t.Fatalf("unexpected pane %q", pane.name)
					}
					gotRect := [4]float64{pane.x, pane.y, pane.w, pane.h}
					wantRect := [4]float64{want.Position.X, want.Position.Y, want.Size.X, want.Size.Y}
					for i := range gotRect {
						if math.Abs(gotRect[i]-wantRect[i]) > 1e-9 {
							t.Fatalf("%s preview %v, workspace %v", pane.name, gotRect, wantRect)
						}
					}
				}
			})
		}
	}
}

func TestTiledPreviewSelectionAndExternalRefresh(t *testing.T) {
	initFont()
	original, oldWindow := gs, tileLayoutWin
	gs = gsdef
	gs.MessagesToConsole = false
	tileLayoutWin = nil
	t.Cleanup(func() { tileLayoutWin.RemoveWindow(); tileLayoutWin = oldWindow; gs = original })
	makeTileLayoutWindow()
	var buttons []*eui.ItemData
	var visit func([]*eui.ItemData)
	visit = func(items []*eui.ItemData) {
		for _, it := range items {
			if it.Name == "tiled-layout-preview" {
				buttons = append(buttons, it)
			}
			if it.Label == "Layout" {
				t.Fatal("layout window still has a redundant layout dropdown")
			}
			visit(it.Contents)
		}
	}
	visit(tileLayoutWin.Contents)
	if len(buttons) != len(tiledLayoutChoices) {
		t.Fatalf("got %d layout choices", len(buttons))
	}
	for index, button := range buttons {
		button.Handler.Emit(eui.UIEvent{Type: eui.EventClick, Item: button})
		if gs.TiledLayout != tiledLayoutChoices[index].layout {
			t.Fatal("click did not select the layout")
		}
		for other, item := range buttons {
			if item.Checked != (index == other) {
				t.Fatal("wrong selected preview")
			}
		}
	}
	gs.TiledLayout = TiledLayoutCenter
	gs.MessagesToConsole = !gs.MessagesToConsole
	refreshWindowSettingsControls()
	if !buttons[0].Checked || tileCombineMessagesCB.Checked != gs.MessagesToConsole {
		t.Fatal("external settings change did not refresh chooser")
	}
}

func TestTiledArrangementCheckboxesAndRowVariants(t *testing.T) {
	initFont()
	original := gs
	t.Cleanup(func() { gs = original })
	gs = gsdef
	gs.MessagesToConsole = false
	root := newTiledArrangementControls(310, nil)
	controls := map[string]*eui.ItemData{}
	for _, it := range root.Contents {
		controls[it.Name] = it
	}
	for layout := TiledLayoutCenter; layout <= TiledLayoutFullMessagesAbove; layout++ {
		for _, combined := range []bool{false, true} {
			gs.TiledLayout, gs.MessagesToConsole = layout, combined
			refreshTiledArrangementControls(root.Contents)
			for name, enabled := range map[string]bool{
				"tiled-swap-game":      layout == TiledLayoutSide,
				"tiled-swap-lists":     true,
				"tiled-swap-messages":  !combined || layout == TiledLayoutCenter,
				"tiled-messages-above": tiledLayoutChoiceIndex() == 2 || tiledLayoutChoiceIndex() == 3,
				"tiled-message-split":  !combined && tiledPairedMessages(),
			} {
				it := controls[name]
				if it.Invisible || it.Disabled == enabled {
					t.Fatalf("layout %d combined %t: %s availability does not match", layout, combined, name)
				}
			}
			// Opening the controls must not rewrite a saved layout or pane order.
			if gs.TiledLayout != layout {
				t.Fatal("displaying layout choices changed the saved layout")
			}
		}
	}
	gs.MessagesToConsole = false
	gs.TiledLayout = TiledLayoutSide
	refreshTiledArrangementControls(root.Contents)
	for _, name := range []string{"tiled-swap-game", "tiled-swap-lists", "tiled-swap-messages"} {
		it := controls[name]
		it.Handler.Emit(eui.UIEvent{Item: it, Type: eui.EventCheckboxChanged, Checked: true})
	}
	if gs.TiledGameLeft || gs.TiledInventoryLeft || gs.TiledConsoleLeft {
		t.Fatal("swap checkboxes did not update pane order")
	}
	for _, pair := range [][2]TiledLayout{
		{TiledLayoutMessagesBelow, TiledLayoutMessagesAbove},
		{TiledLayoutFullMessagesBelow, TiledLayoutFullMessagesAbove},
	} {
		gs.TiledLayout = pair[0]
		refreshTiledArrangementControls(root.Contents)
		above := controls["tiled-messages-above"]
		for _, checked := range []bool{true, false} {
			above.Handler.Emit(eui.UIEvent{Item: above, Type: eui.EventCheckboxChanged, Checked: checked})
			want := pair[0]
			if checked {
				want = pair[1]
			}
			if gs.TiledLayout != want || gs.TiledGameLeft || gs.TiledInventoryLeft || gs.TiledConsoleLeft {
				t.Fatal("moving the message row changed another pane's order")
			}
		}
	}
	gs.MessagesToConsole = true
	refreshTiledArrangementControls(root.Contents)
	messageSwap := controls["tiled-swap-messages"]
	messageSwap.Handler.Emit(eui.UIEvent{Item: messageSwap, Type: eui.EventCheckboxChanged, Checked: false})
	if gs.TiledConsoleLeft {
		t.Fatal("unavailable swap control changed the combined pane position")
	}
	gs.TiledLayout = TiledLayoutCenter
	refreshTiledArrangementControls(root.Contents)
	if messageSwap.Disabled || messageSwap.Text != "Combined messages on right" {
		t.Fatal("centered combined messages have no side control")
	}
	messageSwap.Handler.Emit(eui.UIEvent{Item: messageSwap, Type: eui.EventCheckboxChanged, Checked: false})
	if !gs.TiledConsoleLeft {
		t.Fatal("combined message pane did not move left")
	}
	messageSwap.Handler.Emit(eui.UIEvent{Item: messageSwap, Type: eui.EventCheckboxChanged, Checked: true})
	if gs.TiledConsoleLeft {
		t.Fatal("combined message pane did not move right")
	}
}
