package main

import (
	"testing"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/goregular"
)

func TestTextWindowWrapCacheInvalidatesForWidthAndFont(t *testing.T) {
	face := &text.GoTextFace{Size: 12}
	config := textWindowWrapConfig{width: 120, faceSize: face.Size}
	var cache textWindowWrapCache

	cache.begin(config)
	cache.wrap("A message that wraps", face, config.width)
	cached := cache.entries["A message that wraps"]
	cached.text = "cached sentinel"
	cache.entries["A message that wraps"] = cached

	cache.begin(config)
	if wrapped, _ := cache.wrap("A message that wraps", face, config.width); wrapped != "cached sentinel" {
		t.Fatal("unchanged width and font did not reuse cached wrapping")
	}

	widthConfig := config
	widthConfig.width = 60
	cache.begin(widthConfig)
	if wrapped, _ := cache.wrap("A message that wraps", face, widthConfig.width); wrapped == "cached sentinel" {
		t.Fatal("width change did not invalidate cached wrapping")
	}

	cached = cache.entries["A message that wraps"]
	cached.text = "font sentinel"
	cache.entries["A message that wraps"] = cached
	fontConfig := widthConfig
	fontConfig.faceSize = 16
	cache.begin(fontConfig)
	if wrapped, _ := cache.wrap("A message that wraps", &text.GoTextFace{Size: 16}, fontConfig.width); wrapped == "font sentinel" {
		t.Fatal("font size change did not invalidate cached wrapping")
	}
}

func TestTextWindowInputRemainsInsideClientAreaAtUIScale(t *testing.T) {
	originalScale := eui.UIScale()
	originalWidth, originalHeight := eui.ScreenSize()
	t.Cleanup(func() {
		eui.SetUIScale(originalScale)
		eui.SetScreenSize(originalWidth, originalHeight)
	})

	if err := eui.EnsureFontSource(goregular.TTF); err != nil {
		t.Fatalf("load test font: %v", err)
	}
	eui.SetScreenSize(1600, 1000)
	eui.SetUIScale(2)
	win, list, input := makeTextWindow("Console Test", eui.HZoneLeft, eui.VZoneTop, true)
	t.Cleanup(win.RemoveWindow)
	if !win.SetSize(eui.Point{X: 400, Y: 300}) {
		t.Fatal("could not size text window")
	}

	var wrapCache textWindowWrapCache
	updateTextWindow(win, list, input, []string{"A message"}, 12, "[Press Enter To Type]", nil, true, &wrapCache)

	pad := (win.Padding + win.BorderPad) * eui.UIScale()
	clientHeight := win.GetSize().Y - win.GetTitleSize() - 2*pad
	occupiedHeight := list.GetSize().Y + input.GetSize().Y
	if occupiedHeight > clientHeight+0.01 {
		t.Fatalf("list and input height = %.2f, outside %.2f pixel client area", occupiedHeight, clientHeight)
	}
	if input.GetSize().Y <= 0 {
		t.Fatal("input bar has no visible height")
	}
	if input.GetSize().X > win.GetSize().X-2*pad+0.01 {
		t.Fatalf("input width = %.2f, outside window client width", input.GetSize().X)
	}
}

func TestSizeTextWindowListUsesLogicalUnitsAtUIScale(t *testing.T) {
	originalScale := eui.UIScale()
	t.Cleanup(func() { eui.SetUIScale(originalScale) })
	eui.SetUIScale(2)

	parent := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	toolbar := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	toolbar.Size = eui.Point{X: 150, Y: 30}
	list := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true, Scrollable: true}
	parent.AddItem(toolbar)
	parent.AddItem(list)

	sizeTextWindowList(list, 300, 200)

	if got, want := list.GetSize().Y, float32(140); got != want {
		t.Fatalf("physical list height = %v, want %v after docked row", got, want)
	}
	if got, want := list.GetSize().X, float32(300); got != want {
		t.Fatalf("physical list width = %v, want %v", got, want)
	}
}
