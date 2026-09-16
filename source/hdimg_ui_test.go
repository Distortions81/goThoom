package main

import (
	"testing"

	"gothoom/eui"
)

func TestHDPicturePreviewWindowUsesScrollableCards(t *testing.T) {
	initFont()
	originalWindow := hdPicturePreviewWin
	originalRoot := hdPicturePreviewRoot
	originalList := hdPicturePreviewList
	originalStatus := hdPicturePreviewStatus
	originalCards := hdPicturePreviewCards
	originalColumns := hdPicturePreviewColumns
	originalZoomUI := hdPicturePreviewZoomUI
	originalPackUI := hdPicturePreviewPackUI
	originalZoom := hdPicturePreviewZoom
	originalIDs := hdPicturePreviewIDs
	originalSources := hdPictureSources
	originalPacks := hdPicturePacks
	originalPackKey := hdPicturePreviewPackKey
	originalDirty := hdPicturePreviewDirty
	originalScale := eui.UIScale()
	originalWidth, originalHeight := eui.ScreenSize()
	hdPicturePreviewWin = nil
	hdPicturePreviewRoot = nil
	hdPicturePreviewList = nil
	hdPicturePreviewStatus = nil
	hdPicturePreviewCards = nil
	hdPicturePreviewColumns = 0
	hdPicturePreviewZoomUI = nil
	hdPicturePreviewPackUI = nil
	hdPicturePreviewZoom = 1
	hdPicturePreviewIDs = nil
	hdPictureSources = nil
	hdPicturePacks = []hdPicturePack{
		{key: "loose", label: "Loose files", loose: true, sources: map[uint16]hdPictureSource{}},
		{key: "zip:test.zip", label: "test.zip (user folder)", sources: map[uint16]hdPictureSource{}},
	}
	hdPicturePreviewPackKey = ""
	hdPicturePreviewDirty = false
	eui.SetUIScale(1)
	eui.SetScreenSize(1920, 1080)
	t.Cleanup(func() {
		if hdPicturePreviewWin != nil {
			hdPicturePreviewWin.RemoveWindow()
		}
		hdPicturePreviewWin = originalWindow
		hdPicturePreviewRoot = originalRoot
		hdPicturePreviewList = originalList
		hdPicturePreviewStatus = originalStatus
		hdPicturePreviewCards = originalCards
		hdPicturePreviewColumns = originalColumns
		hdPicturePreviewZoomUI = originalZoomUI
		hdPicturePreviewPackUI = originalPackUI
		hdPicturePreviewZoom = originalZoom
		hdPicturePreviewIDs = originalIDs
		hdPictureSources = originalSources
		hdPicturePacks = originalPacks
		hdPicturePreviewPackKey = originalPackKey
		hdPicturePreviewDirty = originalDirty
		eui.SetUIScale(originalScale)
		eui.SetScreenSize(originalWidth, originalHeight)
	})

	makeHDPicturePreviewWindow()
	if got := hdPicturePreviewWin.GetRawSize(); got != (eui.Point{X: hdPicturePreviewWindowWidth, Y: hdPicturePreviewWindowHeight}) {
		t.Fatalf("HD preview size = %v, want %vx%v", got, hdPicturePreviewWindowWidth, hdPicturePreviewWindowHeight)
	}
	if hdPicturePreviewWin.AutoSize || !hdPicturePreviewWin.Resizable || !hdPicturePreviewWin.NoScroll || !hdPicturePreviewWin.Closable {
		t.Fatalf("HD preview sizing = auto %v, resizable %v, no-scroll %v, closable %v", hdPicturePreviewWin.AutoSize, hdPicturePreviewWin.Resizable, hdPicturePreviewWin.NoScroll, hdPicturePreviewWin.Closable)
	}
	if hdPicturePreviewList == nil || !hdPicturePreviewList.Fixed || !hdPicturePreviewList.Scrollable {
		t.Fatal("HD preview is missing its fixed scrolling gallery")
	}
	if hdPicturePreviewPackUI == nil || hdPicturePreviewPackUI.ItemType != eui.ITEM_DROPDOWN {
		t.Fatal("HD preview is missing its sprite-pack dropdown")
	}
	if len(hdPicturePreviewPackUI.Options) != 2 || hdPicturePreviewPackUI.Options[0] != "Loose files" || hdPicturePreviewPackUI.Selected != 0 {
		t.Fatalf("HD preview pack options = %v selected %d, want Loose files selected", hdPicturePreviewPackUI.Options, hdPicturePreviewPackUI.Selected)
	}
	hdPicturePreviewPackUI.Handler.Emit(eui.UIEvent{Item: hdPicturePreviewPackUI, Type: eui.EventDropdownSelected, Index: 1})
	if hdPicturePreviewPackKey != "zip:test.zip" || hdPicturePreviewPackUI.Selected != 1 {
		t.Fatalf("HD preview selected pack = %q at %d, want ZIP pack", hdPicturePreviewPackKey, hdPicturePreviewPackUI.Selected)
	}
	if hdPicturePreviewZoomUI == nil || hdPicturePreviewZoomUI.ItemType != eui.ITEM_SLIDER || hdPicturePreviewZoomUI.MinValue != 25 || hdPicturePreviewZoomUI.MaxValue != 200 || hdPicturePreviewZoomUI.Value != 100 {
		t.Fatal("HD preview is missing its 100% zoom slider")
	}
	hdPicturePreviewZoomUI.Handler.Emit(eui.UIEvent{Item: hdPicturePreviewZoomUI, Type: eui.EventSliderChanged, Value: 150})
	if hdPicturePreviewZoom != 1.5 {
		t.Fatalf("HD preview zoom = %.2f, want 1.5", hdPicturePreviewZoom)
	}
	if hdPicturePreviewColumns != 1 {
		t.Fatalf("150%% HD preview columns = %d, want 1", hdPicturePreviewColumns)
	}
	hdPicturePreviewZoomUI.Handler.Emit(eui.UIEvent{Item: hdPicturePreviewZoomUI, Type: eui.EventSliderChanged, Value: 25})
	if hdPicturePreviewColumns <= 2 {
		t.Fatalf("25%% HD preview columns = %d, want more than 2", hdPicturePreviewColumns)
	}
	hdPicturePreviewZoomUI.Handler.Emit(eui.UIEvent{Item: hdPicturePreviewZoomUI, Type: eui.EventSliderChanged, Value: 200})
	if hdPicturePreviewColumns != 1 {
		t.Fatalf("maximum HD preview columns = %d, want 1", hdPicturePreviewColumns)
	}
	cardWidth, _ := hdPicturePreviewCardDimensions(hdPicturePreviewList.Size.X)
	wantMaxWidth := hdPicturePreviewList.Size.X - eui.ScrollbarWidth()/eui.UIScale()
	if cardWidth != wantMaxWidth {
		t.Fatalf("maximum HD card width = %.1f, want gallery width %.1f", cardWidth, wantMaxWidth)
	}
	hdPicturePreviewZoomUI.Handler.Emit(eui.UIEvent{Item: hdPicturePreviewZoomUI, Type: eui.EventSliderChanged, Value: 100})
	if hdPicturePreviewColumns != 2 {
		t.Fatalf("HD preview columns = %d, want 2", hdPicturePreviewColumns)
	}
	if !hdPicturePreviewWin.SetSize(eui.Point{X: 380, Y: hdPicturePreviewWindowHeight}) || hdPicturePreviewColumns != 1 {
		t.Fatalf("narrow HD preview columns = %d, want 1", hdPicturePreviewColumns)
	}
	if !hdPicturePreviewWin.SetSize(eui.Point{X: 980, Y: hdPicturePreviewWindowHeight}) || hdPicturePreviewColumns != 3 {
		t.Fatalf("wide HD preview columns = %d, want 3", hdPicturePreviewColumns)
	}
}

