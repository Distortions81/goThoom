package eui

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font/gofont/goregular"
)

func TestTabRenderingUsesThemeFontAndSurfaceColor(t *testing.T) {
	previousScale := uiScale
	uiScale = 1
	t.Cleanup(func() { uiScale = previousScale })
	if err := EnsureFontSource(goregular.TTF); err != nil {
		t.Fatalf("font source: %v", err)
	}
	theme := *baseTheme
	theme.Tab.FontSize = 12
	theme.Tab.Color = NewColor(17, 31, 47, 255)
	theme.Tab.SelectedColor = NewColor(61, 73, 89, 255)
	theme.Tab.TextColor = NewColor(255, 255, 255, 255)
	theme.Tab.OutlineColor = theme.Tab.Color

	flow := &itemData{
		ItemType: ITEM_FLOW,
		FlowType: FLOW_VERTICAL,
		Fixed:    true,
		Filled:   true,
		Size:     point{X: 236, Y: 100},
		Theme:    &theme,
		Tabs: []*itemData{
			{Name: "View", ItemType: ITEM_FLOW, FlowType: FLOW_VERTICAL},
			{Name: "Snells", ItemType: ITEM_FLOW, FlowType: FLOW_VERTICAL},
			{Name: "Sprites", ItemType: ITEM_FLOW, FlowType: FLOW_VERTICAL},
		},
	}
	win := NewWindow()
	win.Size = point{X: 236, Y: 100}
	win.AddItem(flow)
	if got, want := flowFillColor(flow, &theme.Tab), theme.Tab.Color; got != want {
		t.Fatalf("tab content background = %v, want theme surface %v", got, want)
	}
	if got, want := tabFontSize(flow, &theme.Tab), theme.Tab.FontSize; got != want {
		t.Fatalf("tab font size = %v, want theme font size %v", got, want)
	}

	screen := ebiten.NewImage(236, 100)
	t.Cleanup(func() { screen.Deallocate() })
	flow.drawFlows(
		win,
		nil,
		point{},
		point{},
		rect{X0: 0, Y0: 0, X1: 236, Y1: 100},
		screen,
		new([]openDropdown),
	)
}

func TestTabContentBoundsIncludeTabStrip(t *testing.T) {
	previousScale := uiScale
	uiScale = 1
	t.Cleanup(func() { uiScale = previousScale })

	content := &itemData{
		ItemType: ITEM_TEXT,
		Size:     point{X: 200, Y: 40},
	}
	flow := &itemData{
		ItemType: ITEM_FLOW,
		FlowType: FLOW_VERTICAL,
		Tabs: []*itemData{
			{Name: "General", Contents: []*itemData{content}},
		},
	}

	got := flow.contentBounds()
	want := content.GetSize().Y + float32(defaultTabHeight)
	if got.Y != want {
		t.Fatalf("tabbed content height = %.0f, want %.0f", got.Y, want)
	}
}

func TestTabDrawRectsUseScreenCoordinates(t *testing.T) {
	previousScale := uiScale
	uiScale = 1
	t.Cleanup(func() { uiScale = previousScale })
	flow := &itemData{
		ItemType: ITEM_FLOW,
		FlowType: FLOW_VERTICAL,
		Fixed:    true,
		Size:     point{X: 200, Y: 120},
		Tabs: []*itemData{
			{ItemType: ITEM_FLOW, FlowType: FLOW_VERTICAL},
			{ItemType: ITEM_FLOW, FlowType: FLOW_VERTICAL},
		},
	}
	win := NewWindow()
	win.Size = point{X: 300, Y: 200}
	win.AddItem(flow)

	screen := ebiten.NewImage(300, 200)
	t.Cleanup(func() { screen.Deallocate() })
	windowPosition := point{X: 90, Y: 70}
	flow.drawFlows(
		win,
		nil,
		point{X: 10, Y: 20},
		windowPosition,
		rect{X0: 0, Y0: 0, X1: 300, Y1: 200},
		screen,
		new([]openDropdown),
	)

	secondTab := flow.Tabs[1]
	clickPosition := point{
		X: (secondTab.DrawRect.X0 + secondTab.DrawRect.X1) / 2,
		Y: (secondTab.DrawRect.Y0 + secondTab.DrawRect.Y1) / 2,
	}
	if clickPosition.X < windowPosition.X || clickPosition.Y < windowPosition.Y {
		t.Fatalf("tab hit rectangle %#v did not include window position %#v", secondTab.DrawRect, windowPosition)
	}
	if !flow.clickFlows(clickPosition, true) {
		t.Fatal("tab click at its rendered screen position was not handled")
	}
	if flow.ActiveTab != 1 {
		t.Fatalf("active tab = %d, want 1", flow.ActiveTab)
	}
}

