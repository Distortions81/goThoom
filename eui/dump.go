package eui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
)

// DumpCachedImages writes all cached item images and item source images
// to the debug directory. The game must be running so pixels can be read.
// Any pending renders are generated before writing the files.
func DumpCachedImages() error {
	if err := os.MkdirAll("debug", 0755); err != nil {
		return err
	}
	for i, win := range windows {
		dumpItemImages(win.Contents, fmt.Sprintf("window_%d", i))
	}
	for i, ov := range overlays {
		dumpItemImages([]*itemData{ov}, fmt.Sprintf("overlay_%d", i))
	}
	return nil
}

func dumpItemImages(items []*itemData, prefix string) {
	for idx, it := range items {
		if it == nil {
			continue
		}
		name := fmt.Sprintf("%s_%d", prefix, idx)
		if it.ItemType != ITEM_FLOW {
			it.ensureRender()
			if it.Render != nil {
				fn := filepath.Join("debug", name+".png")
				if f, err := os.Create(fn); err == nil {
					png.Encode(f, it.Render)
					f.Close()
				}
			}
			if it.Image != nil {
				fn := filepath.Join("debug", name+"_src.png")
				if f, err := os.Create(fn); err == nil {
					png.Encode(f, it.Image)
					f.Close()
				}
			}
		}
		if len(it.Contents) > 0 {
			dumpItemImages(it.Contents, name)
		}
		if len(it.Tabs) > 0 {
			dumpItemImages(it.Tabs, name+"_tab")
		}
	}
}
