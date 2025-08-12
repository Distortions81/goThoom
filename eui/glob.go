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

	// AutoHiDPI enables automatic scaling when the device scale factor
	// changes, keeping the UI size consistent on HiDPI displays. It is
	// enabled by default and can be disabled if needed.
	AutoHiDPI       bool    = true
	lastDeviceScale float64 = 1.0
)

func init() {
	whiteImage.Fill(color.White)
}

// constants moved to const.go

// Layout reports the dimensions for the game's screen.
// Pass Ebiten's outside size values to this from your Layout function.
func Layout(outsideWidth, outsideHeight int) (int, int) {
	scale := 1.0
	if AutoHiDPI {
		scale = ebiten.Monitor().DeviceScaleFactor()
		if scale <= 0 {
			scale = 1
		}
		if scale != lastDeviceScale {
			SetUIScale(uiScale * float32(scale/lastDeviceScale))
			lastDeviceScale = scale
		}
	}

	scaledW := int(float64(outsideWidth) * scale)
	scaledH := int(float64(outsideHeight) * scale)
	if scaledW != screenWidth || scaledH != screenHeight {
		SetScreenSize(scaledW, scaledH)
	}
	return scaledW, scaledH
}
