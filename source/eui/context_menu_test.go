package eui

import "testing"

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
