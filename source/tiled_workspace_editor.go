package main

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"gothoom/eui"
)

var tileWorkspaceEditor, wizardWorkspaceEditor *tiledWorkspaceEditor

type tiledWorkspaceSnapshot struct {
	tree                                                        *TiledNode
	layout                                                      TiledLayout
	large, inventoryFirst, consoleFirst, gameLeft, stacked      bool
	position, leftWidth, rightWidth, leftBottom, rightBottom    float64
	sideWidth, sideSplit, topHeight, bottomHeight, messageSplit float64
}

func captureTiledWorkspace() tiledWorkspaceSnapshot {
	return tiledWorkspaceSnapshot{cloneTiledNode(gs.TiledCustomLayout), gs.TiledLayout, gs.TiledKeepGameLarge, gs.TiledInventoryLeft, gs.TiledConsoleLeft, gs.TiledGameLeft, gs.TiledMessagesStacked,
		gs.TiledGamePosition, gs.TiledLeftWidth, gs.TiledRightWidth, gs.TiledLeftBottom, gs.TiledRightBottom, gs.TiledSideGameWidth, gs.TiledSideTopSplit, gs.TiledMessagesTopHeight, gs.TiledMessagesBottomHeight, gs.TiledMessagesSplit}
}

func (s tiledWorkspaceSnapshot) restore() {
	gs.TiledCustomLayout, gs.TiledLayout = cloneTiledNode(s.tree), s.layout
	gs.TiledKeepGameLarge, gs.TiledInventoryLeft, gs.TiledConsoleLeft, gs.TiledGameLeft, gs.TiledMessagesStacked = s.large, s.inventoryFirst, s.consoleFirst, s.gameLeft, s.stacked
	gs.TiledGamePosition, gs.TiledLeftWidth, gs.TiledRightWidth, gs.TiledLeftBottom, gs.TiledRightBottom = s.position, s.leftWidth, s.rightWidth, s.leftBottom, s.rightBottom
	gs.TiledSideGameWidth, gs.TiledSideTopSplit, gs.TiledMessagesTopHeight, gs.TiledMessagesBottomHeight, gs.TiledMessagesSplit = s.sideWidth, s.sideSplit, s.topHeight, s.bottomHeight, s.messageSplit
}

type tiledWorkspaceEditor struct {
	root, canvas, starter, action, status, undo *eui.ItemData
	panes                                       map[string]*eui.ItemData
	selected                                    string
	history                                     []tiledWorkspaceSnapshot
	thumbnailsReady, combined                   bool
}

func selectTiledStarter(layout TiledLayout) {
	gs.TiledCustomLayout, gs.TiledLayout = nil, layout
	gs.TiledInventoryLeft, gs.TiledConsoleLeft, gs.TiledGameLeft = true, true, true
	gs.TiledMessagesStacked, gs.TiledMessagesSplit = false, .5
	gs.TiledLeftBottom, gs.TiledRightBottom = .3, .3
	gs.TiledSideGameWidth, gs.TiledSideTopSplit = .6, .5
	gs.TiledLeftWidth, gs.TiledRightWidth, gs.TiledGamePosition = .25, .25, 0
	gs.TiledMessagesTopHeight, gs.TiledMessagesBottomHeight = .25, .25
	if layout == TiledLayoutSideColumns {
		gs.TiledLeftBottom, gs.TiledRightBottom = .5, .5
	}
}

