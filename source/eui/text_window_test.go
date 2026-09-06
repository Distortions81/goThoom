package eui

import (
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"testing"
)

func TestTextWindowWrapCacheInvalidatesForWidthAndFont(t *testing.T) {
	face := &text.GoTextFace{Size: 12}
	config := textWindowWrapConfig{width: 120, faceSize: face.Size}
	var cache TextWindowWrapCache

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
