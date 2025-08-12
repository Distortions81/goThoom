package eui

import (
	"fmt"
	"image"
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
				if err := saveImage(fn, it.Render); err != nil {
					fmt.Printf("failed to save %s: %v\n", fn, err)
				}
			}
			if it.Image != nil {
				fn := filepath.Join("debug", name+"_src.png")
				if err := saveImage(fn, it.Image); err != nil {
					fmt.Printf("failed to save %s: %v\n", fn, err)
				}
			}
			if it.LabelImage != nil {
				fn := filepath.Join("debug", name+"_label.png")
				if err := saveImage(fn, it.LabelImage); err != nil {
					fmt.Printf("failed to save %s: %v\n", fn, err)
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

func saveImage(fn string, img image.Image) error {
	f, err := os.Create(fn)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return err
	}
	return nil
}
