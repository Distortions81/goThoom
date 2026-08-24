package eui

import "testing"

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
