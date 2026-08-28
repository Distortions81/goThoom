package main

import (
	"testing"

	"gothoom/eui"
)

func TestScriptsWindowUsesSingleListWithInfoButtons(t *testing.T) {
	initFont()
	originalWin := scriptsWin
	originalRoot := scriptsRoot
	originalHeader := scriptsHeader
	originalList := scriptsList
	originalButtons := scriptsButtons
	originalDetails := scriptDetails
	originalDebugList := scriptDebugList
	originalSelected := selectedscript
	t.Cleanup(func() {
		if scriptsWin != nil && scriptsWin != originalWin {
			scriptsWin.RemoveWindow()
		}
		scriptsWin = originalWin
		scriptsRoot = originalRoot
		scriptsHeader = originalHeader
		scriptsList = originalList
		scriptsButtons = originalButtons
		scriptDetails = originalDetails
		scriptDebugList = originalDebugList
		selectedscript = originalSelected
	})

	scriptsWin = nil
	scriptsRoot = nil
	scriptsHeader = nil
	scriptsList = nil
	scriptsButtons = nil
	scriptDetails = nil
	scriptDebugList = nil
	selectedscript = ""
	makescriptsWindow()

	if scriptsWin == nil || len(scriptsWin.Contents) != 1 {
		t.Fatal("scripts window was not created")
	}
	if scriptsWin.AutoSize || !scriptsWin.Resizable || !scriptsWin.NoScroll || scriptsWin.OnResize == nil {
		t.Fatalf("scripts window sizing = auto %v, resizable %v, no-scroll %v", scriptsWin.AutoSize, scriptsWin.Resizable, scriptsWin.NoScroll)
	}
	root := scriptsWin.Contents[0]
	if len(root.Contents) < 3 {
		t.Fatalf("scripts root sections = %d, want header, list, and buttons", len(root.Contents))
	}
	if root.Contents[0].FlowType != eui.FLOW_HORIZONTAL {
		t.Fatalf("scripts header layout = %#v, want horizontal columns", root.Contents[0])
	}
	if root.Contents[1] != scriptsList || !scriptsList.Scrollable || !scriptsList.Fixed {
		t.Fatalf("scripts list = %#v, want a fixed independently scrolling list", scriptsList)
	}
	if root.Contents[2] == scriptsList {
		t.Fatal("scripts action buttons were placed inside the item list")
	}
	if !root.Fixed || root.Scrollable {
		t.Fatalf("scripts root layout = %#v, want a fixed resize-aware root", root)
	}
	wantListHeight := scriptsList.Size.Y
	for range 3 {
		refreshscriptsWindow()
	}
	if scriptsList.Size.Y != wantListHeight {
		t.Fatalf("repeated refresh changed list height: %v -> %v", wantListHeight, scriptsList.Size.Y)
	}
}
