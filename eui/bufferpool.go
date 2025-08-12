package eui

import (
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	defaultVertexCap = 64
	defaultIndexCap  = 128
	maxVertexCap     = 1024
	maxIndexCap      = 2048
)

var vertexPool = sync.Pool{
	New: func() any {
		return make([]ebiten.Vertex, 0, defaultVertexCap)
	},
}

var indexPool = sync.Pool{
	New: func() any {
		return make([]uint16, 0, defaultIndexCap)
	},
}

func getVertices() []ebiten.Vertex {
	return vertexPool.Get().([]ebiten.Vertex)[:0]
}

func putVertices(v []ebiten.Vertex) {
	if cap(v) > maxVertexCap {
		v = make([]ebiten.Vertex, 0, defaultVertexCap)
	} else {
		v = v[:0]
	}
	vertexPool.Put(v)
}

func getIndices() []uint16 {
	return indexPool.Get().([]uint16)[:0]
}

func putIndices(i []uint16) {
	if cap(i) > maxIndexCap {
		i = make([]uint16, 0, defaultIndexCap)
	} else {
		i = i[:0]
	}
	indexPool.Put(i)
}
