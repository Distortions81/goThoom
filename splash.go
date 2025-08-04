package main

import (
	"bytes"
	_ "embed"
	"image"
	_ "image/png"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed splash.png
var splashPNG []byte

var splashImg *ebiten.Image

func init() {
	img, _, err := image.Decode(bytes.NewReader(splashPNG))
	if err != nil {
		log.Printf("decode splash: %v", err)
		return
	}
	splashImg = ebiten.NewImageFromImage(img)
}

func drawSplash(screen *ebiten.Image) {
	if splashImg == nil {
		return
	}
	sw, sh := screen.Size()
	iw, ih := splashImg.Bounds().Dx(), splashImg.Bounds().Dy()
	scaleX := float64(sw) / float64(iw)
	scaleY := float64(sh) / float64(ih)
	s := scaleX
	if scaleY < s {
		s = scaleY
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate((float64(sw)-float64(iw)*s)/2, (float64(sh)-float64(ih)*s)/2)
	screen.DrawImage(splashImg, op)
}
