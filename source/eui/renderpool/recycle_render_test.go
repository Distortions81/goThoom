package renderpool

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// Run alone: pixel checks need an active Ebitengine graphics context.
func TestRenderRecycledTargets(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_RECYCLED_TARGETS") == "" {
		t.Skip("set GOTHOOM_RENDER_RECYCLED_TARGETS=1")
	}
	g := &recycledTargetsGame{}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type recycledTargetsGame struct {
	done bool
	err  error
}

func (*recycledTargetsGame) Layout(int, int) (int, int) { return 64, 32 }
func (g *recycledTargetsGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *recycledTargetsGame) Draw(*ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	for _, unmanaged := range []bool{false, true} {
		if err := verifyRecycledTargetDraws(unmanaged); err != nil {
			g.err = err
			return
		}
	}
}

func verifyRecycledTargetDraws(unmanaged bool) error {
	p := Pool{MaxFreeBytes: 1 << 20}
	defer p.Clear()
	dst := ebiten.NewImage(64, 32)
	defer dst.Deallocate()
	red, blue := color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}
	for i, tint := range []color.RGBA{red, blue} {
		view := p.Acquire(24-i*4, 24-i*4, unmanaged)
		// The child has a nonzero origin; recycle it before the queued source
		// draw executes, then recycle and reuse its backing allocation too.
		clip := view.RecyclableSubImage(image.Rect(4, 4, 16, 16))
		clip.Fill(tint)
		clip.Recycle()
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(i*32), 0)
		dst.DrawImage(view, op)
		p.Release(view)
	}
	if s := p.Stats(); s.Allocations != 1 || s.Reuses != 1 || s.Active != 0 {
		return fmt.Errorf("unmanaged=%v: expected one reused allocation: %+v", unmanaged, s)
	}
	pixels := make([]byte, 64*32*4)
	dst.ReadPixels(pixels)
	for y := range 32 {
		for x := range 64 {
			want := color.RGBA{}
			if y >= 4 && y < 16 && x%32 >= 4 && x%32 < 16 {
				want = red
				if x >= 32 {
					want = blue
				}
			}
			i := (y*64 + x) * 4
			got := color.RGBA{R: pixels[i], G: pixels[i+1], B: pixels[i+2], A: pixels[i+3]}
			if got != want {
				return fmt.Errorf("unmanaged=%v: recycled draw at (%d,%d) = %v, want %v", unmanaged, x, y, got, want)
			}
		}
	}
	return nil
}
