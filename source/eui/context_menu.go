package eui

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// contextMenus holds active, screen-space context menus rendered as overlays.
var contextMenus []*itemData

// ShowContextMenu opens a simple context menu at screen pixel position (x, y),
// reusing dropdown option rendering. onSelect is invoked with the selected index
// when the user clicks an item. Returns the underlying item handle for advanced
// customization if desired.
func ShowContextMenu(options []string, x, y float32, onSelect func(int)) *ItemData {
	return showContextMenu(options, nil, x, y, onSelect)
}

// ShowContextMenuWithIcons opens a context menu with an optional leading icon
// for each row. Icons are scaled down with linear filtering and all labels use
// the same inset, including rows whose matching icon is nil.
func ShowContextMenuWithIcons(options []string, icons []*ebiten.Image, x, y float32, onSelect func(int)) *ItemData {
	return showContextMenu(options, icons, x, y, onSelect)
}

func showContextMenu(options []string, icons []*ebiten.Image, x, y float32, onSelect func(int)) *ItemData {
	if len(options) == 0 {
		return nil
	}

	// Start from dropdown defaults for consistent look & feel.
	menu := new(itemData)
	*menu = *defaultDropdown
	menu.Options = append([]string(nil), options...)
	menu.OptionImages = append([]*ebiten.Image(nil), icons...)
	menu.SmoothImage = len(icons) > 0
	menu.Open = true
	menu.HoverIndex = -1
	menu.OnSelect = onSelect
	menu.ParentWindow = nil
	// Use same option height as dropdown (menu.Size.Y). Width is computed below.

	// Compute a suitable width based on the longest label and theme paddings.
	textSize := (menu.FontSize * uiScale) + 2
	face := itemFace(menu, textSize)
	maxW := float32(0)
	for _, s := range menu.Options {
		if w, _ := text.Measure(s, face, 0); float32(w) > maxW {
			maxW = float32(w)
		}
	}
	leftPad := menu.BorderPad + menu.Padding + currentStyle.TextPadding*uiScale
	// Mirror left padding on the right and add a small safety margin.
	desiredW := maxW + 2*leftPad + 4*uiScale
	if itemHasOptionImages(menu) {
		desiredW += dropdownOptionImageSlot(menu)
	}
	if desiredW < (defaultDropdown.Size.X * uiScale) {
		desiredW = defaultDropdown.Size.X * uiScale
	}
	// Store unscaled Size so GetSize() yields desired pixel width/height.
	menu.Size.X = desiredW / uiScale

	// Seed DrawRect X0/Y0 with the screen-space origin for overlay math.
	menu.DrawRect.X0 = x
	menu.DrawRect.Y0 = y

	contextMenus = append(contextMenus, menu)
	return (*ItemData)(menu)
}

func itemHasOptionImages(item *itemData) bool {
	for _, icon := range item.OptionImages {
		if icon != nil {
			return true
		}
	}
	return false
}

func dropdownOptionImageSize(item *itemData) float32 {
	size := item.GetSize().Y - 8*uiScale
	if size < 0 {
		return 0
	}
	return size
}

func dropdownOptionImageSlot(item *itemData) float32 {
	return dropdownOptionImageSize(item) + 4*uiScale
}

// CloseContextMenus closes all open context menus.
func CloseContextMenus() { contextMenus = contextMenus[:0] }

// ContextMenusOpen reports if any context menus are active.
func ContextMenusOpen() bool { return len(contextMenus) > 0 }
