package eui

import (
	"testing"

	"golang.org/x/image/font/gofont/goregular"
)

func TestContextMenuStartsAtAnchorAndMatchesHitTargets(t *testing.T) {
	isolateThemeTest(t)
	if err := EnsureFontSource(goregular.TTF); err != nil {
		t.Fatal(err)
	}
	oldHeight, oldWidth, oldScale := screenHeight, screenWidth, uiScale
	t.Cleanup(func() {
		screenHeight, screenWidth, uiScale = oldHeight, oldWidth, oldScale
		CloseContextMenus()
	})
	screenHeight, screenWidth = 1200, 1200
	for _, scale := range []float32{1, 1.3, 2} {
		uiScale = scale
		CloseContextMenus()
		selected := -1
		anchor := point{X: 100, Y: 200} // The bottom-left corner of a source button.
		menu := ShowContextMenu([]string{"Flute", "Harp", "Bass"}, anchor.X, anchor.Y, func(index int) {
			selected = index
		})
		bounds, visible := dropdownOpenRect(menu, anchor)
		if bounds.X0 != anchor.X || bounds.Y0 != anchor.Y || visible != 3 {
			t.Fatalf("%.1fx context menu left a gap at its anchor: %+v", scale, bounds)
		}
		rowHeight := dropdownOptionHeight(menu)
		pointer := point{X: bounds.X0 + 1, Y: bounds.Y0 + rowHeight*1.5}
		handleContextMenus(pointer, false)
		if menu.HoverIndex != 1 {
			t.Fatalf("%.1fx menu hover does not match the second visible row", scale)
		}
		handleContextMenus(pointer, true)
		if selected != 1 || ContextMenusOpen() {
			t.Fatalf("%.1fx menu click did not select and close the second row", scale)
		}
	}
}

func TestHoverContextMenuDismissesOutsideSourceAndMenu(t *testing.T) {
	CloseContextMenus()
	t.Cleanup(CloseContextMenus)
	menu := &itemData{Options: []string{"hello"}, Open: true, Size: point{X: 100, Y: 20}, DrawRect: rect{X0: 50, Y0: 30}}
	menu.DismissOnPointerLeave(Rect{X0: 20, Y0: 20, X1: 60, Y1: 40})
	regular := &itemData{Options: []string{"Copy"}, Open: true}
	contextMenus = append(contextMenus, menu, regular)
	dismissContextMenusOutside(point{X: 25, Y: 25})
	if !menu.Open || len(contextMenus) != 2 {
		t.Fatal("hover menu closed over its source")
	}
	bounds, _ := dropdownOpenRect(menu, point{X: 50, Y: 30})
	dismissContextMenusOutside(point{X: bounds.X1 - 1, Y: bounds.Y0 + 1})
	if !menu.Open || len(contextMenus) != 2 {
		t.Fatal("hover menu closed while choosing a suggestion")
	}
	dismissContextMenusOutside(point{X: bounds.X1 + 20, Y: bounds.Y1 + 20})
	if menu.Open || len(contextMenus) != 1 || contextMenus[0] != regular || !regular.Open {
		t.Fatal("leaving the hover menu did not dismiss only that menu")
	}
}

func TestContextMenuSelectionCanOpenFollowUpMenu(t *testing.T) {
	CloseContextMenus()
	t.Cleanup(CloseContextMenus)

	first := &itemData{Options: []string{"Next"}, Open: true, Size: point{X: 100, Y: 20}}
	first.DrawRect.X0 = 20
	first.DrawRect.Y0 = 20
	first.OnSelect = func(int) {
		contextMenus = append(contextMenus, &itemData{Options: []string{"Done"}, Open: true, Size: point{X: 100, Y: 20}})
	}
	contextMenus = append(contextMenus, first)
	r, _ := dropdownOpenRect(first, point{X: first.DrawRect.X0, Y: first.DrawRect.Y0})
	handleContextMenus(point{X: r.X0 + 1, Y: r.Y0 + 1}, true)

	if len(contextMenus) != 1 || contextMenus[0] == first || !contextMenus[0].Open {
		t.Fatalf("follow-up context menu was not kept open: %#v", contextMenus)
	}
}