func newTiledWorkspaceEditor(width float32) *tiledWorkspaceEditor {
	ed := &tiledWorkspaceEditor{root: eui.NewColumn(), panes: map[string]*eui.ItemData{}}
	ed.root.Name = "tiled-workspace-editor"
	ed.starter, _ = eui.NewDropdown()
	ed.starter.Name, ed.starter.Label = "tiled-starter", "Start with"
	ed.starter.Options = append([]string{"Current arrangement"}, tiledLayoutNames...)
	ed.starter.Size = eui.Point{X: width, Y: 26}
	ed.starter.MaxVisible = len(ed.starter.Options)
	ed.starter.SetTooltip("Choose a starting arrangement, then move any pane in the preview. Undo restores your previous arrangement.")
	// Starter thumbnails have fixed, predictable pane order.
	for index := range ed.starter.Options {
		var img *ebiten.Image
		if index > 0 {
			img = ebiten.NewImage(332, 160)
			drawTiledPreviewPanes(img, tiledPreviewPanes(TiledLayout(index-1), gs.MessagesToConsole, false, true, true, true), false)
		}
		ed.starter.OptionImages = append(ed.starter.OptionImages, img)
	}
	ed.starter.Handler.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventDropdownSelected || ev.Index < 0 || ev.Index >= len(ed.starter.Options) {
			return
		}
		if ev.Index == 0 {
			ed.refresh()
			return
		}
		ed.remember()
		selectTiledStarter(TiledLayout(ev.Index - 1))
		ed.selected = ""
		ed.apply()
	}
	ed.root.AddItem(ed.starter)

	ed.status = eui.NewLabel("")
	ed.status.FontSize = 11
	ed.status.Size = eui.Point{X: width, Y: 32}
	ed.status.Position.Y = 8
	ed.root.AddItem(ed.status)

	ed.canvas = eui.NewOverlay()
	ed.canvas.Name = "tiled-workspace-preview"
	ed.canvas.Size = eui.Point{X: width, Y: width * .48}
	ed.canvas.Fixed, ed.canvas.ConstrainToSize = true, true
	for _, name := range tiledPaneNames {
		pane, events := eui.NewButton()
		pane.Name = "tiled-pane-" + name
		pane.FontSize = 12
		pane.ConstrainToSize = true
		events.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				ed.clickPane(name)
			}
		}
		ed.panes[name] = pane
		ed.canvas.AddItem(pane)
	}
	ed.root.AddItem(ed.canvas)

	ed.action, _ = eui.NewDropdown()
	ed.action.Name, ed.action.Label = "tiled-edit-action", "Move selected pane"
	ed.action.Options = []string{"Swap", "Left of", "Right of", "Above", "Below"}
	ed.action.Size = eui.Point{X: width, Y: 26}
	ed.action.Position.Y = 10
	ed.action.Handler.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected && ev.Index >= 0 && ev.Index < len(ed.action.Options) {
			ed.action.Selected = ev.Index
			ed.refresh()
			if ed.root.ParentWindow != nil {
				ed.root.ParentWindow.Refresh()
			}
		}
	}
	ed.root.AddItem(ed.action)

	even := eui.NewActionButton("Even splits", func() {
		ed.remember()
		root := cloneTiledNode(currentTiledTree())
		var balance func(*TiledNode)
		balance = func(n *TiledNode) {
			if n == nil || n.Pane != "" {
				return
			}
			n.Ratio = .5
			// Keep three columns equal on either side of the centered game.
			if n.Axis == "x" && n.Last != nil && n.Last.Axis == "x" && n.Last.First.Pane == "Game" {
				n.Ratio = .25
				balance(n.First)
				balance(n.Last)
				n.Last.Ratio = 2.0 / 3
				return
			}
			balance(n.First)
			balance(n.Last)
		}
		balance(root)
		gs.TiledCustomLayout = root
		ed.apply()
	})
	even.Name = "tiled-even-splits"
	even.Size = eui.Point{X: (width - 8) / 2, Y: 26}
	even.SetTooltip("Equalize paired panes. Centered layouts keep more space for the game; the toolbar may require a wider pane.")
	ed.undo = eui.NewActionButton("Undo", func() {
		if len(ed.history) == 0 {
			return
		}
		last := len(ed.history) - 1
		ed.history[last].restore()
		ed.history = ed.history[:last]
		ed.selected = ""
		ed.apply()
	})
	ed.undo.Name = "tiled-undo"
	ed.undo.Size = eui.Point{X: (width - 8) / 2, Y: 26}
	ed.undo.Position.X = 8
	row := eui.NewRow(even, ed.undo)
	row.Position.Y = 10
	ed.root.AddItem(row)
	note := eui.NewLabel("Drag dividers in the workspace to resize panes.")
	note.FontSize = 10
	note.Position.Y = 8
	ed.root.AddItem(note)
	ed.refresh()
	return ed
}

