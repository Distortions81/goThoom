package main

import (
	"bytes"
	_ "embed"
	"log"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

//go:embed data/font/NotoSans-Regular.ttf
var notoSansRegular []byte

//go:embed data/font/NotoSans-Bold.ttf
var notoSansBold []byte

//go:embed data/font/NotoSans-Italic.ttf
var notoSansItalic []byte

//go:embed data/font/NotoSans-BoldItalic.ttf
var notoSansBoldItalic []byte

var mainFont, mainFontBold, mainFontItalic, mainFontBoldItalic, bubbleFont text.Face

func initFont() {
	size := gs.MainFontSize
	bubbleSize := gs.BubbleFontSize
	if !gs.AnyGameWindowSize {
		size *= gs.GameScale
		bubbleSize *= gs.GameScale
	}

	regular, err := text.NewGoTextFaceSource(bytes.NewReader(notoSansRegular))
	if err != nil {
		log.Fatalf("failed to parse font: %v", err)
	}
	mainFont = &text.GoTextFace{
		Source: regular,
		Size:   size,
	}

	bold, err := text.NewGoTextFaceSource(bytes.NewReader(notoSansBold))
	if err != nil {
		log.Fatalf("failed to parse font: %v", err)
	}
	mainFontBold = &text.GoTextFace{
		Source: bold,
		Size:   size,
	}

	italic, err := text.NewGoTextFaceSource(bytes.NewReader(notoSansItalic))
	if err != nil {
		log.Fatalf("failed to parse font: %v", err)
	}
	mainFontItalic = &text.GoTextFace{
		Source: italic,
		Size:   size,
	}

	boldItalic, err := text.NewGoTextFaceSource(bytes.NewReader(notoSansBoldItalic))
	if err != nil {
		log.Fatalf("failed to parse font: %v", err)
	}
	mainFontBoldItalic = &text.GoTextFace{
		Source: boldItalic,
		Size:   size,
	}

	//Bubble
	bubbleFont = &text.GoTextFace{
		Source: bold,
		Size:   bubbleSize,
	}
}
