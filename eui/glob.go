//go:build !test

package eui

import (
	"image"
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

var (
	screenWidth  = 1024
	screenHeight = 1024

	mplusFaceSource  *text.GoTextFaceSource
	windows          []*windowData
	overlays         []*itemData
	activeWindow     *windowData
	focusedItem      *itemData
	hoveredItem      *itemData
	uiScale          float32 = 1.0
	currentTheme     *Theme
	currentThemeName string = "AccentDark"
	clickFlash              = time.Millisecond * 100

	// DebugMode enables rendering of debug outlines.
	DebugMode bool

	// DumpMode causes the library to write cached images to disk
	// before exiting when enabled.
	DumpMode bool

	// TreeMode dumps the window hierarchy to debug/tree.json
	// before exiting when enabled.
	TreeMode bool

	whiteImage    = ebiten.NewImage(3, 3)
	whiteSubImage = whiteImage.SubImage(image.Rect(1, 1, 2, 2)).(*ebiten.Image)
)

func init() {
	whiteImage.Fill(color.White)
}

// constants moved to const.go

// RenderSize sets the current screen size from Ebiten's layout values.
// Pass Ebiten's outside size values to this from your Layout function.
func RenderSize(outsideWidth, outsideHeight int) {
	if outsideWidth != screenWidth || outsideHeight != screenHeight {
		SetScreenSize(outsideWidth, outsideHeight)
	}
}
