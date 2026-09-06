package renderpool

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestReuseClearsOwnershipAndPreservesAllocation(t *testing.T) {
	p := Pool{MaxFreeBytes: 1 << 20}
	first := p.Acquire(80, 48, false)
	parent := p.active[first].parent
	other := p.Acquire(80, 48, false)
	if p.active[other].parent == parent {
		t.Fatal("active allocation was lent twice")
	}
	if !p.Release(first) || p.Release(first) {
		t.Fatal("release must succeed exactly once")
	}
	smaller := p.Acquire(66, 40, false)
	if p.active[smaller].parent != parent {
		t.Fatal("compatible released allocation was replaced")
	}
	if smaller.Bounds() != image.Rect(0, 0, 66, 40) {
		t.Fatalf("view bounds: %v", smaller.Bounds())
	}
	stats := p.Stats()
	if stats.Allocations != 2 || stats.Reuses != 1 || stats.Active != 2 || stats.Free != 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	p.Release(smaller)
	p.Release(other)
	p.Clear()
	if s := p.Stats(); s.ActiveBytes != 0 || s.FreeBytes != 0 {
		t.Fatalf("clear leaked allocation bytes: %+v", s)
	}
}

func TestIdleBudgetAndModeIsolation(t *testing.T) {
	freed := 0
	p := Pool{MaxFreeBytes: BytesForSize(50, 50, false), Deallocate: func(img *ebiten.Image) { freed++; img.Deallocate() }}
	a := p.Acquire(50, 50, false)
	b := p.Acquire(50, 50, false)
	p.Release(a)
	p.Release(b)
	if s := p.Stats(); s.Free != 1 || s.FreeBytes > p.MaxFreeBytes || freed != 1 {
		t.Fatalf("idle reserve is not bounded: %+v, freed=%d", s, freed)
	}
	u := p.Acquire(50, 50, true)
	if !p.active[u].unmanaged || p.Stats().Reuses != 0 {
		t.Fatal("unmanaged request reused a managed allocation")
	}
	p.Clear()
	if p.Stats().Active != 1 {
		t.Fatal("clearing idle slots invalidated an active image")
	}
	p.Release(u)
	p.Clear()
}

func TestSizeClassesIncludeManagedBorder(t *testing.T) {
	if got := BytesForSize(63, 31, false); got != 64*32*4 {
		t.Fatalf("border pushed size class across power-of-two boundary: %d", got)
	}
	if got := BytesForSize(64, 32, false); got != 96*64*4 {
		t.Fatalf("insufficient room for managed border: %d", got)
	}
	if got := BytesForSize(64, 32, true); got != 64*32*4 {
		t.Fatalf("standalone rounding not counted: %d", got)
	}
	if got := dimensions(4096, 2048, true); got != image.Pt(4096, 2048) {
		t.Fatalf("standalone size limit exceeded by padding: %v", got)
	}
}
