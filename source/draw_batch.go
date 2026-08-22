package main

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

// rectBatch batches solid rectangle draws into a single DrawTriangles call.
type rectBatch struct {
	vs []ebiten.Vertex
	is []uint16
}

// Add appends a rectangle at the given destination coordinates and size using
// the provided RGBA color.
func (b *rectBatch) Add(x, y, w, h float32, clr color.RGBA) {
	start := uint16(len(b.vs))
	r, g, bcol, a := float32(clr.R)/255, float32(clr.G)/255, float32(clr.B)/255, float32(clr.A)/255
	b.vs = append(b.vs,
		ebiten.Vertex{DstX: x, DstY: y, SrcX: 0, SrcY: 0, ColorR: r, ColorG: g, ColorB: bcol, ColorA: a},
		ebiten.Vertex{DstX: x + w, DstY: y, SrcX: 1, SrcY: 0, ColorR: r, ColorG: g, ColorB: bcol, ColorA: a},
		ebiten.Vertex{DstX: x, DstY: y + h, SrcX: 0, SrcY: 1, ColorR: r, ColorG: g, ColorB: bcol, ColorA: a},
		ebiten.Vertex{DstX: x + w, DstY: y + h, SrcX: 1, SrcY: 1, ColorR: r, ColorG: g, ColorB: bcol, ColorA: a},
	)
	b.is = append(b.is, start, start+1, start+2, start+1, start+3, start+2)
}

// Draw flushes the accumulated rectangles onto dst and resets the batch.
func (b *rectBatch) Draw(dst *ebiten.Image) {
	if len(b.is) == 0 {
		return
	}
	dst.DrawTriangles(b.vs, b.is, whiteImage, nil)
	b.vs = b.vs[:0]
	b.is = b.is[:0]
}
