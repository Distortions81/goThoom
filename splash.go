package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/draw"
	"log"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed data/images/splash.png

var splashPNG []byte

var splashImg *ebiten.Image

func init() {
	img, _, err := image.Decode(bytes.NewReader(splashPNG))
	if err != nil {
		log.Printf("decode splash: %v", err)
		return
	}
	b := img.Bounds()
	withBorder := image.NewRGBA(image.Rect(0, 0, b.Dx()+2, b.Dy()+2))
	draw.Draw(withBorder, image.Rect(1, 1, b.Dx()+1, b.Dy()+1), img, b.Min, draw.Src)
	splashImg = newImageFromImage(withBorder)
}

func drawSplash(screen *ebiten.Image, ox, oy int) {
	if splashImg == nil {
		return
	}
	sw := int(math.Round(float64(gameAreaSizeX) * gs.GameScale))
	sh := int(math.Round(float64(gameAreaSizeY) * gs.GameScale))
	iw, ih := splashImg.Bounds().Dx(), splashImg.Bounds().Dy()
	scaleX := float64(sw) / float64(iw)
	scaleY := float64(sh) / float64(ih)
	s := scaleX
	if scaleY < s {
		s = scaleY
	}
	scaledW := math.Round(float64(iw) * s)
	scaledH := math.Round(float64(ih) * s)
	sx := scaledW / float64(iw)
	sy := scaledH / float64(ih)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(sx, sy)
	tx := math.Round(float64(ox) + (float64(sw)-scaledW)/2)
	ty := math.Round(float64(oy) + (float64(sh)-scaledH)/2)
	op.GeoM.Translate(tx, ty)
	screen.DrawImage(splashImg, op)
}
