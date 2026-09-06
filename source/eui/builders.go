package eui

import (
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

// Init initializes embedded regular and bold fonts unless the application has
// already supplied its own. Built-in themes and styles are loaded automatically.
// Call once before creating UI. No data directory or external assets are needed.
func Init() error {
	if err := EnsureFontSource(goregular.TTF); err != nil {
		return err
	}
	return EnsureBoldFontSource(gobold.TTF)
}

// NewColumn arranges children vertically. Set Size, Fixed, or Scrollable on the
// returned item when the container needs a fixed viewport instead of auto sizing.
func NewColumn(children ...*ItemData) *ItemData {
	item := &ItemData{ItemType: ITEM_FLOW, FlowType: FLOW_VERTICAL}
	for _, child := range children {
		if child != nil {
			item.AddItem(child)
		}
	}
	return item
}

// NewRow arranges children horizontally.
func NewRow(children ...*ItemData) *ItemData {
	item := &ItemData{ItemType: ITEM_FLOW, FlowType: FLOW_HORIZONTAL}
	for _, child := range children {
		if child != nil {
			item.AddItem(child)
		}
	}
	return item
}

// NewLabel creates a compact, automatically measured label using theme colors.
func NewLabel(label string) *ItemData {
	item, _ := NewText()
	item.Text = label
	item.FontSize = 12
	item.Face = nil
	item.Size = Point{}
	return item
}

// NewActionButton creates a themed button with an optional click callback.
// Use NewButton when access to the full event handler is needed.
func NewActionButton(label string, action func()) *ItemData {
	item, events := NewButton()
	item.Text = label
	item.Size = Point{X: max(128, NewLabel(label).GetSize().X/UIScale()+24), Y: 28}
	events.Handle = func(ev UIEvent) {
		if ev.Type == EventClick && action != nil {
			action()
		}
	}
	return item
}
