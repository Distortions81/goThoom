package main

import (
	"testing"
	"time"

	"gothoom/eui"
)

func TestToolbarMinimumWidthCoversDockedControls(t *testing.T) {
	const dockedControlsWidth = 84 + 3*(84+4)
	const dockedWindowPadding = 8
	if dockedToolbarMinimumWidth < dockedControlsWidth+dockedWindowPadding {
		t.Fatalf("toolbar minimum width = %v, need at least %d", dockedToolbarMinimumWidth, dockedControlsWidth+dockedWindowPadding)
	}
	const floatingControlsWidth = 84 + 3*(88+4)
	const floatingWindowPadding = 8
	if floatingToolbarMinimumWidth < floatingControlsWidth+floatingWindowPadding {
		t.Fatalf("floating toolbar minimum width = %v, need at least %d", floatingToolbarMinimumWidth, floatingControlsWidth+floatingWindowPadding)
	}
}

func TestFormatToolbarLatencyUsesOneDecimalPlace(t *testing.T) {
	if got := formatToolbarLatency(1234567 * time.Nanosecond); got != "1.2ms" {
		t.Fatalf("formatted latency = %q, want %q", got, "1.2ms")
	}
}

func TestFormatToolbarLossSuppressesZeroDecimal(t *testing.T) {
	if got := formatToolbarLoss(0); got != "0%" {
		t.Fatalf("zero loss = %q, want 0%%", got)
	}
	if got := formatToolbarLoss(0.3); got != "0.3%" {
		t.Fatalf("nonzero loss = %q, want 0.3%%", got)
	}
}

func TestToolbarWindowsControlFollowsTiledMode(t *testing.T) {
	initFont()
	originalSettings := gs
	originalButton := toolbarWindowsBtn
	originalRecord := recordBtn
	originalWindows, originalTileLayout := windowsWin, tileLayoutWin
	t.Cleanup(func() {
		if windowsWin != nil && windowsWin != originalWindows {
			windowsWin.RemoveWindow()
		}
		if tileLayoutWin != nil && tileLayoutWin != originalTileLayout {
			tileLayoutWin.RemoveWindow()
		}
		gs = originalSettings
		toolbarWindowsBtn = originalButton
		recordBtn = originalRecord
		windowsWin = originalWindows
		tileLayoutWin = originalTileLayout
	})

	findWindows := func(root *eui.ItemData) *eui.ItemData {
		var visit func([]*eui.ItemData) *eui.ItemData
		visit = func(items []*eui.ItemData) *eui.ItemData {
			for _, item := range items {
				if item.Text == "Windows" {
					return item
				}
				if found := visit(item.Contents); found != nil {
					return found
				}
			}
			return nil
		}
		return visit(root.Contents)
	}

	gs.TiledWindows = true
	toolbar := buildToolbar(10, 84, 24)
	button := findWindows(toolbar)
	if button == nil || toolbarWindowsBtn != button || button.Disabled {
		t.Fatal("tiled workspace control is missing or disabled")
	}
	windowsWin, tileLayoutWin = nil, nil
	button.Handler.Emit(eui.UIEvent{Item: button, Type: eui.EventClick})
	if tileLayoutWin == nil || !tileLayoutWin.IsOpen() || windowsWin != nil {
		t.Fatal("Windows control did not open the tiled layout settings")
	}
	tileLayoutWin.RemoveWindow()
	tileLayoutWin = nil

	gs.TiledWindows = false
	toolbar = buildToolbar(10, 84, 24)
	button = findWindows(toolbar)
	if button == nil || button.Disabled {
		t.Fatal("floating Windows control is missing or disabled")
	}
	button.Handler.Emit(eui.UIEvent{Item: button, Type: eui.EventClick})
	if windowsWin == nil || !windowsWin.IsOpen() || tileLayoutWin != nil {
		t.Fatal("Windows control did not open the floating window picker")
	}
}
