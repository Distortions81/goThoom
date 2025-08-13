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

// FontSource returns the current text face source.
func FontSource() *text.GoTextFaceSource { return mplusFaceSource }

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

// AddItem appends a child item to the parent item.
func (parent *ItemData) AddItem(child *ItemData) { parent.addItemTo(child) }

// AddItem appends a child item to the window.
func (win *WindowData) AddItem(child *ItemData) { win.addItemTo(child) }

// ListThemes returns the available palette names.
func ListThemes() ([]string, error) { return listThemes() }

// ListStyles returns the available style theme names.
func ListStyles() ([]string, error) { return listStyles() }

// CurrentThemeName returns the active theme name.
func CurrentThemeName() string { return currentThemeName }

// SetCurrentThemeName updates the active theme name.
func SetCurrentThemeName(name string) { currentThemeName = name }

// CurrentStyleName returns the active style theme name.
func CurrentStyleName() string { return currentStyleName }

// SetCurrentStyleName updates the active style theme name.
func SetCurrentStyleName(name string) { currentStyleName = name }

// AccentSaturation returns the current accent color saturation value.
func AccentSaturation() float64 { return accentSaturation }
