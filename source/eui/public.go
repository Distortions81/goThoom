package eui

import (
	"bytes"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// Windows returns the list of active windows.
func Windows() []*WindowData { return windows }

// PointerPressWindow returns the window under the current pointer press. It
// remains available for the duration of the press even if that window closes.
func PointerPressWindow() *WindowData { return downWin }

// PointerPressHandled reports whether EUI handled the pointer press this frame.
func PointerPressHandled() bool { return pointerPressHandled }

// PointerCursorClaimed reports whether EUI selected a non-default cursor for
// the current frame. Callers that also manage the system cursor should not
// overwrite it while this is true.
func PointerCursorClaimed() bool { return cursorShape != ebiten.CursorShapeDefault }

// WindowSnapping reports whether window snapping is enabled.
func WindowSnapping() bool { return windowSnapping }

// SetWindowSnapping enables or disables window snapping.
func SetWindowSnapping(enabled bool) { windowSnapping = enabled }

// MiddleClickMove reports whether middle-click window dragging is enabled.
func MiddleClickMove() bool { return middleClickMove }

// SetMiddleClickMove enables or disables dragging windows with the middle mouse button.
func SetMiddleClickMove(enabled bool) { middleClickMove = enabled }

// SetKeyboardInputCaptured pauses EUI keyboard actions while leaving pointer
// and window controls active.
func SetKeyboardInputCaptured(captured bool) { keyboardInputCaptured = captured }

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
	updateAllTooltipBounds()
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

// SetUserDataRoot places editable themes and generated theme documentation
// under the application's persistent user data directory.
func SetUserDataRoot(root string) {
	if root == "" {
		themeDirectory = "themes"
	} else {
		themeDirectory = filepath.Join(root, "themes")
	}
	updateThemePath()
	updateStylePath()
	ensureThemeDocs()
	refreshThemeMod()
	refreshStyleMod()
}

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

// TextColor returns the active theme's normal text color.
func TextColor() Color {
	if currentTheme != nil {
		return currentTheme.Text.TextColor
	}
	return baseTheme.Text.TextColor
}

// IsLightTheme reports whether the active theme uses a light window background.
func IsLightTheme() bool {
	bg := baseTheme.Window.BGColor
	if currentTheme != nil {
		bg = currentTheme.Window.BGColor
	}
	// ITU-R BT.601 luma is sufficient here and avoids classifying saturated
	// backgrounds by their unweighted channel sum.
	luma := 299*int(bg.R) + 587*int(bg.G) + 114*int(bg.B)
	return luma >= 128000
}

// SubtleAlternateRowColor returns a low-contrast opaque row color derived
// from the active window background.
func SubtleAlternateRowColor() Color {
	bg := Color{R: 48, G: 48, B: 48, A: 255}
	if currentTheme != nil {
		bg = currentTheme.Window.BGColor
	}
	target := 255
	if int(bg.R)+int(bg.G)+int(bg.B) >= 384 {
		target = 0
	}
	blend := func(v uint8) uint8 {
		return uint8((int(v)*97 + target*3) / 100)
	}
	return Color{R: blend(bg.R), G: blend(bg.G), B: blend(bg.B), A: 255}
}

// ClearFocus removes focus from the provided item if it is currently focused.
func ClearFocus(it *ItemData) {
	if focusedItem == it {
		focusedItem.Focused = false
		focusedItem.markDirty()
		focusedItem = nil
	}
}

// Focus gives keyboard focus to a text input. It is useful for transient
// keyboard-driven windows that should be ready for typing as soon as opened.
func Focus(it *ItemData) {
	if it == nil || !itemAcceptsTextEditing(it) {
		return
	}
	if focusedItem != nil && focusedItem != it {
		focusedItem.Focused = false
		focusedItem.markDirty()
	}
	focusedItem = it
	it.CursorPos = len([]rune(it.Text))
	it.Focused = true
	it.markDirty()
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
