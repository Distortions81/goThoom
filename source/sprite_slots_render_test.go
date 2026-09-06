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
	g := &spriteSlotRenderTestGame{t: t}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
	t.Logf("preallocation: %d slot bytes, %d additional GPU image bytes", g.reserved, g.gpuGrowth)
}

type spriteSlotRenderTestGame struct {
	t                   *testing.T
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
	for y := range 9 {
		for x := range 9 {
			if x == 0 && y == 0 {
				continue
			}
			offset := (y*32 + x) * 4
			if parentPixels[offset] != 0 || parentPixels[offset+1] != 0 || parentPixels[offset+2] != 0 || parentPixels[offset+3] != 0 {
				return fmt.Errorf("stale pixels inside new content/gutter at (%d,%d)", x, y)
			}
		}
	}
	if parentPixels[(20*32+20)*4] != 255 {
		return fmt.Errorf("test did not retain stale pixels outside sampled area")
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
	return g.checkPartialUploads()
}

func (g *spriteSlotRenderTestGame) checkPartialUploads() error {
	var pool spriteSlotPool
	pool.init()
	defer pool.clear()
	size := image.Pt(64, 64)
	slot := pool.allocate(size)
	pool.free[size] = append(pool.free[size], slot)
	dimensions := []image.Point{{48, 8}, {8, 48}, {36, 36}, {2, 33}, {50, 50}, {8, 33}}
	pixels := make([]byte, 64*64*4)
	for i, dims := range dimensions {
		// Nonzero origin and stride exercise the production row copies too.
		sheet := image.NewRGBA(image.Rect(0, 0, 80, 80))
		src := sheet.SubImage(image.Rectangle{Min: image.Pt(3, 5), Max: image.Pt(3+dims.X, 5+dims.Y)}).(*image.RGBA)
		tint := color.RGBA{R: uint8(30 + i*20), G: 60, B: 20, A: 180}
		for y := src.Rect.Min.Y; y < src.Rect.Max.Y; y++ {
			for x := src.Rect.Min.X; x < src.Rect.Max.X; x++ {
				src.SetRGBA(x, y, tint)
			}
		}
		// A recycled staging buffer need not be zero. Content is overwritten;
		// only the sampling gutter must be explicitly cleared.
		staging := acquireArtworkRGBA(image.Rectangle{Max: dims.Add(image.Pt(1, 1))})
		for n := range staging.Pix {
			staging.Pix[n] = 255
		}
		releaseArtworkRGBA(staging)
		key := testSpriteSlotPicture(uint16(1000+i), 0)
		view := pool.upload(key, src, 1<<20)
		if pool.owners[key.id()][0] != slot {
			return fmt.Errorf("partial upload failed to reuse parent at step %d", i)
		}
		slot.parent.ReadPixels(pixels)
		for y := range dims.Y + 1 {
			for x := range dims.X + 1 {
				want := color.RGBA{}
				if x < dims.X && y < dims.Y {
					want = tint
				}
				off := (y*64 + x) * 4
				got := color.RGBA{R: pixels[off], G: pixels[off+1], B: pixels[off+2], A: pixels[off+3]}
				if got != want {
					return fmt.Errorf("step %d pixel (%d,%d): got %v want %v", i, x, y, got, want)
				}
			}
		}
		if err := verifySpriteSlotFilteredView(view, src); err != nil {
			return err
		}
		pool.evict(key.id())
	}
	// Each write covers only the current content and one-texel gutter.
	const wantBytes = (49*9 + 9*49 + 37*37 + 3*34 + 51*51 + 9*34) * 4
	if pool.uploadBytes != wantBytes {
		return fmt.Errorf("submitted %d bytes, want %d", pool.uploadBytes, wantBytes)
	}
	fullBytes := uint64(len(dimensions) * 64 * 64 * 4)
	g.t.Logf("six mixed-size uploads: partial=%d bytes full-slot=%d bytes (%.1f%% less)", pool.uploadBytes, fullBytes, 100*(1-float64(pool.uploadBytes)/float64(fullBytes)))
	return g.checkDirectUploads()
}

func verifySpriteSlotFilteredView(view *ebiten.Image, src *image.RGBA) error {
	referenceParent := ebiten.NewImage(64, 64)
	defer referenceParent.Deallocate()
	full := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := range src.Bounds().Dy() {
		from := src.PixOffset(src.Bounds().Min.X, src.Bounds().Min.Y+y)
		copy(full.Pix[y*full.Stride:y*full.Stride+src.Bounds().Dx()*4], src.Pix[from:from+src.Bounds().Dx()*4])
	}
	referenceParent.WritePixels(full.Pix)
	reference := referenceParent.SubImage(image.Rectangle{Max: src.Bounds().Size()}).(*ebiten.Image)
	want, got := ebiten.NewImage(160, 160), ebiten.NewImage(160, 160)
	defer want.Deallocate()
	defer got.Deallocate()
	a, b := make([]byte, 160*160*4), make([]byte, 160*160*4)
	for _, filter := range []ebiten.Filter{ebiten.FilterNearest, ebiten.FilterLinear} {
		for _, scale := range []float64{0.5, 1.5, 2.25} {
			want.Clear()
			got.Clear()
			op := &ebiten.DrawImageOptions{Filter: filter, DisableMipmaps: true}
			op.GeoM.Scale(scale, scale)
			op.GeoM.Rotate(0.2)
			op.GeoM.Translate(30.25, 10.75)
			want.DrawImage(reference, op)
			got.DrawImage(view, op)
			want.ReadPixels(a)
			got.ReadPixels(b)
			for i := range a {
				if d := int(a[i]) - int(b[i]); d < -1 || d > 1 {
					return fmt.Errorf("filtered reused view %v filter=%v scale=%v differs at byte %d: want=%d got=%d", src.Bounds().Size(), filter, scale, i, a[i], b[i])
				}
			}
		}
	}
	return nil
}

func (g *spriteSlotRenderTestGame) checkDirectUploads() error {
	var pool spriteSlotPool
	pool.init()
	defer pool.clear()
	slot := pool.allocate(image.Pt(64, 64))
	pool.free[slot.size] = append(pool.free[slot.size], slot)
	for i, dims := range []image.Point{{40, 40}, {40, 40}, {8, 40}, {8, 40}} {
		// A packed image can have a nonzero origin. WritePixels must also permit
		// its buffer to be recycled immediately, as the artwork workers do.
		src := image.NewRGBA(image.Rectangle{Min: image.Pt(3, 5), Max: image.Pt(3+dims.X, 5+dims.Y)})
		for n := 0; n < len(src.Pix); n += 4 {
			src.Pix[n] = uint8(30 + i*30)
			src.Pix[n+2] = 70
			src.Pix[n+3] = 200
		}
		expected := image.NewRGBA(src.Bounds())
		copy(expected.Pix, src.Pix)
		key := testSpriteSlotPicture(uint16(2000+i), 0)
		previousDirect := pool.directUploads
		view := pool.upload(key, src, 1<<20)
		direct := pool.directUploads != previousDirect
		if direct != (i != 2) {
			return fmt.Errorf("upload %d: direct=%v", i, direct)
		}
		if pool.owners[key.id()][0] != slot {
			return fmt.Errorf("direct upload test did not reuse its slot")
		}
		clear(src.Pix)
		if err := verifySpriteSlotFilteredView(view, expected); err != nil {
			return err
		}
		pool.evict(key.id())
	}
	if pool.directUploads != 3 {
		return fmt.Errorf("direct uploads=%d want 3", pool.directUploads)
	}
	g.t.Logf("packed uploads: %d direct; resized sprite retained one padded write", pool.directUploads)
	return nil
}
