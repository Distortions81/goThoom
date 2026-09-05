package main

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// Run separately: Ebitengine permits only one RunGame per test process.
func TestRenderSpriteSlotReuse(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_SPRITE_SLOT_TESTS") == "" {
		t.Skip("set GOTHOOM_RENDER_SPRITE_SLOT_TESTS=1 to verify GPU slot reuse")
	}
	isolateSpriteSlots(t)
	g := &spriteSlotRenderTestGame{}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
	t.Logf("preallocation: %d slot bytes, %d additional GPU image bytes", g.reserved, g.gpuGrowth)
}

type spriteSlotRenderTestGame struct {
	done                bool
	err                 error
	reserved, gpuGrowth int64
}

func (g *spriteSlotRenderTestGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (*spriteSlotRenderTestGame) Layout(_, _ int) (int, int) { return 64, 64 }
func (g *spriteSlotRenderTestGame) Draw(_ *ebiten.Image) {
	if g.done {
		return
	}
	g.err = g.check()
	g.done = true
}

func (g *spriteSlotRenderTestGame) check() error {
	red := image.NewRGBA(image.Rect(0, 0, 30, 30))
	for y := range 30 {
		for x := range 30 {
			red.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	budget := spriteSlotBytes(spriteSlotSize(red.Bounds()))
	a := insertTestSpriteSlot(testSpriteSlotPicture(10, 0), red, budget)
	parent := spriteSlots.owners[10][0].parent
	before := newUnmanagedImage(32, 32)
	defer deallocateImage(before)
	before.DrawImage(a, nil)
	// Reuse the same size class for a smaller image with a nonzero source
	// origin and a stride larger than its view width.
	blueSheet := image.NewRGBA(image.Rect(0, 0, 40, 40))
	blueSheet.SetRGBA(5, 7, color.RGBA{B: 255, A: 255})
	blue := blueSheet.SubImage(image.Rect(5, 7, 13, 15)).(*image.RGBA)
	b := insertTestSpriteSlot(testSpriteSlotPicture(20, 0), blue, budget)
	if spriteSlots.owners[20][0].parent != parent || spriteSlots.count != 1 {
		return fmt.Errorf("reused sprite allocated a new parent")
	}
	if b.Bounds() != image.Rect(0, 0, 8, 8) {
		return fmt.Errorf("new view bounds: %v", b.Bounds())
	}
	after := newUnmanagedImage(32, 32)
	defer deallocateImage(after)
	after.DrawImage(b, nil)
	oldPixels := make([]byte, 32*32*4)
	before.ReadPixels(oldPixels)
	if oldPixels[0] != 255 || oldPixels[3] != 255 {
		return fmt.Errorf("overwrite changed an earlier queued draw: %v", oldPixels[:4])
	}
	newPixels := make([]byte, 32*32*4)
	after.ReadPixels(newPixels)
	if newPixels[2] != 255 || newPixels[3] != 255 {
		return fmt.Errorf("new draw did not receive blue pixels: %v", newPixels[:4])
	}
	parentPixels := make([]byte, 32*32*4)
	parent.ReadPixels(parentPixels)
	for offset := 4; offset < len(parentPixels); offset++ {
		if parentPixels[offset] != 0 {
			return fmt.Errorf("old pixels survived outside the new content at byte %d", offset)
		}
	}
	// Verify that warming allocates real GPU atlas space even though the
	// reserve writes only two pixels per parent, rather than full zero images.
	var beforeInfo, afterInfo ebiten.DebugInfo
	ebiten.ReadDebugInfo(&beforeInfo)
	const reserveBudget = defaultSpriteCacheMiB << 20
	spriteSlots.preallocate(2, reserveBudget)
	// Submit the queued allocation commands before reading driver statistics.
	// Startup normally gets this submission at the end of its loading frame.
	for _, slots := range spriteSlots.free {
		if len(slots) != 0 {
			slots[0].parent.SubImage(image.Rect(0, 0, 2, 1)).(*ebiten.Image).ReadPixels(make([]byte, 8))
			break
		}
	}
	ebiten.ReadDebugInfo(&afterInfo)
	g.reserved = spriteSlots.bytes - budget
	g.gpuGrowth = afterInfo.TotalGPUImageMemoryUsageInBytes - beforeInfo.TotalGPUImageMemoryUsageInBytes
	if g.reserved < reserveBudget*9/10 || g.reserved > reserveBudget {
		return fmt.Errorf("unexpected reserve size: %d", g.reserved)
	}
	if g.gpuGrowth == 0 {
		return fmt.Errorf("reserve did not allocate GPU atlas space: before=%d after=%d reserved=%d", beforeInfo.TotalGPUImageMemoryUsageInBytes, afterInfo.TotalGPUImageMemoryUsageInBytes, g.reserved)
	}
	// The preallocated parent must be used by the first compatible upload.
	size := spriteSlotSize(image.Rect(0, 0, 64, 64))
	slots := spriteSlots.free[size]
	if len(slots) == 0 {
		return fmt.Errorf("no common pose slots were preallocated")
	}
	warmParent := slots[len(slots)-1].parent
	count := spriteSlots.count
	insertTestSpriteSlot(testSpriteSlotPicture(30, 0), image.NewRGBA(image.Rect(0, 0, 64, 64)), reserveBudget)
	if spriteSlots.count != count || spriteSlots.owners[30][0].parent != warmParent {
		return fmt.Errorf("first-use sprite missed the preallocated slot")
	}
	return nil
}
