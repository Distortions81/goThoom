package eui

import (
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

var drawImageOptionsPool = sync.Pool{
	New: func() any {
		return &ebiten.DrawImageOptions{}
	},
}

func acquireDrawImageOptions() *ebiten.DrawImageOptions {
	op := drawImageOptionsPool.Get().(*ebiten.DrawImageOptions)
	*op = ebiten.DrawImageOptions{}
	op.GeoM.Reset()
	op.ColorScale.Reset()
	return op
}

func releaseDrawImageOptions(op *ebiten.DrawImageOptions) {
	drawImageOptionsPool.Put(op)
}

var textDrawOptionsPool = sync.Pool{
	New: func() any {
		return &text.DrawOptions{}
	},
}

func acquireTextDrawOptions() *text.DrawOptions {
	op := textDrawOptionsPool.Get().(*text.DrawOptions)
	op.DrawImageOptions = ebiten.DrawImageOptions{}
	op.DrawImageOptions.GeoM.Reset()
	op.DrawImageOptions.ColorScale.Reset()
	op.LayoutOptions = text.LayoutOptions{}
	op.ColorScale.Reset()
	return op
}

func releaseTextDrawOptions(op *text.DrawOptions) {
	textDrawOptionsPool.Put(op)
}
