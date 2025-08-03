package main

import (
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
)

var (
	poolMu  sync.Mutex
	imgPool = make(map[int][]*ebiten.Image)
)

const (
	// maxUnusedSprites limits the number of cached images retained per size.
	maxUnusedSprites = 100
)

// nextPow2 returns the next power-of-two value >= n.
func nextPow2(n int) int {
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}

// getTempImage returns a temporary image with a power-of-two size.
// The image is cleared before being returned.
func getTempImage(size int) *ebiten.Image {
	s := nextPow2(size)
	poolMu.Lock()
	defer poolMu.Unlock()
	pool := imgPool[s]
	var img *ebiten.Image
	if n := len(pool); n > 0 {
		img = pool[n-1]
		imgPool[s] = pool[:n-1]
		img.Clear()
	} else {
		img = ebiten.NewImage(s, s)
	}
	return img
}

// recycleTempImage returns an image to the pool for reuse.
func recycleTempImage(img *ebiten.Image) {
	if img == nil {
		return
	}
	s := img.Bounds().Dx()
	poolMu.Lock()
	defer poolMu.Unlock()
	if len(imgPool[s]) < maxUnusedSprites {
		imgPool[s] = append(imgPool[s], img)
	} else {
		logDebug("recycleTempImage: full")
	}
}
