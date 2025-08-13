//go:build !test

package main

import (
	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
)

// MapTile describes a tile on the world map.
type MapTile struct {
	PictID uint16
	Frame  int
	X      int
	Y      int
}

// GetMapTiles retrieves the map tiles to be drawn.
// Placeholder implementation; the real function should
// return the current visible tiles.
func GetMapTiles() []MapTile { return nil }

var (
	mapWin       *eui.WindowData
	mapImageItem *eui.ItemData
	mapImage     *ebiten.Image

	mapOffsetX int
	mapOffsetY int
	mapZoom    float64 = 1.0

	dragging   bool
	lastMouseX int
	lastMouseY int
)

// updateMapWindow redraws the map window and handles input for
// panning and zooming.
func updateMapWindow() {
	if mapWin == nil {
		return
	}

	// Lazily create the image item to draw on.
	if mapImageItem == nil || mapImage == nil {
		w := int(mapWin.Size.X)
		h := int(mapWin.Size.Y)
		mapImageItem, mapImage = eui.NewImageItem(w, h)
		mapWin.AddItem(mapImageItem)
	}

	// Handle dragging for panning.
	mx, my := ebiten.CursorPosition()
	if dragging {
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			mapOffsetX += mx - lastMouseX
			mapOffsetY += my - lastMouseY
		} else {
			dragging = false
		}
	} else if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		// Start dragging only if the cursor is within the window bounds.
		s := float64(eui.UIScale())
		wx := int(mapWin.Position.X * float32(s))
		wy := int(mapWin.Position.Y * float32(s))
		ww := int(mapWin.Size.X * float32(s))
		wh := int(mapWin.Size.Y * float32(s))
		if mx >= wx && mx <= wx+ww && my >= wy && my <= wy+wh {
			dragging = true
		}
	}
	lastMouseX, lastMouseY = mx, my

	// Zoom using the scroll wheel.
	_, wy := ebiten.Wheel()
	if wy != 0 {
		mapZoom *= 1 + wy*0.1
		if mapZoom < 0.25 {
			mapZoom = 0.25
		}
		if mapZoom > 4 {
			mapZoom = 4
		}
	}

	mapImage.Clear()

	tiles := GetMapTiles()
	for _, t := range tiles {
		var img *ebiten.Image
		if t.Frame > 0 {
			img = loadImageFrame(t.PictID, t.Frame)
		} else {
			img = loadImage(t.PictID)
		}
		if img == nil {
			continue
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(mapZoom, mapZoom)
		op.GeoM.Translate(float64(t.X+mapOffsetX)*mapZoom, float64(t.Y+mapOffsetY)*mapZoom)
		mapImage.DrawImage(img, op)
	}

	mapImageItem.Dirty = true
	mapWin.Refresh()
}
