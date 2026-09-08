package main

import (
	"fmt"
	"math"
	"testing"

	"gothoom/eui"
)

func TestAllTiledLayoutsFillWorkspaceWithoutOverlap(t *testing.T) {
	original, oldScale := gs, eui.UIScale()
	oldWidth, oldHeight := eui.ScreenSize()
	t.Cleanup(func() { gs = original; eui.SetUIScale(oldScale); eui.SetScreenSize(oldWidth, oldHeight) })
	for layout := TiledLayoutCenter; layout <= TiledLayoutFullMessagesAbove; layout++ {
		for _, stacked := range []bool{false, true} {
			for _, combined := range []bool{false, true} {
				for _, first := range []bool{false, true} {
					for _, large := range []bool{false, true} {
						for _, scale := range []float32{1, 2} {
							t.Run(fmt.Sprintf("%s/combined=%t/first=%t/large=%t/scale=%g/stacked=%t", tiledLayoutNames[layout], combined, first, large, scale, stacked), func(t *testing.T) {
								gs = gsdef
								eui.SetScreenSize(1920, 1080)
								eui.SetUIScale(scale)
								gs.TiledWindows, gs.TiledLayout = true, layout
								gs.MessagesToConsole, gs.TiledConsoleLeft, gs.TiledInventoryLeft = combined, first, first
								gs.TiledMessagesStacked = stacked
								gs.TiledKeepGameLarge = large
								gs.TiledGameLeft = first
								applyTiledWindowStates()
								panes := []WindowState{gs.GameWindow, gs.InventoryWindow, gs.PlayersWindow, gs.MessagesWindow}
								if !combined {
									panes = append(panes, gs.ChatWindow)
								}
								if gs.MessagesToConsole != combined || gs.ChatWindow.Open == combined {
									t.Fatal("layout changed the selected combined mode or Chat visibility")
								}
								area := 0.0
								for i, a := range panes {
									if !a.Open || a.Size.X <= 0 || a.Size.Y <= 0 || a.Position.X < 0 || a.Position.Y < 0 || a.Position.X+a.Size.X > 1+1e-9 || a.Position.Y+a.Size.Y > 1+1e-9 {
										t.Fatalf("pane %d is closed or out of bounds: %+v", i, a)
									}
									area += a.Size.X * a.Size.Y
									for _, b := range panes[:i] {
										x := math.Min(a.Position.X+a.Size.X, b.Position.X+b.Size.X) - math.Max(a.Position.X, b.Position.X)
										y := math.Min(a.Position.Y+a.Size.Y, b.Position.Y+b.Size.Y) - math.Max(a.Position.Y, b.Position.Y)
										if x > 1e-9 && y > 1e-9 {
											t.Fatal("tiled panes overlap")
										}
									}
								}
								if math.Abs(area-1) > 1e-9 {
									t.Fatalf("pane area = %g; workspace contains gaps", area)
								}
								if first != (gs.InventoryWindow.Position.X < gs.PlayersWindow.Position.X) {
									t.Fatal("list order was not applied")
								}
								if !combined {
									ordered := gs.MessagesWindow.Position.X < gs.ChatWindow.Position.X
									if layout == TiledLayoutMessagesSplit || (stacked && tiledPairedMessages()) {
										ordered = gs.MessagesWindow.Position.Y < gs.ChatWindow.Position.Y
									}
									if ordered != first {
										t.Fatal("message order was not applied")
									}
								}
								if layout == TiledLayoutMessagesAbove || layout == TiledLayoutFullMessagesAbove {
									if gs.MessagesWindow.Position.Y+gs.MessagesWindow.Size.Y > gs.GameWindow.Position.Y+1e-9 {
										t.Fatal("messages are not above the game")
									}
								}
								if layout == TiledLayoutMessagesBelow || layout == TiledLayoutFullMessagesBelow {
									if gs.MessagesWindow.Position.Y < gs.GameWindow.Position.Y+gs.GameWindow.Size.Y-1e-9 {
										t.Fatal("messages are not below the game")
									}
								}
							})
						}
					}
				}
			}
		}
	}
}

