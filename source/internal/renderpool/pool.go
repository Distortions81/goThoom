// Package renderpool recycles render-target allocations. Ebitengine owns atlas
// placement; the pool only lends exclusive, zero-origin views of live images.
package renderpool

import (
	"image"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
)

type allocation struct {
	parent    *ebiten.Image
	size      image.Point
	unmanaged bool
	bytes     int64
}

type Stats struct {
	Active, Free           int
	ActiveBytes, FreeBytes int64
	Allocations, Reuses    uint64
}

// Pool must not be copied after first use. Callers must return each acquired
// view with Release and must not retain or draw it after releasing it.
// Previously submitted draws remain ordered before the next Clear/repaint.
type Pool struct {
	MaxFreeBytes int64
	// Optional hooks preserve the caller's allocation diagnostics.
	NewImage   func(width, height int, unmanaged bool) *ebiten.Image
	Deallocate func(*ebiten.Image)

	mu     sync.Mutex
	active map[*ebiten.Image]*allocation
	free   []*allocation // oldest release first
	stats  Stats
}

func dimensions(width, height int, unmanaged bool) image.Point {
	if unmanaged {
		// Standalone images have no atlas border. Preserve exact powers of two,
		// including images at a compatibility device's texture-size limit.
		return image.Pt((max(1, width)+31)/32*32, (max(1, height)+31)/32*32)
	}
	// Include Ebitengine's managed-image border in the 32-pixel size class.
	// This avoids turning an exact power-of-two class into a larger atlas node.
	return image.Pt((max(1, width)+1+31)/32*32-1, (max(1, height)+1+31)/32*32-1)
}

// BytesForSize estimates the allocation area, including Ebitengine's border
// for managed images, or standalone power-of-two rounding for unmanaged ones.
// Shared atlas packing gaps are additional.
func BytesForSize(width, height int, unmanaged bool) int64 {
	size := dimensions(width, height, unmanaged)
	if !unmanaged {
		return int64(size.X+1) * int64(size.Y+1) * 4
	}
	pow2 := func(n int) int64 {
		v := int64(16)
		for v < int64(n) {
			v *= 2
		}
		return v
	}
	return pow2(size.X) * pow2(size.Y) * 4
}

func (p *Pool) Acquire(width, height int, unmanaged bool) *ebiten.Image {
	p.mu.Lock()
	defer p.mu.Unlock()
	size := dimensions(width, height, unmanaged)
	wanted := BytesForSize(width, height, unmanaged)
	best := -1
	for index, slot := range p.free {
		if slot.unmanaged != unmanaged || slot.size.X < size.X || slot.size.Y < size.Y || slot.bytes > wanted*2 {
			continue
		}
		if best < 0 || slot.bytes < p.free[best].bytes {
			best = index
		}
	}
	var slot *allocation
	if best >= 0 {
		slot = p.free[best]
		copy(p.free[best:], p.free[best+1:])
		p.free[len(p.free)-1] = nil
		p.free = p.free[:len(p.free)-1]
		p.stats.FreeBytes -= slot.bytes
		p.stats.Reuses++
		// Clear the entire parent, including padding left outside a smaller
		// view. This uses ordered GPU commands, with no pixel readback.
		slot.parent.Clear()
	} else {
		var parent *ebiten.Image
		if p.NewImage != nil {
			parent = p.NewImage(size.X, size.Y, unmanaged)
		} else {
			parent = ebiten.NewImageWithOptions(image.Rectangle{Max: size}, &ebiten.NewImageOptions{Unmanaged: unmanaged})
		}
		slot = &allocation{parent: parent, size: size, unmanaged: unmanaged, bytes: wanted}
		p.stats.Allocations++
	}
	view := slot.parent.SubImage(image.Rect(0, 0, max(1, width), max(1, height))).(*ebiten.Image)
	if p.active == nil {
		p.active = make(map[*ebiten.Image]*allocation)
	}
	p.active[view] = slot
	p.stats.ActiveBytes += slot.bytes
	return view
}

// Bytes returns the area of the allocation backing an acquired view.
func (p *Pool) Bytes(view *ebiten.Image) int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	if slot := p.active[view]; slot != nil {
		return slot.bytes
	}
	return 0
}

// Release returns false for images not owned by this pool. It retains a
// bounded reserve for reuse; excess idle images are explicitly deallocated.
func (p *Pool) Release(view *ebiten.Image) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	slot := p.active[view]
	if slot == nil {
		return false
	}
	delete(p.active, view)
	p.stats.ActiveBytes -= slot.bytes
	p.free = append(p.free, slot)
	p.stats.FreeBytes += slot.bytes
	for len(p.free) > 128 || p.stats.FreeBytes > max(int64(0), p.MaxFreeBytes) {
		old := p.free[0]
		p.free[0] = nil
		p.free = p.free[1:]
		p.stats.FreeBytes -= old.bytes
		p.dispose(old.parent)
	}
	return true
}

func (p *Pool) dispose(img *ebiten.Image) {
	if p.Deallocate != nil {
		p.Deallocate(img)
	} else {
		img.Deallocate()
	}
}

// Clear releases the idle reserve. Active views remain valid.
func (p *Pool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, slot := range p.free {
		p.dispose(slot.parent)
	}
	p.free = nil
	p.stats.FreeBytes = 0
}

func (p *Pool) Stats() Stats {
	p.mu.Lock()
	defer p.mu.Unlock()
	s := p.stats
	s.Active = len(p.active)
	s.Free = len(p.free)
	return s
}
