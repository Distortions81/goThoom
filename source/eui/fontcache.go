package eui

import "github.com/hajimehoshi/ebiten/v2/text/v2"

var faceCache = map[float64]*text.GoTextFace{}
var boldFaceCache = map[float64]*text.GoTextFace{}

func textFace(size float32) *text.GoTextFace {
	if mplusFaceSource == nil {
		return &text.GoTextFace{Size: float64(size)}
	}
	s := float64(size)
	if f, ok := faceCache[s]; ok {
		return f
	}
	f := &text.GoTextFace{Source: mplusFaceSource, Size: s}
	faceCache[s] = f
	return f
}

func boldFace(size float32) *text.GoTextFace {
	if mplusBoldFaceSource == nil {
		return textFace(size)
	}
	s := float64(size)
	if f, ok := boldFaceCache[s]; ok {
		return f
	}
	f := &text.GoTextFace{Source: mplusBoldFaceSource, Size: s}
	boldFaceCache[s] = f
	return f
}
