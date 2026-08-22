package eui

import (
	"bytes"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// Windows returns the list of active windows.
func Windows() []*WindowData { return windows }

// WindowTiling reports whether window tiling is enabled.
func WindowTiling() bool { return windowTiling }

// SetWindowTiling enables or disables window tiling.
func SetWindowTiling(enabled bool) { windowTiling = enabled }

// WindowSnapping reports whether window snapping is enabled.
func WindowSnapping() bool { return windowSnapping }

// SetWindowSnapping enables or disables window snapping.
func SetWindowSnapping(enabled bool) { windowSnapping = enabled }

// WindowPinning reports whether title-bar pin buttons are enabled.
func WindowPinning() bool { return windowPinning }

// SetWindowPinning enables title-bar buttons for pinning windows to layout zones.
func SetWindowPinning(enabled bool) {
	if windowPinning == enabled {
		return
	}
	windowPinning = enabled
	for _, win := range windows {
		win.markDirty()
	}
}

// ShowPinLocations reports whether pin-to zone indicators are shown while dragging windows.
func ShowPinLocations() bool { return showPinLocations }

// SetShowPinLocations enables or disables pin-to zone indicators while dragging windows.
func SetShowPinLocations(enabled bool) { showPinLocations = enabled }

// MiddleClickMove reports whether middle-click window dragging is enabled.
func MiddleClickMove() bool { return middleClickMove }

// SetMiddleClickMove enables or disables dragging windows with the middle mouse button.
func SetMiddleClickMove(enabled bool) { middleClickMove = enabled }

// SetScreenSize sets the current screen size used for layout calculations.
func SetScreenSize(w, h int) {
	screenWidth = w
	screenHeight = h
	needDirty := false
	for _, win := range windows {
		size := win.GetSize()
		resized := false
		if size.X > float32(screenWidth) {
			if win.NoScale {
				win.Size.X = float32(screenWidth)
			} else {
				win.Size.X = float32(screenWidth) / uiScale
			}
			resized = true
		}
		if size.Y > float32(screenHeight) {
			if win.NoScale {
				win.Size.Y = float32(screenHeight)
			} else {
				win.Size.Y = float32(screenHeight) / uiScale
			}
			resized = true
		}
		if win.AutoSize {
			win.updateAutoSize()
			win.adjustScrollForResize()
			needDirty = true
		} else if resized {
			win.resizeFlows()
			win.adjustScrollForResize()
			needDirty = true
			if win.OnResize != nil {
				win.OnResize()
			}
		}
		if win.zone != nil {
			win.updateZonePosition()
		}
		win.clampToScreen()
	}
	if needDirty {
		markAllDirty()
	}

}

// ScreenSize returns the current screen size.
func ScreenSize() (int, int) { return screenWidth, screenHeight }

// SetFontSource sets the text face source used when rendering text.
func SetFontSource(src *text.GoTextFaceSource) {
	mplusFaceSource = src
	faceCache = map[float64]*text.GoTextFace{}
}

// SetBoldFontSource sets the bold text face source used when rendering bold text.
func SetBoldFontSource(src *text.GoTextFaceSource) {
	mplusBoldFaceSource = src
	boldFaceCache = map[float64]*text.GoTextFace{}
}

// FontSource returns the current text face source.
func FontSource() *text.GoTextFaceSource { return mplusFaceSource }

// BoldFontSource returns the current bold text face source.
func BoldFontSource() *text.GoTextFaceSource { return mplusBoldFaceSource }

// EnsureFontSource initializes the font source from ttf data if needed.
func EnsureFontSource(ttf []byte) error {
	if mplusFaceSource != nil {
		return nil
	}
	s, err := text.NewGoTextFaceSource(bytes.NewReader(ttf))
	if err != nil {
		return err
	}
	mplusFaceSource = s
	faceCache = map[float64]*text.GoTextFace{}
	return nil
}

// EnsureBoldFontSource initializes the bold font source from ttf data if needed.
func EnsureBoldFontSource(ttf []byte) error {
	if mplusBoldFaceSource != nil {
		return nil
	}
	s, err := text.NewGoTextFaceSource(bytes.NewReader(ttf))
	if err != nil {
		return err
	}
	mplusBoldFaceSource = s
	boldFaceCache = map[float64]*text.GoTextFace{}
	return nil
}

// ShowContextMenu opens a simple context menu at screen-space pixel
// coordinates (x, y) with the provided options. onSelect is called with the
// selected index when an item is clicked. Returns the handle to the menu.
// (context menu APIs are defined in context_menu.go)

// AddItem appends a child item to the parent item.
func (parent *ItemData) AddItem(child *ItemData) { parent.addItemTo(child) }

// AddItem appends a child item to the window.
func (win *WindowData) AddItem(child *ItemData) { win.addItemTo(child) }

// PrependItem prepends a child item to the parent item.
func (parent *ItemData) PrependItem(child *ItemData) { parent.prependItemTo(child) }

// PrependItem prepends a child item to the window.
func (win *WindowData) PrependItem(child *ItemData) { win.prependItemTo(child) }

// RemoveItem removes a child item from the parent item.
func (parent *ItemData) RemoveItem(child *ItemData) { parent.removeItem(child) }

// RemoveItem removes a child item from the window.
func (win *WindowData) RemoveItem(child *ItemData) { win.removeItem(child) }

// InsertItem inserts a child item into the parent item at the provided index.
func (parent *ItemData) InsertItem(idx int, child *ItemData) { parent.insertItem(idx, child) }

// InsertItem inserts a child item into the window at the provided index.
func (win *WindowData) InsertItem(idx int, child *ItemData) { win.insertItem(idx, child) }

// ReplaceItem replaces the child item at the provided index with the new item.
func (parent *ItemData) ReplaceItem(idx int, child *ItemData) { parent.replaceItem(idx, child) }

// ReplaceItem replaces the child item at the provided index in the window.
func (win *WindowData) ReplaceItem(idx int, child *ItemData) { win.replaceItem(idx, child) }

// UpdateText updates the text content and marks the item dirty when it changes.
func (item *ItemData) UpdateText(text string) { item.updateText(text) }

// SetProgressIndeterminate updates the indeterminate state of a progress bar
// and refreshes its parent window's tracking flag.
func SetProgressIndeterminate(pb *ItemData, ind bool) {
	if pb == nil || pb.ItemType != ITEM_PROGRESS {
		return
	}
	if pb.Indeterminate == ind {
		return
	}
	pb.Indeterminate = ind
	if pb.ParentWindow != nil {
		pb.ParentWindow.updateHasIndeterminate()
	}
	pb.markDirty()
}

// ListThemes returns the available palette names.
func ListThemes() ([]string, error) { return listThemes() }

// ListStyles returns the available style theme names.
func ListStyles() ([]string, error) { return listStyles() }

// CurrentThemeName returns the active theme name.
func CurrentThemeName() string { return currentThemeName }

// SetCurrentThemeName updates the active theme name.
func SetCurrentThemeName(name string) {
	currentThemeName = name
	updateThemePath()
}

// CurrentStyleName returns the active style theme name.
func CurrentStyleName() string { return currentStyleName }

// SetCurrentStyleName updates the active style theme name.
func SetCurrentStyleName(name string) {
	currentStyleName = name
	updateStylePath()
}

// AccentSaturation returns the current accent color saturation value.
func AccentSaturation() float64 { return accentSaturation }

// AccentColor returns the current accent color.
func AccentColor() Color {
	return Color(hsvaToRGBA(accentHue, accentSaturation, accentValue, accentAlpha))
}

// ClearFocus removes focus from the provided item if it is currently focused.
func ClearFocus(it *ItemData) {
	if focusedItem == it {
		focusedItem.Focused = false
		focusedItem.markDirty()
		focusedItem = nil
	}
}

// SetActiveSearchForTest sets the active search window for tests.
// This helper mirrors logic used internally when a window's search box
// is activated. It exists so external tests can simulate an active
// search without poking unexported symbols.
func SetActiveSearchForTest(win *WindowData) {
	activeSearch = win
	if win != nil {
		win.searchOpen = true
	}
}

// ScrollbarWidth returns the pixel width used when rendering scrollbars
// so callers can reserve space and avoid overlap.
func ScrollbarWidth() float32 {
	// Matches the value used in draw routines.
	return currentStyle.BorderPad.Slider * 2
}
