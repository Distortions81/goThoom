package eui

import (
	"bytes"
	"fmt"
	"image/color"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/goregular"
)

func TestRenderColorTextCache(t *testing.T) {
	path := os.Getenv("EUI_COLOR_FONT")
	if path == "" {
		t.Skip("set EUI_COLOR_FONT to a color emoji font; run alone")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	source, err := text.NewGoTextFaceSource(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if err := EnsureFontSource(goregular.TTF); err != nil {
		t.Fatal(err)
	}
	emoji := text.NewLimitedFace(&text.GoTextFace{Source: source, Size: 24})
	emoji.AddUnicodeRange(0x80, 0x10ffff)
	face, err := text.NewMultiFace(emoji, textFace(24))
	if err != nil {
		t.Fatal(err)
	}
	plainUIText.clear()
	defer plainUIText.clear()
	g := &colorTextCacheGame{face: face}
	ebiten.SetWindowVisible(false)
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type colorTextCacheGame struct {
	face text.Face
	done bool
	err  error
}

func (*colorTextCacheGame) Layout(int, int) (int, int) { return 360, 120 }
func (g *colorTextCacheGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *colorTextCacheGame) Draw(*ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	g.err = g.verify()
}
func (g *colorTextCacheGame) verify() error {
	a, b := ebiten.NewImage(360, 120), ebiten.NewImage(360, 120)
	defer a.Deallocate()
	defer b.Deallocate()
	ap, bp := make([]byte, 360*120*4), make([]byte, 360*120*4)
	for _, value := range []string{"Text 😄 ❤️ 🚀", "😄\n❤️"} {
		for _, tint := range []color.Color{color.Black, color.White, color.NRGBA{R: 150, G: 50, B: 90, A: 128}} {
			op := &text.DrawOptions{LayoutOptions: text.LayoutOptions{LineSpacing: 34}}
			op.GeoM.Translate(5.25, 5.5)
			op.DisableMipmaps = true
			op.ColorScale.ScaleWithColor(tint)
			for range 2 {
				a.Clear()
				b.Clear()
				text.Draw(a, value, g.face, op)
				drawCachedUIText(b, value, g.face, op)
				a.ReadPixels(ap)
				b.ReadPixels(bp)
				for i := range ap {
					if d := int(ap[i]) - int(bp[i]); d < -2 || d > 2 {
						return fmt.Errorf("cached color text %q differs at %d: %d vs %d", value, i, ap[i], bp[i])
					}
				}
			}
		}
	}
	if plainUIText.hits < 6 {
		return fmt.Errorf("composed text did not reuse cached images")
	}
	return nil
}
