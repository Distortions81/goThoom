package main

import (
	"math"

	"gothoom/eui"
)

// TiledNode is either a named pane or a split. Ratios belong to the split,
// so hiding Chat can collapse its space without losing the separate layout.
type TiledNode struct {
	Pane  string     `json:"pane,omitempty"`
	Axis  string     `json:"axis,omitempty"` // x places children side by side; y stacks them.
	Ratio float64    `json:"ratio,omitempty"`
	First *TiledNode `json:"first,omitempty"`
	Last  *TiledNode `json:"last,omitempty"`
}

var tiledPaneNames = []string{"Game", "Inventory", "Players", "Console", "Chat"}

func tiledLeaf(name string) *TiledNode { return &TiledNode{Pane: name} }

func tiledBranch(axis string, ratio float64, first, last *TiledNode) *TiledNode {
	return &TiledNode{Axis: axis, Ratio: ratio, First: first, Last: last}
}

func cloneTiledNode(n *TiledNode) *TiledNode {
	if n == nil {
		return nil
	}
	copy := *n
	copy.First, copy.Last = cloneTiledNode(n.First), cloneTiledNode(n.Last)
	return &copy
}

// A workspace contains each of the five panes exactly once, even when Chat
// is combined. Reject incomplete, duplicate, or excessively nested saved data.
func validTiledTree(root *TiledNode) bool {
	seen := map[string]bool{}
	var visit func(*TiledNode, int) bool
	visit = func(n *TiledNode, depth int) bool {
		if n == nil || depth > 4 {
			return false
		}
		if n.Pane != "" {
			known := false
			for _, name := range tiledPaneNames {
				known = known || n.Pane == name
			}
			if !known || seen[n.Pane] || n.First != nil || n.Last != nil {
				return false
			}
			seen[n.Pane] = true
			return true
		}
		return (n.Axis == "x" || n.Axis == "y") && n.Ratio > 0 && n.Ratio < 1 && visit(n.First, depth+1) && visit(n.Last, depth+1)
	}
	return visit(root, 0) && len(seen) == len(tiledPaneNames)
}

// Import the existing starter layouts without changing saved preferences.
func tiledTemplateTree(s settings) *TiledNode {
	game := tiledLeaf("Game")
	list1, list2 := tiledLeaf("Inventory"), tiledLeaf("Players")
	msg1, msg2 := tiledLeaf("Console"), tiledLeaf("Chat")
	if !s.TiledInventoryLeft {
		list1, list2 = list2, list1
	}
	if !s.TiledConsoleLeft {
		msg1, msg2 = msg2, msg1
	}
	columns := func(left, center, right *TiledNode) *TiledNode {
		return tiledBranch("x", s.TiledLeftWidth, left,
			tiledBranch("x", (1-s.TiledLeftWidth-s.TiledRightWidth)/(1-s.TiledLeftWidth), center, right))
	}
	axis := "x"
	if s.TiledMessagesStacked {
		axis = "y"
	}
	pair := tiledBranch(axis, s.TiledMessagesSplit, msg1, msg2)
	switch s.TiledLayout {
	case TiledLayoutCenter:
		return columns(tiledBranch("y", 1-s.TiledLeftBottom, list1, msg1), game,
			tiledBranch("y", 1-s.TiledRightBottom, list2, msg2))
	case TiledLayoutSideColumns:
		return columns(tiledBranch("y", 1-s.TiledLeftBottom, msg1, msg2), game,
			tiledBranch("y", 1-s.TiledRightBottom, list1, list2))
	case TiledLayoutSide:
		panels := tiledBranch("y", 1-s.TiledRightBottom, tiledBranch("x", s.TiledSideTopSplit, list1, list2), pair)
		if s.TiledGameLeft {
			return tiledBranch("x", s.TiledSideGameWidth, game, panels)
		}
		return tiledBranch("x", 1-s.TiledSideGameWidth, panels, game)
	case TiledLayoutMessagesAbove:
		return columns(list1, tiledBranch("y", s.TiledMessagesTopHeight, pair, game), list2)
	case TiledLayoutMessagesBelow:
		return columns(list1, tiledBranch("y", 1-s.TiledMessagesBottomHeight, game, pair), list2)
	case TiledLayoutFullMessagesAbove:
		return tiledBranch("y", s.TiledMessagesTopHeight, pair, columns(list1, game, list2))
	case TiledLayoutFullMessagesBelow:
		return tiledBranch("y", 1-s.TiledMessagesBottomHeight, columns(list1, game, list2), pair)
	case TiledLayoutMessagesSplit:
		if !s.TiledConsoleLeft {
			// Keep Console's bottom band fixed when the top Chat band is hidden.
			return columns(list1, tiledBranch("y", 1-s.TiledMessagesBottomHeight,
				tiledBranch("y", s.TiledMessagesTopHeight/(1-s.TiledMessagesBottomHeight), msg1, game), msg2), list2)
		}
		return columns(list1, tiledBranch("y", s.TiledMessagesTopHeight, msg1,
			tiledBranch("y", (1-s.TiledMessagesTopHeight-s.TiledMessagesBottomHeight)/(1-s.TiledMessagesTopHeight), game, msg2)), list2)
	}
	return nil
}

