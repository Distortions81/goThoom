package eui

import (
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

var (
	screenWidth  = 1024
	screenHeight = 1024

	mplusFaceSource     *text.GoTextFaceSource
	mplusBoldFaceSource *text.GoTextFaceSource
	windows             []*windowData
	activeWindow        *windowData
	focusedItem         *itemData
	hoveredItem         *itemData
	uiScale             float32 = 1.0
	currentTheme        *Theme
	currentThemeName    string = "AccentDark"
	clickFlash                 = time.Millisecond * 100

	// DebugMode enables rendering of debug outlines.
	DebugMode bool

	// DumpMode causes the library to write cached images to disk
	// before exiting when enabled.
	DumpMode bool

	// TreeMode dumps the window hierarchy to debug/tree.json
	// before exiting when enabled.
	TreeMode bool

	// CacheCheck shows render counts for windows and items when enabled.
	CacheCheck bool

	// windowSnapping snaps windows to screen edges or other windows when enabled.
	windowSnapping bool = true

	// middleClickMove enables moving windows with the middle mouse button when enabled.
	middleClickMove bool

	whiteImage = newImage(3, 3)

	// AutoHiDPI enables automatic scaling when the device scale factor
	// changes, keeping the UI size consistent on HiDPI displays. It is
	// enabled by default and can be disabled if needed.
	AutoHiDPI          bool    = true
	lastDeviceScale    float64 = 1.0
	lastScaleCheck     time.Time
	lastOutsideW       int
	lastOutsideH       int
	scaleCheckInterval = time.Second

	// WindowStateChanged is an optional callback fired when any window
	// is opened or closed.
	WindowStateChanged func()
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
		scale = lastDeviceScale
		now := time.Now()
		if outsideWidth != lastOutsideW || outsideHeight != lastOutsideH || now.Sub(lastScaleCheck) >= scaleCheckInterval {
			scale = ebiten.Monitor().DeviceScaleFactor()
			lastDeviceScale = scale
			lastScaleCheck = now
		}
	}
	lastOutsideW = outsideWidth
	lastOutsideH = outsideHeight

	scaledW := int(float64(outsideWidth) * scale)
	scaledH := int(float64(outsideHeight) * scale)
	if scaledW != screenWidth || scaledH != screenHeight {
		SetScreenSize(scaledW, scaledH)
	}
	return scaledW, scaledH
}
