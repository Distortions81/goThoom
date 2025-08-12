package eui

import (
	"sync"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

var (
	faceCache   = map[float64]*text.GoTextFace{}
	faceCacheMu sync.Mutex
)

func textFace(size float32) *text.GoTextFace {
	if mplusFaceSource == nil {
		return &text.GoTextFace{Size: float64(size)}
	}
	s := float64(size)
	faceCacheMu.Lock()
	defer faceCacheMu.Unlock()
	if f, ok := faceCache[s]; ok {
		return f
	}
	f := &text.GoTextFace{Source: mplusFaceSource, Size: s}
	faceCache[s] = f
	return f
}