func currentTiledTree() *TiledNode {
	if gs.TiledCustomLayout != nil {
		return gs.TiledCustomLayout
	}
	return tiledTemplateTree(gs)
}

func tiledTreeContains(n *TiledNode, pane string) bool {
	return n != nil && (n.Pane == pane || tiledTreeContains(n.First, pane) || tiledTreeContains(n.Last, pane))
}

func tiledNodeVisible(n *TiledNode, combined bool) bool {
	return n != nil && !(combined && n.Pane == "Chat")
}

type tiledRect struct{ x, y, w, h float64 }
type tiledTreeDivider struct {
	path  string
	axis  string
	rect  tiledRect
	ratio float64
}

// Minimums are computed for whole subtrees, including the toolbar's host.
// Ratios stay untouched when a smaller screen temporarily constrains a split.
func tiledTreeMinimum(n *TiledNode, combined bool, toolbar string) (w, h float64) {
	if !tiledNodeVisible(n, combined) {
		return 0, 0
	}
	if n.Pane != "" {
		w, h = eui.MinWindowSize, eui.MinWindowSize
		if n.Pane == toolbar {
			w = dockedToolbarMinimumWidth
		}
		return
	}
	w1, h1 := tiledTreeMinimum(n.First, combined, toolbar)
	w2, h2 := tiledTreeMinimum(n.Last, combined, toolbar)
	if n.Axis == "x" {
		return w1 + w2, max(h1, h2)
	}
	return max(w1, w2), h1 + h2
}

func tiledTreeGeometry(root *TiledNode, combined bool, width, height float64, toolbar string) (map[string]tiledRect, []tiledTreeDivider) {
	panes := map[string]tiledRect{}
	var dividers []tiledTreeDivider
	var visit func(*TiledNode, tiledRect, string)
	visit = func(n *TiledNode, r tiledRect, path string) {
		if !tiledNodeVisible(n, combined) {
			return
		}
		if n.Pane != "" {
			panes[n.Pane] = r
			return
		}
		if !tiledNodeVisible(n.First, combined) {
			visit(n.Last, r, path+"1")
			return
		}
		if !tiledNodeVisible(n.Last, combined) {
			visit(n.First, r, path+"0")
			return
		}
		w1, h1 := tiledTreeMinimum(n.First, combined, toolbar)
		w2, h2 := tiledTreeMinimum(n.Last, combined, toolbar)
		a, b, extent := w1, w2, r.w*width
		if n.Axis == "y" {
			a, b, extent = h1, h2, r.h*height
		}
		ratio := n.Ratio
		if extent > 0 {
			if extent < a+b {
				ratio = a / (a + b)
			} else {
				ratio = math.Max(a/extent, math.Min(1-b/extent, ratio))
			}
		}
		dividers = append(dividers, tiledTreeDivider{path, n.Axis, r, ratio})
		first, last := r, r
		if n.Axis == "x" {
			first.w = r.w * ratio
			last.x, last.w = r.x+first.w, r.w-first.w
		} else {
			first.h = r.h * ratio
			last.y, last.h = r.y+first.h, r.h-first.h
		}
		visit(n.First, first, path+"0")
		visit(n.Last, last, path+"1")
	}
	visit(root, tiledRect{0, 0, 1, 1}, "")
	return panes, dividers
}

func tiledToolbarPane() string {
	switch gs.ToolbarPlacement {
	case ToolbarInInventory:
		return "Inventory"
	case ToolbarInPlayers:
		return "Players"
	}
	return ""
}