func TestTabClickRefreshesAndResetsTheActiveFlow(t *testing.T) {
	win := NewWindow()
	win.Open = true
	win.Size = point{X: 300, Y: 200}

	flow := &itemData{
		ItemType: ITEM_FLOW,
		FlowType: FLOW_VERTICAL,
		Fixed:    true,
		Size:     point{X: 200, Y: 120},
		Scroll:   point{X: 0, Y: 40},
		Tabs: []*itemData{
			{Name: "View", ItemType: ITEM_FLOW, FlowType: FLOW_VERTICAL},
			{Name: "Sprites", ItemType: ITEM_FLOW, FlowType: FLOW_VERTICAL},
		},
	}
	win.AddItem(flow)
	flow.Tabs[1].DrawRect = rect{X0: 60, Y0: 0, X1: 120, Y1: 24}
	win.Dirty = false

	if !flow.clickFlows(point{X: 80, Y: 12}, true) {
		t.Fatal("tab click was not handled")
	}
	if flow.ActiveTab != 1 {
		t.Fatalf("active tab = %d, want 1", flow.ActiveTab)
	}
	if flow.Scroll != (point{}) {
		t.Fatalf("tab switch retained scroll offset %#v", flow.Scroll)
	}
	if !win.Dirty {
		t.Fatal("tab switch did not refresh the cached window")
	}
}

func TestStaggeredTabsReserveSpaceAndHandleSecondRowClicks(t *testing.T) {
	previousScale := uiScale
	t.Cleanup(func() { uiScale = previousScale })
	if err := EnsureFontSource(goregular.TTF); err != nil {
		t.Fatal(err)
	}
	for _, scale := range []float32{1, 1.3, 2} {
		uiScale = scale
		content := &itemData{ItemType: ITEM_TEXT, Size: point{X: 400, Y: 40}}
		flow := &itemData{
			ItemType: ITEM_FLOW, FlowType: FLOW_VERTICAL, Fixed: true,
			Size: point{X: 440, Y: 160}, TabColumns: 3, TabRowOffset: 16,
			Tabs: []*itemData{
				{Name: "Display", Contents: []*itemData{content}}, {Name: "Audio"}, {Name: "Text"},
				{Name: "Controls"}, {Name: "Files"}, {Name: "Network"},
			},
		}
		win := NewWindow()
		win.Size = point{X: 500, Y: 250}
		win.AddItem(flow)
		rowHeight := tabRowHeight(flow, flow.themeStyle())
		stripHeight := 2*rowHeight + 4*scale
		if got := flow.contentBounds().Y; got != content.GetSize().Y+stripHeight {
			t.Fatalf("content height at %.1fx = %v, want %v", scale, got, content.GetSize().Y+stripHeight)
		}
		if got := flow.contentBounds().X; got < 408*scale {
			t.Fatalf("content width at %.1fx = %v, missing staggered row space", scale, got)
		}
		screen := ebiten.NewImage(1200, 600)
		flow.drawFlows(win, nil, point{X: 10, Y: 20}, point{X: 30, Y: 40},
			rect{X0: 0, Y0: 0, X1: 1200, Y1: 600}, screen, new([]openDropdown))
		screen.Deallocate()
		first, secondRow := flow.Tabs[0].DrawRect, flow.Tabs[3].DrawRect
		if secondRow.X0 != first.X0+16*scale || secondRow.Y0 != first.Y1+4*scale {
			t.Fatalf("second row does not wrap below first at %.1fx: %v, %v", scale, first, secondRow)
		}
		click := point{X: (secondRow.X0 + secondRow.X1) / 2, Y: (secondRow.Y0 + secondRow.Y1) / 2}
		if !flow.clickFlows(click, true) || flow.ActiveTab != 3 {
			t.Fatalf("second row click at %.1fx selected tab %d, want 3", scale, flow.ActiveTab)
		}
	}
}
