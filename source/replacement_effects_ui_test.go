package main

import (
	"image"
	"testing"

	"gothoom/eui"
)

func TestReplacementEffectsPreviewWindowUsesScrollableCards(t *testing.T) {
	initFont()
	originalWindow := replacementEffectsPreviewWin
	originalRoot := replacementEffectsPreviewRoot
	originalList := replacementEffectsPreviewList
	originalCards := replacementEffectsPreviewCards
	originalColumns := replacementEffectsPreviewColumns
	originalZoomUI := replacementEffectsPreviewZoomUI
	originalZoom := replacementEffectsPreviewZoom
	originalPreview := replacementEffectsPreview
	originalScale := eui.UIScale()
	originalWidth, originalHeight := eui.ScreenSize()
	replacementEffectsPreviewWin = nil
	replacementEffectsPreviewRoot = nil
	replacementEffectsPreviewList = nil
	replacementEffectsPreviewCards = nil
	replacementEffectsPreviewColumns = 0
	replacementEffectsPreviewZoomUI = nil
	replacementEffectsPreviewZoom = 1
	replacementEffectsPreview = false
	eui.SetUIScale(1)
	eui.SetScreenSize(1920, 1080)
	t.Cleanup(func() {
		if replacementEffectsPreviewWin != nil {
			replacementEffectsPreviewWin.RemoveWindow()
		}
		replacementEffectsPreviewWin = originalWindow
		replacementEffectsPreviewRoot = originalRoot
		replacementEffectsPreviewList = originalList
		replacementEffectsPreviewCards = originalCards
		replacementEffectsPreviewColumns = originalColumns
		replacementEffectsPreviewZoomUI = originalZoomUI
		replacementEffectsPreviewZoom = originalZoom
		replacementEffectsPreview = originalPreview
		eui.SetUIScale(originalScale)
		eui.SetScreenSize(originalWidth, originalHeight)
	})

	makeReplacementEffectsPreviewWindow()
	if got := replacementEffectsPreviewWin.GetRawSize(); got != (eui.Point{X: replacementEffectsPreviewWindowWidth, Y: replacementEffectsPreviewWindowHeight}) {
		t.Fatalf("effects preview size = %v, want %vx%v", got, replacementEffectsPreviewWindowWidth, replacementEffectsPreviewWindowHeight)
	}
	if replacementEffectsPreviewWin.AutoSize || !replacementEffectsPreviewWin.Resizable || !replacementEffectsPreviewWin.NoScroll || !replacementEffectsPreviewWin.Closable {
		t.Fatalf("effects preview sizing = auto %v, resizable %v, no-scroll %v, closable %v", replacementEffectsPreviewWin.AutoSize, replacementEffectsPreviewWin.Resizable, replacementEffectsPreviewWin.NoScroll, replacementEffectsPreviewWin.Closable)
	}
	if replacementEffectsPreviewList == nil || !replacementEffectsPreviewList.Fixed || !replacementEffectsPreviewList.Scrollable {
		t.Fatal("effects preview is missing its fixed scrolling gallery")
	}
	if replacementEffectsPreviewZoomUI == nil || replacementEffectsPreviewZoomUI.ItemType != eui.ITEM_SLIDER || replacementEffectsPreviewZoomUI.MinValue != 25 || replacementEffectsPreviewZoomUI.MaxValue != 200 || replacementEffectsPreviewZoomUI.Value != 100 {
		t.Fatal("effects preview is missing its 100% zoom slider")
	}
	replacementEffectsPreviewZoomUI.Handler.Emit(eui.UIEvent{Item: replacementEffectsPreviewZoomUI, Type: eui.EventSliderChanged, Value: 150})
	if replacementEffectsPreviewZoom != 1.5 {
		t.Fatalf("effects preview zoom = %.2f, want 1.5", replacementEffectsPreviewZoom)
	}
	if replacementEffectsPreviewColumns != 1 {
		t.Fatalf("150%% effects preview columns = %d, want 1", replacementEffectsPreviewColumns)
	}
	if got := replacementEffectsPreviewCards[0].image.Bounds().Size(); got != image.Pt(438, 276) {
		t.Fatalf("150%% effects card image = %v, want 438x276", got)
	}
	replacementEffectsPreviewZoomUI.Handler.Emit(eui.UIEvent{Item: replacementEffectsPreviewZoomUI, Type: eui.EventSliderChanged, Value: 25})
	if replacementEffectsPreviewColumns <= 2 {
		t.Fatalf("25%% effects preview columns = %d, want more than 2", replacementEffectsPreviewColumns)
	}
	replacementEffectsPreviewZoomUI.Handler.Emit(eui.UIEvent{Item: replacementEffectsPreviewZoomUI, Type: eui.EventSliderChanged, Value: 200})
	if replacementEffectsPreviewColumns != 1 {
		t.Fatalf("maximum effects preview columns = %d, want 1", replacementEffectsPreviewColumns)
	}
	wantMaxWidth := roundToInt(float64(replacementEffectsPreviewList.Size.X - eui.ScrollbarWidth()/eui.UIScale()))
	if got := replacementEffectsPreviewCards[0].image.Bounds().Dx(); got != wantMaxWidth {
		t.Fatalf("maximum effects card width = %d, want gallery width %d", got, wantMaxWidth)
	}
	replacementEffectsPreviewZoomUI.Handler.Emit(eui.UIEvent{Item: replacementEffectsPreviewZoomUI, Type: eui.EventSliderChanged, Value: 100})
	if replacementEffectsPreviewColumns != 2 {
		t.Fatalf("effects preview columns = %d, want 2", replacementEffectsPreviewColumns)
	}
	if got, want := len(replacementEffectsPreviewCards), len(replacementEffectPreviewGalleryGroups()); got != want {
		t.Fatalf("effects preview cards = %d, want %d", got, want)
	}
	for _, card := range replacementEffectsPreviewCards {
		if card.checkbox == nil || card.checkbox.ItemType != eui.ITEM_CHECKBOX || card.image == nil {
			t.Fatalf("invalid effects preview card for %q", card.group.label)
		}
	}
	if !replacementEffectsPreviewWin.SetSize(eui.Point{X: 380, Y: replacementEffectsPreviewWindowHeight}) {
		t.Fatal("effects preview refused a valid narrow resize")
	}
	if replacementEffectsPreviewColumns != 1 {
		t.Fatalf("narrow effects preview columns = %d, want 1", replacementEffectsPreviewColumns)
	}
	if !replacementEffectsPreviewWin.SetSize(eui.Point{X: 980, Y: replacementEffectsPreviewWindowHeight}) {
		t.Fatal("effects preview refused a valid wide resize")
	}
	if replacementEffectsPreviewColumns != 3 {
		t.Fatalf("wide effects preview columns = %d, want 3", replacementEffectsPreviewColumns)
	}
}
