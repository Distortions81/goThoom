package eui

import "testing"

func TestSetItemsReparentsChildren(t *testing.T) {
	win := NewWindow()
	parent := &itemData{ItemType: ITEM_FLOW, FlowType: FLOW_VERTICAL}
	oldChild, _ := NewText()
	newChild, _ := NewText()
	parent.AddItem(oldChild)
	win.AddItem(parent)

	parent.SetItems([]*ItemData{newChild})
	if len(parent.Contents) != 1 || parent.Contents[0] != newChild {
		t.Fatalf("replacement contents = %v, want new child", parent.Contents)
	}
	if oldChild.Parent != nil || oldChild.ParentWindow != nil {
		t.Fatal("removed child retained its parent")
	}
	if newChild.Parent != parent || newChild.ParentWindow != win {
		t.Fatal("new child was not attached to the parent window")
	}
}