func (ed *tiledWorkspaceEditor) remember() {
	ed.history = append(ed.history, captureTiledWorkspace())
	if len(ed.history) > 20 {
		ed.history = ed.history[1:]
	}
}

func (ed *tiledWorkspaceEditor) clickPane(name string) {
	if name == "Chat" && gs.MessagesToConsole {
		return
	}
	if ed.selected == "" || ed.selected == name {
		if ed.selected == name {
			ed.selected = ""
		} else {
			ed.selected = name
		}
		ed.refresh()
		if ed.root.ParentWindow != nil {
			ed.root.ParentWindow.Refresh()
		}
		return
	}
	root := editTiledTree(currentTiledTree(), ed.selected, name, ed.action.Options[ed.action.Selected])
	if !validTiledTree(root) {
		return
	}
	ed.remember()
	gs.TiledCustomLayout = root
	ed.selected = ""
	ed.apply()
}

func (ed *tiledWorkspaceEditor) apply() {
	applyTiledWorkspaceLayout()
	ed.refresh()
	if ed.root.ParentWindow != nil {
		ed.root.ParentWindow.Refresh()
	}
}

func (ed *tiledWorkspaceEditor) refresh() {
	if ed == nil {
		return
	}
	panes, _ := currentTiledGeometry()
	if _, ok := panes[ed.selected]; !ok {
		ed.selected = ""
	}
	ed.starter.Selected = 0
	if gs.TiledCustomLayout == nil {
		ed.starter.Selected = int(gs.TiledLayout) + 1
	}
	if !ed.thumbnailsReady || ed.combined != gs.MessagesToConsole {
		for index, img := range ed.starter.OptionImages {
			if img != nil {
				drawTiledPreviewPanes(img, tiledPreviewPanes(TiledLayout(index-1), gs.MessagesToConsole, false, true, true, true), false)
			}
		}
		ed.thumbnailsReady, ed.combined = true, gs.MessagesToConsole
	}
	ed.starter.Dirty = true
	for name, button := range ed.panes {
		r, visible := panes[name]
		button.Invisible, button.Checked = !visible, name == ed.selected
		button.SelectionIndicator = button.Checked
		button.Text = name
		if name == "Console" && gs.MessagesToConsole {
			button.Text = "Chat +\nConsole"
		}
		button.Position = eui.Point{X: float32(r.x) * ed.canvas.Size.X, Y: float32(r.y) * ed.canvas.Size.Y}
		button.Size = eui.Point{X: max(1, float32(r.w)*ed.canvas.Size.X-4), Y: max(1, float32(r.h)*ed.canvas.Size.Y-4)}
		button.FitButtonCaption(12)
		button.Dirty = true
	}
	ed.status.Text = "Click a pane, then another to swap their places."
	if ed.selected != "" {
		name := ed.selected
		if name == "Console" && gs.MessagesToConsole {
			name = "Chat + Console"
		}
		if ed.action.Selected == 0 {
			ed.status.Text = fmt.Sprintf("%s selected. Click a pane to swap with it.", name)
		} else {
			ed.status.Text = fmt.Sprintf("%s selected. Click a target for %s.", name, ed.action.Options[ed.action.Selected])
		}
	} else if ed.action.Selected != 0 {
		ed.status.Text = "Click the pane to move, then its target."
	}
	ed.status.Dirty = true
	ed.undo.Disabled, ed.undo.Dirty = len(ed.history) == 0, true
}
