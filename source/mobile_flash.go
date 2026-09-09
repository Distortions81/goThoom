package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/colorm"
)

// drawMobileFlashImage matches the shader paths: replace RGB after normal tint
// and shading, retaining the sprite alpha. At full strength even black artwork
// becomes the chosen flash color.
func drawMobileFlashImage(destination, sprite *ebiten.Image, options frameBlendDrawOptions) {
	flash := options.FlashColor
	remaining := float64(1 - flash[3])
	var matrix colorm.ColorM
	matrix.Scale(float64(options.Red)*remaining, float64(options.Green)*remaining, float64(options.Blue)*remaining, float64(options.Alpha))
	matrix.Translate(float64(flash[0]*flash[3]), float64(flash[1]*flash[3]), float64(flash[2]*flash[3]), 0)
	op := &colorm.DrawImageOptions{}
	if options.Linear {
		op.Filter = ebiten.FilterLinear
	}
	op.GeoM.Scale(options.ScaleX, options.ScaleY)
	op.GeoM.Translate(options.Left, options.Top)
	colorm.DrawImage(destination, sprite, matrix, op)
}
