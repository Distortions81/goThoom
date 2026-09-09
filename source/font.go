package main

import (
	"bytes"
	_ "embed"
	"log"

	"gothoom/eui"

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

//go:embed data/font/NotoColorEmoji.ttf
var notoColorEmoji []byte

var emojiFaceSource *text.GoTextFaceSource

type emojiFaceKey struct {
	source *text.GoTextFaceSource
	size   float64
}

var emojiFaces = make(map[emojiFaceKey]text.Face)

// withEmojiFace keeps ordinary text in its chosen font and gives emoji glyphs
// priority over monochrome symbols. Reuse the composed faces for layout caches.
func withEmojiFace(face text.Face) text.Face {
	base, ok := face.(*text.GoTextFace)
	if !ok || base.Source == nil || emojiFaceSource == nil {
		return face
	}
	key := emojiFaceKey{base.Source, base.Size}
	if cached := emojiFaces[key]; cached != nil {
		return cached
	}
	emoji := text.NewLimitedFace(&text.GoTextFace{Source: emojiFaceSource, Size: base.Size})
	// Keep spaces, punctuation, and ordinary digits in the text font.
	emoji.AddUnicodeRange(0x80, 0x10ffff)
	mixed, err := text.NewMultiFace(emoji, base)
	if err != nil {
		return face
	}
	if len(emojiFaces) >= 256 {
		clear(emojiFaces)
	}
	emojiFaces[key] = mixed
	return mixed
}

var mainFont, mainFontBold, mainFontItalic, mainFontBoldItalic, bubbleFont, bubbleFontRegular text.Face
var monoFaceSource *text.GoTextFaceSource
var fontGen uint32
var mainFontRasterScale = 1.0

func initFont() {
	fontGen++
	clear(emojiFaces)
	clearBubbleTextCaches()
	clearSharedNameTagCache()
	regular, err := text.NewGoTextFaceSource(bytes.NewReader(notoSansRegular))
	if err != nil {
		log.Fatalf("failed to parse font: %v", err)
	}
	if emojiFaceSource == nil {
		emojiFaceSource, err = text.NewGoTextFaceSource(bytes.NewReader(notoColorEmoji))
		if err != nil {
			log.Fatalf("failed to parse emoji font: %v", err)
		}
	}
	eui.SetFontSource(regular)
	mainFontRasterScale = gs.GameScale
	if mainFontRasterScale <= 0 {
		mainFontRasterScale = 1
	}
	mainFont = &text.GoTextFace{
		Source: regular,
		Size:   gs.MainFontSize * mainFontRasterScale,
	}

	bold, err := text.NewGoTextFaceSource(bytes.NewReader(notoSansBold))
	if err != nil {
		log.Fatalf("failed to parse font: %v", err)
	}
	mainFontBold = &text.GoTextFace{
		Source: bold,
		Size:   gs.MainFontSize * mainFontRasterScale,
	}
	eui.SetBoldFontSource(bold)

	italic, err := text.NewGoTextFaceSource(bytes.NewReader(notoSansItalic))
	if err != nil {
		log.Fatalf("failed to parse font: %v", err)
	}
	mainFontItalic = &text.GoTextFace{
		Source: italic,
		Size:   gs.MainFontSize * mainFontRasterScale,
	}

	boldItalic, err := text.NewGoTextFaceSource(bytes.NewReader(notoSansBoldItalic))
	if err != nil {
		log.Fatalf("failed to parse font: %v", err)
	}
	mainFontBoldItalic = &text.GoTextFace{
		Source: boldItalic,
		Size:   gs.MainFontSize * mainFontRasterScale,
	}

	//Bubble
	bubbleFont = &text.GoTextFace{
		Source: bold,
		Size:   gs.BubbleFontSize,
	}
	bubbleFontRegular = &text.GoTextFace{
		Source: regular,
		Size:   gs.BubbleFontSize,
	}
}
