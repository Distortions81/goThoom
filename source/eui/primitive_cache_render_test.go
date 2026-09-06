package eui

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// Run alone: coverage comparisons and timings require Ebitengine's game loop.
func TestRenderPrimitiveCache(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_PRIMITIVE_CACHE") == "" {
		t.Skip("set GOTHOOM_RENDER_PRIMITIVE_CACHE=1")
	}
	uiPrimitives.clear()
	defer uiPrimitives.clear()
	g := &primitiveCacheGame{t: t}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type primitiveCacheGame struct {
	t    *testing.T
	done bool
	err  error
}

func (g *primitiveCacheGame) Layout(_, _ int) (int, int) { return 360, 220 }
func (g *primitiveCacheGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *primitiveCacheGame) Draw(_ *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	g.err = g.verify()
}

func (g *primitiveCacheGame) verify() error {
	want, got := ebiten.NewImage(360, 220), ebiten.NewImage(360, 220)
	defer want.Deallocate()
	defer got.Deallocate()
	wantPixels, gotPixels := make([]byte, 360*220*4), make([]byte, 360*220*4)
	compare := func(label string, reference, cached func(*ebiten.Image)) error {
		want.Clear()
		got.Clear()
		// Include clipping through a subimage with a nonzero origin.
		reference(want.SubImage(image.Rect(8, 8, 350, 210)).(*ebiten.Image))
		cached(got.SubImage(image.Rect(8, 8, 350, 210)).(*ebiten.Image))
		want.ReadPixels(wantPixels)
		got.ReadPixels(gotPixels)
		totalDelta := 0
		for i, a := range wantPixels {
			delta := int(a) - int(gotPixels[i])
			totalDelta += int(math.Abs(float64(delta)))
			// The vector shader uses eight AA samples. Translating its curves
			// into a small mask can put a boundary sample on the other side of
			// a floating-point tie; allow one sample at a few edge pixels.
			if delta < -33 || delta > 33 {
				return fmt.Errorf("%s differs at (%d,%d) channel %d: vector=%d cache=%d", label, (i/4)%360, i/(360*4), i%4, a, gotPixels[i])
			}
		}
		if totalDelta > 2048 {
			return fmt.Errorf("%s changed too much coverage: %d", label, totalDelta)
		}
		return nil
	}
	for _, tint := range []Color{ColorWhite, NewColor(31, 92, 115, 128)} {
		for _, scale := range []float32{1, 1.25, 1.5, 2} {
			for _, size := range []point{{8, 16}, {16, 16}, {140, 28}, {220, 60}, {5, 7}} {
				for _, radius := range []float32{0, 3, 6, 20} {
					for _, filled := range []bool{false, true} {
						r := roundRect{Size: point{size.X * scale, size.Y * scale}, Position: point{7.25, 8.75}, Fillet: radius * scale, Border: 1.5 * scale, Filled: filled, Color: tint}
						label := fmt.Sprintf("rectangle %+v", r)
						if err := compare(label, func(dst *ebiten.Image) { drawRoundRectVector(dst, &r) }, func(dst *ebiten.Image) { drawRoundRect(dst, &r) }); err != nil {
							return err
						}
					}
				}
			}
			start, mid, end := point{12.25 * scale, 16.25 * scale}, point{17.5 * scale, 21.5 * scale}, point{25.75 * scale, 11.75 * scale}
			if err := compare("checkmark", func(dst *ebiten.Image) { drawCheckmarkVector(dst, start, mid, end, 2*scale, tint) }, func(dst *ebiten.Image) { drawCheckmark(dst, start, mid, end, 2*scale, tint) }); err != nil {
				return err
			}
			if err := compare("triangle", func(dst *ebiten.Image) { drawTriangleVector(dst, start, 12*scale, tint) }, func(dst *ebiten.Image) { drawTriangle(dst, start, 12*scale, tint) }); err != nil {
				return err
			}
		}
		for _, width := range []float32{1, 2, 3, 4} {
			for _, endpoints := range [][4]float32{{12, 12, 28, 28}, {28, 12, 12, 28}, {28, 28, 12, 12}, {8.25, 9.75, 23.75, 17.25}} {
				x0, y0, x1, y1 := endpoints[0], endpoints[1], endpoints[2], endpoints[3]
				if err := compare("diagonal line", func(dst *ebiten.Image) {
					off := pixelOffset(width)
					strokeLineFn(dst, float32(roundedPixel(x0))+off, float32(roundedPixel(y0))+off, float32(roundedPixel(x1))+off, float32(roundedPixel(y1))+off, width, color.RGBA(tint), true)
				}, func(dst *ebiten.Image) { strokeLine(dst, x0, y0, x1, y1, width, color.RGBA(tint), true) }); err != nil {
					return err
				}
			}
			for _, length := range []float32{0, 1, 2, 5, 40, 200} {
				for _, vertical := range []bool{false, true} {
					for _, aa := range []bool{false, true} {
						x0, y0, x1, y1 := float32(9.25), float32(10.75), 9.25+length, float32(10.75)
						if vertical {
							x1, y1 = x0, y0+length
						}
						if err := compare(fmt.Sprintf("line width=%v length=%v vertical=%v aa=%v", width, length, vertical, aa), func(dst *ebiten.Image) {
							off := pixelOffset(width)
							strokeLineFn(dst, float32(roundedPixel(x0))+off, float32(roundedPixel(y0))+off, float32(roundedPixel(x1))+off, float32(roundedPixel(y1))+off, width, color.RGBA(tint), aa)
						}, func(dst *ebiten.Image) { strokeLine(dst, x0, y0, x1, y1, width, color.RGBA(tint), aa) }); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	uiPrimitives.clear()
	for i := range 100 {
		r := roundRect{Size: point{float32(100 + i), 32}, Fillet: 4, Border: 1, Color: NewColor(uint8(i), 100, 150, 200)}
		drawRoundRect(got, &r)
		strokeLine(got, 0, 100, float32(100+i), 100, 2, r.Color, true)
	}
	if uiPrimitives.misses != 2 {
		return fmt.Errorf("width/color changes created %d masks; want 2", uiPrimitives.misses)
	}
	g.t.Logf("Shared geometry: %d hits, %d misses, %d bytes", uiPrimitives.hits, uiPrimitives.misses, uiPrimitives.bytes)
	for radius := float32(1); radius <= 120; radius++ {
		r := roundRect{Size: point{600, 600}, Fillet: radius, Color: ColorWhite, Filled: true}
		drawRoundRect(got, &r)
	}
	if uiPrimitives.bytes > maxPrimitiveBytes || len(uiPrimitives.entries) > maxPrimitiveEntries || uiPrimitives.evictions == 0 {
		return fmt.Errorf("cache budget not enforced: bytes=%d entries=%d evictions=%d", uiPrimitives.bytes, len(uiPrimitives.entries), uiPrimitives.evictions)
	}
	uiPrimitives.clear()
	for x := 5; x < 22; x++ {
		for y := 5; y < 22; y++ {
			strokeLine(got, 10, 10, float32(10+x), float32(10+y), 1, ColorWhite, true)
		}
	}
	if len(uiPrimitives.entries) != maxPrimitiveEntries || uiPrimitives.evictions == 0 {
		return fmt.Errorf("entry budget not enforced: %d entries, %d evictions", len(uiPrimitives.entries), uiPrimitives.evictions)
	}
	uiPrimitives.clear()
	// Identical repeated controls, with a GPU readback after each group so these
	// timings include submitted work instead of only queuing draw commands.
	measure := func(cached bool) time.Duration {
		start := time.Now()
		for range 20 {
			got.Clear()
			for i := range 100 {
				r := roundRect{Position: point{float32(i%4) * 80, float32(i/4%8) * 24}, Size: point{72, 20}, Fillet: 4, Border: 1, Color: ColorWhite, Filled: i%2 == 0}
				if cached {
					drawRoundRect(got, &r)
				} else {
					drawRoundRectVector(got, &r)
				}
			}
			got.ReadPixels(gotPixels)
		}
		return time.Since(start)
	}
	measure(true) // Warm both geometry variants and the driver.
	vectorTime, cachedTime := measure(false), measure(true)
	g.t.Logf("2,000 repeated rounded controls, including readback: vector=%s cached=%s (%.2fx)", vectorTime, cachedTime, float64(vectorTime)/math.Max(1, float64(cachedTime)))
	return nil
}
