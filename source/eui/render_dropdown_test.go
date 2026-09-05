package eui

import (
	"fmt"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font/gofont/goregular"
)

// collectDropdownsForTest mirrors the dropdown collection logic in Draw without rendering.
func collectDropdownsForTest() []openDropdown {
	dropdowns := dropdownReuse[:0]
	if cap(dropdowns) < len(windows) {
		dropdowns = make([]openDropdown, 0, len(windows))
	}
	for _, win := range windows {
		if !win.Open {
			continue
		}
		win.collectDropdowns(&dropdowns)
	}
	dropdownReuse = dropdowns
	return dropdowns
}

func TestDropdownSliceAdjustsWithWindowChanges(t *testing.T) {
	oldWindows := windows
	oldReuse := dropdownReuse
	defer func() {
		windows = oldWindows
		dropdownReuse = oldReuse
	}()

	i1 := &itemData{ItemType: ITEM_DROPDOWN, Open: true, DrawRect: rect{}}
	w1 := &windowData{Open: true, Contents: []*itemData{i1}}
	i2 := &itemData{ItemType: ITEM_DROPDOWN, Open: true, DrawRect: rect{}}
	w2 := &windowData{Open: true, Contents: []*itemData{i2}}

	windows = []*windowData{w1}
	if dds := collectDropdownsForTest(); len(dds) != 1 {
		t.Fatalf("expected 1 dropdown, got %d", len(dds))
	}

	windows = []*windowData{w1, w2}
	if dds := collectDropdownsForTest(); len(dds) != 2 {
		t.Fatalf("expected 2 dropdowns, got %d", len(dds))
	}

	windows = []*windowData{w1}
	if dds := collectDropdownsForTest(); len(dds) != 1 {
		t.Fatalf("expected 1 dropdown after removal, got %d", len(dds))
	}

	windows = nil
	if dds := collectDropdownsForTest(); len(dds) != 0 {
		t.Fatalf("expected 0 dropdowns after clearing windows, got %d", len(dds))
	}
}

func TestWindowDrawCollectsDropdownOnce(t *testing.T) {
	if err := EnsureFontSource(goregular.TTF); err != nil {
		t.Fatal(err)
	}
	for _, noCache := range []bool{false, true} {
		for _, opacity := range []float32{1, 0.5} {
			t.Run(fmt.Sprintf("noCache=%v/opacity=%v", noCache, opacity), func(t *testing.T) {
				win := NewWindow()
				win.NoCache = noCache
				win.Opacity = opacity
				win.Size = point{X: 300, Y: 200}
				win.Position = point{X: 20, Y: 30}
				item := &itemData{ItemType: ITEM_DROPDOWN, Open: true,
					Size: point{X: 100, Y: 24}, FontSize: 12, Label: "Choose"}
				win.AddItem(item)
				screen := ebiten.NewImage(400, 300)
				t.Cleanup(func() { screen.Deallocate(); win.deallocate() })
				var firstOffset point
				for frame := range 2 {
					var dropdowns []openDropdown
					win.Draw(screen, &dropdowns)
					if len(dropdowns) != 1 || dropdowns[0].item != item {
						t.Fatalf("frame %d: expected one dropdown, got %v", frame, dropdowns)
					}
					if frame == 0 {
						firstOffset = dropdowns[0].offset
					} else if dropdowns[0].offset != firstOffset {
						t.Fatalf("dropdown moved between frames: %v -> %v", firstOffset, dropdowns[0].offset)
					}
				}
			})
		}
	}
}