func TestHDPicturePreviewColumnsFollowWindowWidth(t *testing.T) {
	if got := hdPicturePreviewColumnCount(300); got != 1 {
		t.Fatalf("narrow HD preview columns = %d, want 1", got)
	}
	if got := hdPicturePreviewColumnCount(600); got != 2 {
		t.Fatalf("default HD preview columns = %d, want 2", got)
	}
	if got := hdPicturePreviewColumnCount(900); got != 3 {
		t.Fatalf("wide HD preview columns = %d, want 3", got)
	}
}

func TestHDPictureSelectionIsSortedAndIdempotent(t *testing.T) {
	original := append([]uint16(nil), gs.DisabledHDPictures...)
	gs.DisabledHDPictures = nil
	t.Cleanup(func() { gs.DisabledHDPictures = original })

	setHDPictureEnabled(1068, false)
	setHDPictureEnabled(23, false)
	setHDPictureEnabled(1068, false)
	if got := gs.DisabledHDPictures; len(got) != 2 || got[0] != 23 || got[1] != 1068 {
		t.Fatalf("disabled HD pictures = %v, want [23 1068]", got)
	}
	setHDPictureEnabled(23, true)
	if got := gs.DisabledHDPictures; len(got) != 1 || got[0] != 1068 {
		t.Fatalf("disabled HD pictures after re-enable = %v, want [1068]", got)
	}
}
