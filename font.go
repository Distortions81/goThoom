package main

import (
	"bytes"
	_ "embed"
	"log"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

//go:embed font.ttf
var fontData []byte

var mainFont, bubbleFont text.Face

var (
	mainFontSize   float64 = 10
	bubbleFontSize float64 = 7
)

func initFont() {
	src, err := text.NewGoTextFaceSource(bytes.NewReader(fontData))
	if err != nil {
		log.Fatalf("failed to parse font: %v", err)
	}
	mainFont = &text.GoTextFace{
		Source: src,
		Size:   mainFontSize * float64(scale),
	}

	src, err = text.NewGoTextFaceSource(bytes.NewReader(fontData))
	if err != nil {
		log.Fatalf("failed to parse font: %v", err)
	}
	bubbleFont = &text.GoTextFace{
		Source: src,
		Size:   bubbleFontSize * float64(scale),
	}
}
