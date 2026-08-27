package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	brandSpritePictID = uint16(3003)
	brandSpriteState  = uint8(8) // canonical forward-facing pose
	brandSpriteSize   = 375
	brandSpriteScale  = 4
	brandSpriteHeight = 350
)

var brandSpriteColors = []byte{
	0x24, 0x2b, 0x08, 0x3a, 0x65, 0x81, 0xac, 0xff, 0x01, 0x07,
	0x0e, 0x74, 0xa5, 0x81, 0x2b, 0x81, 0x2b, 0x81, 0x6c, 0xc2,
}

// exportBrandSprite renders the hardcoded goThoom character through the same
// sprite loader and Ultra Smooth upscale shader used by the game. Rendering
// runs inside Ebiten's game loop because shader output must be read on its UI
// thread.
func exportBrandSprite(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	previousSettings := gs
	gs.SpriteUpscale = brandSpriteScale
	gs.SpriteUpscaleFilter = true
	gs.SpriteUpscaleMode = artworkUpscaleUltraSmooth
	defer func() { gs = previousSettings }()

	ebiten.SetWindowSize(brandSpriteSize, brandSpriteSize)
	ebiten.SetWindowTitle("goThoom sprite export")
	game := &brandSpriteExportGame{path: path}
	if err := ebiten.RunGame(game); err != nil {
		return err
	}
	return game.err
}

type brandSpriteExportGame struct {
	path     string
	rendered bool
	err      error
}

func (g *brandSpriteExportGame) Update() error {
	if g.rendered {
		return ebiten.Termination
	}
	return nil
}

func (g *brandSpriteExportGame) Draw(screen *ebiten.Image) {
	sprite := loadMobileFrame(brandSpritePictID, brandSpriteState, brandSpriteColors)
	if sprite == nil {
		g.err = fmt.Errorf("load mobile pict %d", brandSpritePictID)
		g.rendered = true
		return
	}

	upscaled := upscaleTransientSpriteImageWithMode(sprite, brandSpriteScale, artworkUpscaleMode())
	defer upscaled.Deallocate()
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
	iconScale := float64(brandSpriteHeight) / float64(upscaled.Bounds().Dy())
	op.GeoM.Scale(iconScale, iconScale)
	drawnWidth := float64(upscaled.Bounds().Dx()) * iconScale
	drawnHeight := float64(upscaled.Bounds().Dy()) * iconScale
	op.GeoM.Translate(
		(float64(brandSpriteSize)-drawnWidth)/2,
		(float64(brandSpriteSize)-drawnHeight)/2,
	)
	screen.DrawImage(upscaled, op)

	pixels := make([]byte, brandSpriteSize*brandSpriteSize*4)
	screen.ReadPixels(pixels)
	img := &image.RGBA{
		Pix: pixels, Stride: brandSpriteSize * 4,
		Rect: image.Rect(0, 0, brandSpriteSize, brandSpriteSize),
	}
	file, err := os.Create(g.path)
	if err == nil {
		err = png.Encode(file, img)
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}
	g.err = err
	g.rendered = true
}

func (*brandSpriteExportGame) Layout(_, _ int) (int, int) {
	return brandSpriteSize, brandSpriteSize
}