func TestMessageBandSplittersAndCombinedRecovery(t *testing.T) {
	original := gs
	t.Cleanup(func() { gs = original })
	gs = gsdef
	gs.TiledLayout, gs.TiledWindows, gs.TiledKeepGameLarge = TiledLayoutMessagesSplit, true, false
	gs.MessagesToConsole = false
	if !updateTiledSplitter(tiledSplitterMessagesTop, 250, 1000) || !updateTiledSplitter(tiledSplitterMessagesBottom, 700, 1000) {
		t.Fatal("message row dividers did not update")
	}
	applyTiledWindowStates()
	if math.Abs(gs.GameWindow.Position.Y-0.25) > 1e-9 || math.Abs(gs.GameWindow.Size.Y-0.45) > 1e-9 {
		t.Fatal("row heights did not resize the game")
	}
	gs.MessagesToConsole = true
	applyTiledWindowStates()
	if math.Abs(gs.GameWindow.Size.Y-0.75) > 1e-9 {
		t.Fatal("combining messages did not reclaim Chat's row")
	}
	gs.MessagesToConsole = false
	applyTiledWindowStates()
	if !gs.ChatWindow.Open || math.Abs(gs.GameWindow.Size.Y-0.45) > 1e-9 {
		t.Fatal("separating messages did not restore the saved row heights")
	}
	gs.TiledLayout = TiledLayoutMessagesBelow
	x, width := tiledMessageBandSpan()
	if !updateTiledSplitter(tiledSplitterMessagesSplit, (x+width*0.65)*1000, 1000) {
		t.Fatal("message width divider did not update")
	}
	applyTiledWindowStates()
	if math.Abs(gs.MessagesWindow.Size.X/(gs.MessagesWindow.Size.X+gs.ChatWindow.Size.X)-0.65) > 1e-9 {
		t.Fatal("message width divider used the whole screen instead of its row")
	}
}

func TestSideLayoutSeparateMessagesRetainSplitAfterReload(t *testing.T) {
	original := gs
	t.Cleanup(func() { gs = original })
	for _, stacked := range []bool{false, true} {
		for _, gameLeft := range []bool{false, true} {
			gs = gsdef
			gs.TiledWindows, gs.TiledLayout, gs.MessagesToConsole = true, TiledLayoutSide, false
			gs.TiledGameLeft = gameLeft
			gs.TiledMessagesStacked = stacked
			applyTiledWindowStates()
			x, width := tiledMessageBandSpan()
			if stacked {
				x, width = tiledMessageBandVerticalSpan()
			}
			if !updateTiledSplitter(tiledSplitterMessagesSplit, (x+width*0.60)*1000, 1000) {
				t.Fatal("side message divider did not update")
			}
			data, err := marshalSettingsDocument(gs)
			if err != nil {
				t.Fatal(err)
			}
			gs, err = unmarshalSettingsDocument(data, gsdef)
			if err != nil {
				t.Fatal(err)
			}
			applyTiledWindowStates()
			if gs.MessagesToConsole || !gs.ChatWindow.Open {
				t.Fatal("reloading the side layout forced combined mode")
			}
			share := gs.MessagesWindow.Size.X / (gs.MessagesWindow.Size.X + gs.ChatWindow.Size.X)
			if stacked {
				share = gs.MessagesWindow.Size.Y / (gs.MessagesWindow.Size.Y + gs.ChatWindow.Size.Y)
			}
			if gs.TiledMessagesStacked != stacked || math.Abs(share-0.60) > 1e-9 {
				t.Fatal("reloading lost the side message divider position")
			}
		}
	}
}

func TestStackedMessagePanesRespectNativeMinimumSize(t *testing.T) {
	original, oldScale := gs, eui.UIScale()
	w, h := eui.ScreenSize()
	t.Cleanup(func() { gs = original; eui.SetUIScale(oldScale); eui.SetScreenSize(w, h) })
	eui.SetScreenSize(1920, 1080)
	eui.SetUIScale(2)
	for _, layout := range []TiledLayout{TiledLayoutSide, TiledLayoutMessagesAbove, TiledLayoutMessagesBelow, TiledLayoutFullMessagesAbove, TiledLayoutFullMessagesBelow} {
		for _, share := range []float64{0.2, 0.8} {
			gs = gsdef
			gs.TiledWindows, gs.MessagesToConsole, gs.TiledMessagesStacked = true, false, true
			gs.TiledLayout, gs.TiledMessagesSplit = layout, share
			gs.TiledMessagesTopHeight, gs.TiledMessagesBottomHeight, gs.TiledRightBottom = 0.15, 0.15, 0.15
			applyTiledWindowStates()
			for _, pane := range []WindowState{gs.MessagesWindow, gs.ChatWindow} {
				if pane.Size.Y*1080/2 < eui.MinWindowSize-1e-6 {
					t.Fatalf("layout %v produced a message pane shorter than EUI's minimum: %v", layout, pane.Size.Y)
				}
			}
		}
	}
}
