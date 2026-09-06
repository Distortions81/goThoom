package eui

import (
	"testing"
	"time"
)

func TestClickFeedbackDefersHiddenPagesUntilSelected(t *testing.T) {
	now := time.Now()
	win := NewWindow()
	visible := &itemData{ItemType: ITEM_BUTTON, ParentWindow: win, Clicked: now.Add(-2 * clickFlash)}
	hidden := &itemData{ItemType: ITEM_BUTTON, ParentWindow: win, Clicked: now.Add(-2 * clickFlash)}
	flow := &itemData{ItemType: ITEM_FLOW, Tabs: []*itemData{
		{Contents: []*itemData{visible}},
		{Contents: []*itemData{hidden}},
	}}
	clearExpiredClicksAt([]*itemData{flow}, now)
	if !visible.Clicked.IsZero() || hidden.Clicked.IsZero() {
		t.Fatal("click cleanup must visit only the visible page")
	}
	win.Dirty = false
	flow.ActiveTab = 1
	clearExpiredClicksAt([]*itemData{flow}, now)
	if !hidden.Clicked.IsZero() || !win.Dirty {
		t.Fatal("selecting a page must clear expired feedback before painting it")
	}
}