func currentTiledGeometry() (map[string]tiledRect, []tiledTreeDivider) {
	w, h := eui.ScreenSize()
	scale := float64(eui.UIScale())
	if w <= 0 || h <= 0 || scale <= 0 {
		w, h, scale = 1920, 1080, 1
	}
	return tiledTreeGeometry(currentTiledTree(), gs.MessagesToConsole, float64(w)/scale, float64(h)/scale, tiledToolbarPane())
}

func applyCustomTiledWindowStates() {
	panes, _ := currentTiledGeometry()
	for name, state := range map[string]*WindowState{"Game": &gs.GameWindow, "Inventory": &gs.InventoryWindow, "Players": &gs.PlayersWindow, "Console": &gs.MessagesWindow, "Chat": &gs.ChatWindow} {
		r, open := panes[name]
		state.Open = open
		if open {
			tiledWindowState(state, r.x, r.y, r.w, r.h)
		}
	}
}

// All edits replace the tree, preserving settings snapshots and Undo history.
func editTiledTree(root *TiledNode, source, target, action string) *TiledNode {
	if source == target || !tiledTreeContains(root, source) || !tiledTreeContains(root, target) {
		return nil
	}
	next := cloneTiledNode(root)
	if action == "Swap" {
		var swap func(*TiledNode)
		swap = func(n *TiledNode) {
			if n == nil {
				return
			}
			switch n.Pane {
			case source:
				n.Pane = target
			case target:
				n.Pane = source
			}
			swap(n.First)
			swap(n.Last)
		}
		swap(next)
		return next
	}
	if action != "Left of" && action != "Right of" && action != "Above" && action != "Below" {
		return nil
	}
	var remove func(*TiledNode) *TiledNode
	remove = func(n *TiledNode) *TiledNode {
		if n.Pane == source {
			return nil
		}
		if n.Pane != "" {
			return n
		}
		n.First, n.Last = remove(n.First), remove(n.Last)
		if n.First == nil {
			return n.Last
		}
		if n.Last == nil {
			return n.First
		}
		return n
	}
	next = remove(next)
	var insert func(*TiledNode) *TiledNode
	insert = func(n *TiledNode) *TiledNode {
		if n.Pane == target {
			axis := "x"
			if action == "Above" || action == "Below" {
				axis = "y"
			}
			if action == "Right of" || action == "Below" {
				return tiledBranch(axis, .5, n, tiledLeaf(source))
			}
			return tiledBranch(axis, .5, tiledLeaf(source), n)
		}
		if n.Pane == "" {
			n.First, n.Last = insert(n.First), insert(n.Last)
		}
		return n
	}
	return insert(next)
}

func setCustomTiledRatio(path string, ratio float64) {
	root := cloneTiledNode(gs.TiledCustomLayout)
	n := root
	for _, step := range path {
		if n == nil {
			return
		}
		if step == '0' {
			n = n.First
		} else {
			n = n.Last
		}
	}
	if n == nil || n.Pane != "" || math.IsNaN(ratio) {
		return
	}
	n.Ratio = math.Max(.02, math.Min(.98, ratio))
	gs.TiledCustomLayout = root
}

func customTiledDividers(width, height int) []eui.TileDivider {
	_, splits := currentTiledGeometry()
	dividers := make([]eui.TileDivider, 0, len(splits))
	for _, split := range splits {
		r := split.rect
		d := eui.TileDivider{Orientation: eui.TileDividerHorizontal, Position: float32((r.y + r.h*split.ratio) * float64(height)), Start: float32(r.x * float64(width)), End: float32((r.x + r.w) * float64(width)), Thickness: 5 * eui.UIScale(), HitSize: 16 * eui.UIScale()}
		start, extent := r.y*float64(height), r.h*float64(height)
		if split.axis == "x" {
			d.Orientation = eui.TileDividerVertical
			d.Position, d.Start, d.End = float32((r.x+r.w*split.ratio)*float64(width)), float32(r.y*float64(height)), float32((r.y+r.h)*float64(height))
			start, extent = r.x*float64(width), r.w*float64(width)
		}
		d.OnDrag = func(position float32) {
			if extent > 0 {
				setCustomTiledRatio(split.path, (float64(position)-start)/extent)
				applyManagedWindowLayout()
				settingsDirty = true
			}
		}
		dividers = append(dividers, d)
	}
	return dividers
}
