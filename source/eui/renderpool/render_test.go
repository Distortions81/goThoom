package renderpool

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestRenderRecycledManagedTarget(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_POOL_TEST") == "" {
		t.Skip("set GOTHOOM_RENDER_POOL_TEST=1 for rendered recycling checks")
	}
	g := &recycleGame{pool: Pool{MaxFreeBytes: 1 << 20}}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
	s := g.pool.Stats()
	if s.Allocations != 1 || s.Reuses != 3 {
		t.Fatalf("repaint replaced allocation: %+v", s)
	}
	g.pool.Release(g.current)
	g.pool.Clear()
}

type recycleGame struct {
	pool           Pool
	current, glyph *ebiten.Image
	frames         int
	err            error
}

func (g *recycleGame) Update() error {
	if g.err != nil || g.frames >= 121 {
		return ebiten.Termination
	}
	return nil
}
func (*recycleGame) Layout(_, _ int) (int, int) { return 128, 96 }
func (g *recycleGame) Draw(screen *ebiten.Image) {
	if g.frames == 0 {
		pixels := image.NewRGBA(image.Rect(0, 0, 8, 8))
		for y := range 8 {
			for x := range 8 {
				pixels.SetRGBA(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
			}
		}
		g.glyph = ebiten.NewImageFromImage(pixels)
		g.current = g.pool.Acquire(100, 60, false)
		g.current.Fill(color.RGBA{R: 255, A: 255})
		g.current.DrawImage(g.glyph, nil)
	}
	if g.frames > 0 && g.frames%40 == 0 {
		g.err = g.recycle()
	}
	screen.DrawImage(g.current, nil)
	g.frames++
}

func (g *recycleGame) recycle() error {
	parent := g.pool.active[g.current].parent
	// Queue a source read before recycling. Give Ebitengine 40 intervening
	// frames between repaints so source-atlas migration can also occur.
	previous := ebiten.NewImageWithOptions(image.Rect(0, 0, 100, 60), &ebiten.NewImageOptions{Unmanaged: true})
	defer previous.Deallocate()
	previous.DrawImage(g.current, nil)
	wasRed := g.frames == 40
	g.pool.Release(g.current)
	g.current = g.pool.Acquire(80, 50, false)
	if g.pool.active[g.current].parent != parent {
		return fmt.Errorf("parent allocation changed")
	}
	g.current.Fill(color.RGBA{G: 255, A: 255})
	g.current.DrawImage(g.glyph, nil)
	old := make([]byte, 100*60*4)
	previous.ReadPixels(old)
	pos := (10*100 + 10) * 4
	if old[pos+3] != 255 || (wasRed && old[pos] != 255) || (!wasRed && old[pos+1] != 255) {
		return fmt.Errorf("queued draw changed after reuse: %v", old[pos:pos+4])
	}
	bounds := parent.Bounds()
	all := make([]byte, bounds.Dx()*bounds.Dy()*4)
	parent.ReadPixels(all)
	pos = (10*bounds.Dx() + 10) * 4
	if all[pos] != 0 || all[pos+1] != 255 || all[pos+3] != 255 {
		return fmt.Errorf("new content not rendered correctly: %v", all[pos:pos+4])
	}
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			if x < 80 && y < 50 {
				continue
			}
			i := (y*bounds.Dx() + x) * 4
			if all[i+3] != 0 {
				return fmt.Errorf("stale pixels outside smaller view at %d,%d", x, y)
			}
		}
	}
	return nil
}
