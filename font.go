package main

import (
	"bytes"
	_ "embed"
	"log"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

//go:embed font.ttf
var fontData []byte

var nameFace text.Face

func initFont() {
	src, err := text.NewGoTextFaceSource(bytes.NewReader(fontData))
	if err != nil {
		log.Fatalf("failed to parse font: %v", err)
	}
	nameFace = &text.GoTextFace{
		Source: src,
		Size:   10 * float64(scale),
	}
}
