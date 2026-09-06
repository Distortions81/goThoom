package eui

import (
	"math"
	"testing"
)

func TestDropdownDismissalEndsHover(t *testing.T) {
	isolateThemeTest(t)
	dd, _ := NewDropdown()
	dd.Options = []string{"Current", "Preview"}
	dd.Selected = 0
	calls := 0
	dd.OnHover = func(index int) {
		calls++
		if index != 0 || dd.HoverIndex != -1 {
			t.Fatal("dismissal did not restore selected option")
		}
	}
	dd.Open, dd.HoverIndex = true, 1
	closeDropdowns([]*itemData{dd})
	if dd.Open || calls != 1 {
		t.Fatal("outside dismissal failed")
	}
	closeDropdowns([]*itemData{dd})
	if calls != 1 {
		t.Fatal("dismissal callback repeated")
	}
	dd.Open, dd.HoverIndex = true, 1
	win := NewWindow()
	win.Contents = []*itemData{dd}
	win.Close()
	if dd.Open || calls != 2 {
		t.Fatal("window close left preview active")
	}
}

func TestLabeledDropdownRowsMatchHitTargets(t *testing.T) {
	isolateThemeTest(t)
	oldHeight, oldWidth, oldScale := screenHeight, screenWidth, uiScale
	t.Cleanup(func() { screenHeight, screenWidth, uiScale = oldHeight, oldWidth, oldScale })
	screenHeight, screenWidth = 1200, 1200
	for _, scale := range []float32{1, 1.3, 2} {
		uiScale = scale
		dd, _ := NewDropdown()
		dd.Label = "Color Theme"
		dd.Size = point{200, 28}
		dd.Options = []string{"Current", "Preview", "Third"}
		dd.Open = true
		dd.DrawRect = rect{X0: 20, Y0: 40, X1: 20 + dd.GetSize().X, Y1: 40 + dd.GetSize().Y}
		var menus []openDropdown
		collectItemDropdowns([]*itemData{dd}, &menus)
		r, _ := dropdownOpenRect(dd, menus[0].offset)
		if r.Y0 != dd.DrawRect.Y1 || math.Abs(float64((r.Y1-r.Y0)-3*28*scale)) > 0.001 {
			t.Fatalf("%.1fx menu includes label height or has a gap: %v", scale, r)
		}
		point := point{X: r.X0 + 10, Y: r.Y0 + 28*scale*1.5}
		if !clickOpenDropdown([]*itemData{dd}, point, false) || dd.HoverIndex != 1 {
			t.Fatalf("%.1fx hovered row differs from rendered row: %d", scale, dd.HoverIndex)
		}
		clickOpenDropdown([]*itemData{dd}, point, true)
		if dd.Selected != 1 || dd.Open {
			t.Fatal("click did not select hovered row")
		}
	}
}

func TestPreviewDropdownKeepsOpeningMetricsAndHitTargets(t *testing.T) {
	isolateThemeTest(t)
	oldHeight, oldWidth, oldScale := screenHeight, screenWidth, uiScale
	t.Cleanup(func() { screenHeight, screenWidth, uiScale = oldHeight, oldWidth, oldScale })
	screenHeight, screenWidth = 1200, 1200
	for _, scale := range []float32{1, 1.3, 2} {
		uiScale = scale
		if err := LoadTheme("AccentDark"); err != nil {
			t.Fatal(err)
		}
		dd, _ := NewDropdown()
		dd.Label = "Color Theme"
		dd.Size = point{200, 28}
		dd.Options = []string{"Current", "Large preview", "Third"}
		dd.Open, dd.HoverIndex = true, -1
		dd.DrawRect = rect{X0: 20, Y0: 40}
		previews := 0
		dd.OnHover = func(index int) {
			if dd.HoverIndex < 0 {
				return
			}
			previews++
			if err := LoadStyle("Rounded"); err != nil {
				t.Fatal(err)
			}
			// Simulate a custom theme changing type size and reflowing its window.
			dd.FontSize = 30
			dd.Size = point{300, 64}
			dd.BorderPad = 20
			dd.DrawRect = rect{X0: 120, Y0: 140}
		}
		opening := dropdownMenuLayout(dd, point{20, 40})
		pointer := point{X: opening.bounds.X0 + 10, Y: opening.bounds.Y0 + opening.rowHeight*1.1}
		for range 5 {
			if !clickOpenDropdown([]*itemData{dd}, pointer, false) || dd.HoverIndex != 1 {
				t.Fatalf("%.1fx preview moved the option under the pointer", scale)
			}
			if got := dropdownMenuLayout(dd, point{120, 140}); got != opening {
				t.Fatalf("%.1fx open menu changed its text or geometry: %+v -> %+v", scale, opening, got)
			}
		}
		if previews != 1 {
			t.Fatalf("stationary pointer retriggered %d previews", previews)
		}
		clickOpenDropdown([]*itemData{dd}, pointer, true)
		if dd.Selected != 1 || dd.Open || dd.dropdownLayout != nil {
			t.Fatal("click failed or retained opening metrics after selection")
		}
		dd.Open = true
		reopened := dropdownMenuLayout(dd, point{120, 140})
		if reopened.fontSize != 30*scale+2 || math.Abs(float64(reopened.rowHeight-64*scale)) > 0.001 || reopened.bounds == opening.bounds {
			t.Fatalf("%.1fx reopened menu did not adopt the new theme's metrics: %+v", scale, reopened)
		}
		// Dismiss without having hovered a row in the reopened menu.
		dd.HoverIndex = -1
		closeDropdowns([]*itemData{dd})
		if dd.dropdownLayout != nil {
			t.Fatal("dismissed menu retained opening metrics")
		}
	}
}
