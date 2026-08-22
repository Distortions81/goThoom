package eui

import "testing"

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
