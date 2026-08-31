package main

import (
	"embed"
	"image/png"
	"path/filepath"
	"strings"
	"sync"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed data/icons/material/*.png
var materialIconFiles embed.FS

type materialIconBinding struct {
	name     string
	fallback string
	iconOnly bool
}

var (
	materialIconOnce     sync.Once
	materialIcons        map[string]*ebiten.Image
	materialIconMu       sync.Mutex
	materialIconBindings = make(map[*eui.ItemData]materialIconBinding)
)

// loadMaterialIcons is called from Draw so Ebitengine's graphics context is
// available before the decoded PNGs are uploaded.
func loadMaterialIcons() {
	materialIconMu.Lock()
	defer materialIconMu.Unlock()
	materialIconOnce.Do(func() {
		materialIcons = make(map[string]*ebiten.Image)
		entries, err := materialIconFiles.ReadDir("data/icons/material")
		if err != nil {
			logError("read material icons: %v", err)
			return
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".png") {
				continue
			}
			file, err := materialIconFiles.Open("data/icons/material/" + entry.Name())
			if err != nil {
				logError("open material icon %q: %v", entry.Name(), err)
				continue
			}
			decoded, err := png.Decode(file)
			_ = file.Close()
			if err != nil {
				logError("decode material icon %q: %v", entry.Name(), err)
				continue
			}
			name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			materialIcons[name] = newManagedImageFromImage(decoded)
		}
	})

	for button, binding := range materialIconBindings {
		applyMaterialIconBinding(button, binding)
	}
	clear(materialIconBindings)
}

func applyMaterialIconBinding(button *eui.ItemData, binding materialIconBinding) {
	button.Image = materialIcons[binding.name]
	button.SmoothImage = true
	if binding.iconOnly {
		if button.Image == nil {
			button.Text = binding.fallback
		} else {
			button.Text = ""
		}
	}
	button.Dirty = true
	if button.ParentWindow != nil {
		button.ParentWindow.Refresh()
	}
}

func setMaterialButtonIcon(button *eui.ItemData, name string) {
	setMaterialIconBinding(button, materialIconBinding{name: name})
}

func setMaterialIconOnly(button *eui.ItemData, name, fallback string) {
	setMaterialIconBinding(button, materialIconBinding{name: name, fallback: fallback, iconOnly: true})
}

func setMaterialIconBinding(button *eui.ItemData, binding materialIconBinding) {
	materialIconMu.Lock()
	defer materialIconMu.Unlock()
	if materialIcons != nil {
		applyMaterialIconBinding(button, binding)
		return
	}
	materialIconBindings[button] = binding
	if binding.iconOnly {
		button.Text = binding.fallback
	}
}
