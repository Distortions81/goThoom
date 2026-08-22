package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/draw"
	"log"
	"math"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed data/images/splash.png

var splashPNG []byte

var splashImg *ebiten.Image
var backgroundImg *ebiten.Image

func init() {
	img, _, err := image.Decode(bytes.NewReader(splashPNG))
	if err != nil {
		log.Printf("decode splash: %v", err)
	} else {
		b := img.Bounds()
		withBorder := image.NewRGBA(image.Rect(0, 0, b.Dx()+2, b.Dy()+2))
		draw.Draw(withBorder, image.Rect(1, 1, b.Dx()+1, b.Dy()+1), img, b.Min, draw.Src)
		splashImg = newImageFromImage(withBorder)
	}

	if f, err := os.Open(filepath.Join("data", "splash.png")); err == nil {
		defer f.Close()
		if img, _, err := image.Decode(f); err == nil {
			b := img.Bounds()
			withBorder := image.NewRGBA(image.Rect(0, 0, b.Dx()+2, b.Dy()+2))
			draw.Draw(withBorder, image.Rect(1, 1, b.Dx()+1, b.Dy()+1), img, b.Min, draw.Src)
			splashImg = newImageFromImage(withBorder)
		} else {
			log.Printf("decode custom splash: %v", err)
		}
	}

	if f, err := os.Open(filepath.Join("data", "background.png")); err == nil {
		defer f.Close()
		if img, _, err := image.Decode(f); err == nil {
			backgroundImg = newImageFromImage(img)
		} else {
			log.Printf("decode background: %v", err)
		}
	}
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
	if scaleY > s {
		s = scaleY
	}
	scaledW := float64(iw) * s
	scaledH := float64(ih) * s
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear, DisableMipmaps: true}
	op.GeoM.Scale(s, s)
	tx := float64(ox) + (float64(sw)-scaledW)/2
	ty := float64(oy) + (float64(sh)-scaledH)/2
	op.GeoM.Translate(tx, ty)
	screen.DrawImage(splashImg, op)
}

func drawBackground(screen *ebiten.Image) {
	if backgroundImg == nil {
		return
	}
	sw := screen.Bounds().Dx()
	sh := screen.Bounds().Dy()
	iw, ih := backgroundImg.Bounds().Dx(), backgroundImg.Bounds().Dy()
	scaleX := float64(sw) / float64(iw)
	scaleY := float64(sh) / float64(ih)
	s := scaleX
	if scaleY > s {
		s = scaleY
	}
	scaledW := float64(iw) * s
	scaledH := float64(ih) * s
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear, DisableMipmaps: true}
	op.GeoM.Scale(s, s)
	tx := (float64(sw) - scaledW) / 2
	ty := (float64(sh) - scaledH) / 2
	op.GeoM.Translate(tx, ty)
	screen.DrawImage(backgroundImg, op)
}
