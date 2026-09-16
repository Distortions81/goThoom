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
	for layout := TiledLayoutCenter; layout <= TiledLayoutSideColumns; layout++ {
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
				case TiledLayoutSideColumns:
					applySideColumnTiledWindowStates()
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

func TestTiledEditorKeepsEveryStarterWhenMessagesAreCombined(t *testing.T) {
	initFont()
	original := gs
	t.Cleanup(func() { gs = original })
	gs = gsdef
	editor := newTiledWorkspaceEditor(540)
	for _, combined := range []bool{false, true, false} {
		gs.MessagesToConsole = combined
		editor.refresh()
		if len(editor.starter.Options) != len(tiledLayoutNames)+1 {
			t.Fatal("combined mode changed the available starting arrangements")
		}
		for layout := TiledLayoutCenter; layout <= TiledLayoutSideColumns; layout++ {
			editor.starter.Handler.Emit(eui.UIEvent{Type: eui.EventDropdownSelected, Index: int(layout) + 1})
			if gs.TiledLayout != layout || editor.starter.Selected != int(layout)+1 {
				t.Fatalf("could not select starter %d with combined messages %v", layout, combined)
			}
			if editor.panes["Chat"].Invisible != combined || editor.panes["Console"].Invisible {
				t.Fatal("preview does not reflect combined-message visibility")
			}
		}
	}
}
