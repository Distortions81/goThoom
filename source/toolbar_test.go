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
	t.Cleanup(func() {
		gs = originalSettings
		toolbarWindowsBtn = originalButton
		recordBtn = originalRecord
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

	for _, tiled := range []bool{true, false} {
		gs.TiledWindows = tiled
		toolbar := buildToolbar(10, 84, 24)
		button := findWindows(toolbar)
		if button == nil || toolbarWindowsBtn != button {
			t.Fatal("toolbar is missing the Windows control")
		}
		if button.Disabled != tiled {
			t.Fatalf("Windows control disabled = %t with tiled mode %t", button.Disabled, tiled)
		}
	}
}
