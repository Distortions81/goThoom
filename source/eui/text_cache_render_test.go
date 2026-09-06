package eui

import (
	"bytes"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/goregular"
	"image"
	"image/color"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRenderUITextCache(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_TEXT_CACHE") == "" {
		t.Skip("set GOTHOOM_RENDER_TEXT_CACHE=1")
	}
	if err := EnsureFontSource(goregular.TTF); err != nil {
		t.Fatal(err)
	}
	plainUIText.clear()
	defer plainUIText.clear()
	game := &textCacheGame{t: t}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

type textCacheGame struct {
	t    *testing.T
	done bool
	err  error
}

func (g *textCacheGame) Layout(_, _ int) (int, int) { return 360, 220 }
func (g *textCacheGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *textCacheGame) Draw(_ *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	g.err = g.verify()
}
func (g *textCacheGame) verify() error {
	before, after := ebiten.NewImage(360, 220), ebiten.NewImage(360, 220)
	defer before.Deallocate()
	defer after.Deallocate()
	a, b := make([]byte, 360*220*4), make([]byte, 360*220*4)
	for _, size := range []float32{12, 17.5, 28} {
		for _, value := range []string{"Player status\nAdditional information", "Ágj Å\u0301 fffi", "مرحبا بالعالم", strings.Repeat("W", 100), strings.Repeat("row\n", 24), "", " \n "} {
			for _, offset := range []float64{0, 0.25, 0.5, 0.75, -10.25} {
				for _, bg := range []color.Color{color.Transparent, color.NRGBA{R: 31, G: 72, B: 110, A: 255}} {
					for _, tint := range []color.Color{color.White, color.NRGBA{R: 220, G: 130, B: 70, A: 128}} {
						op := &text.DrawOptions{LayoutOptions: text.LayoutOptions{LineSpacing: float64(size) * 1.2}}
						op.Filter = ebiten.FilterNearest
						op.DisableMipmaps = true
						op.GeoM.Translate(10+offset, 20+offset)
						op.ColorScale.ScaleWithColor(tint)
						// Check both cache miss and hit, with a clipped nonzero destination.
						for range 2 {
							before.Fill(bg)
							after.Fill(bg)
							clip := image.Rect(12, 14, 340, 210)
							text.Draw(before.SubImage(clip).(*ebiten.Image), value, textFace(size), op)
							drawCachedUIText(after.SubImage(clip).(*ebiten.Image), value, textFace(size), op)
							before.ReadPixels(a)
							after.ReadPixels(b)
							for i := range a {
								if d := int(a[i]) - int(b[i]); d < -2 || d > 2 {
									return fmt.Errorf("text %q size=%v offset=%v differs at %d: %d != %d", value, size, offset, i, a[i], b[i])
								}
							}
						}
					}
				}
			}
		}
	}
	plainUIText.clear()
	op := &text.DrawOptions{}
	op.Filter = ebiten.FilterNearest
	op.DisableMipmaps = true
	for i := 0; i < maxUITextEntries+10; i++ {
		drawCachedUIText(after, fmt.Sprintf("line %d", i), textFace(14), op)
	}
	if len(plainUIText.entries) > maxUITextEntries || plainUIText.bytes > maxUITextBytes {
		return fmt.Errorf("text cache exceeded budget")
	}
	misses := plainUIText.misses
	drawCachedUIText(after, "line 265", textFace(14), op)
	if plainUIText.misses != misses {
		return fmt.Errorf("recent text was evicted")
	}
	plainUIText.clear()
	op.LineSpacing = 28
	for i := range 24 {
		value := fmt.Sprintf("%d", i) + strings.Repeat(strings.Repeat("W", 30)+"\n", 7)
		drawCachedUIText(after, value, textFace(24), op)
	}
	if len(plainUIText.entries) == 0 || len(plainUIText.entries) >= 24 || plainUIText.bytes > maxUITextBytes {
		return fmt.Errorf("byte budget not exercised/enforced: entries=%d bytes=%d", len(plainUIText.entries), plainUIText.bytes)
	}
	// Exercise the production item path: NewText supplies a default face,
	// so checking only item.Face == nil would silently bypass this cache.
	plainUIText.clear()
	item, _ := NewText()
	item.Text = "Unchanged row"
	for range 2 {
		item.drawItemInternal(point{}, point{}, point{360, 220}, rect{X1: 360, Y1: 220}, after)
	}
	if plainUIText.hits != 1 || plainUIText.misses != 1 {
		return fmt.Errorf("plain item did not reuse text: hits=%d misses=%d", plainUIText.hits, plainUIText.misses)
	}
	item.Text = "Changed row"
	item.drawItemInternal(point{}, point{}, point{360, 220}, rect{X1: 360, Y1: 220}, after)
	if plainUIText.misses != 2 {
		return fmt.Errorf("changed text reused stale pixels")
	}
	plainUIText.clear()
	for _, ttf := range [][]byte{goregular.TTF, gobold.TTF, goitalic.TTF} {
		source, err := text.NewGoTextFaceSource(bytes.NewReader(ttf))
		if err != nil {
			return err
		}
		for _, size := range []float64{14, 21} {
			for range 2 {
				// Text windows recreate equivalent face objects during refresh.
				face := &text.GoTextFace{Source: source, Size: size}
				before.Clear()
				after.Clear()
				text.Draw(before, "Styled text Ágj", face, op)
				drawCachedUIText(after, "Styled text Ágj", face, op)
				before.ReadPixels(a)
				after.ReadPixels(b)
				for i := range a {
					if d := int(a[i]) - int(b[i]); d < -2 || d > 2 {
						return fmt.Errorf("font variant differs at %d", i)
					}
				}
			}
		}
	}
	if plainUIText.hits != 6 || plainUIText.misses != 6 {
		return fmt.Errorf("font variants or equivalent faces were not keyed correctly")
	}
	// This measures repeated text submission with readback, not gameplay FPS.
	measure := func(cached bool) time.Duration {
		start := time.Now()
		for range 30 {
			after.Clear()
			for i := range 60 {
				op.GeoM.Reset()
				op.GeoM.Translate(float64(i%3*110), float64(i/3*10))
				if cached {
					drawCachedUIText(after, "Player status text", textFace(14), op)
				} else {
					text.Draw(after, "Player status text", textFace(14), op)
				}
			}
			after.ReadPixels(b)
		}
		return time.Since(start)
	}
	measure(false)
	measure(true)
	direct, cached := measure(false), measure(true)
	g.t.Logf("1800 repeated text draws with readback: direct=%s cached=%s", direct, cached)
	return nil
}
