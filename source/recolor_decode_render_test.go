package main

import (
	_ "embed"
	"fmt"
	"image"
	"os"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// Frozen floating-point implementations provide an independent pixel reference.
//
//go:embed testdata/shaders/mobile_recolor_float.kage
var referenceRecolorFloat []byte

//go:embed testdata/shaders/mobile_recolor_blend_float.kage
var referenceRecolorBlendFloat []byte

// Run alone: compares real shader output, including linear sampling and atlas offsets.
func TestRenderRecolorDecode(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_RECOLOR_DECODE") == "" {
		t.Skip("set GOTHOOM_RENDER_RECOLOR_DECODE=1")
	}
	if err := ReloadSpriteUpscaleShader(); err != nil {
		t.Fatal(err)
	}
	ebiten.SetWindowVisible(false)
	g := &recolorDecodeGame{t: t}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type recolorDecodeGame struct {
	t    *testing.T
	done bool
	err  error
}

func (*recolorDecodeGame) Layout(int, int) (int, int) { return 1280, 400 }
func (g *recolorDecodeGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *recolorDecodeGame) Draw(*ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	g.err = g.verify()
}
func (g *recolorDecodeGame) verify() error {
	oldSingle, err := ebiten.NewShader(referenceRecolorFloat)
	if err != nil {
		return err
	}
	defer oldSingle.Deallocate()
	oldBlend, err := ebiten.NewShader(referenceRecolorBlendFloat)
	if err != nil {
		return err
	}
	defer oldBlend.Deallocate()
	newSingle, newBlend := mobileRecolorShader, mobileRecolorBlendShader
	defer func() { mobileRecolorShader, mobileRecolorBlendShader = newSingle, newBlend }()
	mobilePaletteBlendCache = make(map[mobilePalettePairKey]*mobilePaletteBlendShaderState)
	const width, height = 31 * 31, 256
	base := image.NewRGBA(image.Rect(0, 0, width+16, height+16))
	mask := image.NewRGBA(base.Bounds())
	// Every weight byte and every pair of center/first slots; the second slot
	// cycles independently with the weight. All bytes cross their bit boundaries.
	for y := range height {
		for x := range width {
			center, first, second := x%31, x/31, (x%31+x/31+y)%31
			packed := uint32(center) | uint32(first)<<5 | uint32(second)<<10 | uint32(y)<<15
			i := base.PixOffset(x+5, y+3)
			copy(base.Pix[i:i+4], []byte{40, 60, 80, 255})
			copy(mask.Pix[i:i+4], []byte{byte(packed), byte(packed >> 8), byte(packed >> 16), 255})
		}
	}
	baseBacking, maskBacking := ebiten.NewImageFromImage(base), ebiten.NewImageFromImage(mask)
	defer baseBacking.Deallocate()
	defer maskBacking.Deallocate()
	prevBounds, curBounds := image.Rect(5, 3, width+5, height+3), image.Rect(7, 5, width+3, height+1)
	prev, prevMask := baseBacking.SubImage(prevBounds).(*ebiten.Image), maskBacking.SubImage(prevBounds).(*ebiten.Image)
	cur, curMask := baseBacking.SubImage(curBounds).(*ebiten.Image), maskBacking.SubImage(curBounds).(*ebiten.Image)
	p := testMobilePalette(makeMobileKey(60000, 0, []byte{1}), 0, 0, 0, 0)
	c := testMobilePalette(makeMobileKey(60000, 0, []byte{2}), 0, 0, 0, 0)
	for i := range p.r {
		p.r[i], p.g[i], p.b[i] = float32(i-15)/32, float32((i*7)%30)/32, float32(i)/64
		c.r[i], c.g[i], c.b[i] = float32(i)/64, -float32(i)/64, float32((i*13)%30)/32
	}
	dst := ebiten.NewImage(1280, 400)
	defer dst.Deallocate()
	want, got := make([]byte, 1280*400*4), make([]byte, 1280*400*4)
	for _, blend := range []bool{false, true} {
		for _, linear := range []bool{false, true} {
			op := frameBlendDrawOptions{ScaleX: 1.25, ScaleY: 1.25, Left: 3.25, Top: 2.75, Red: 1, Green: 0.8, Blue: 0.9, Alpha: 0.85, Fade: 0.375, Linear: linear, FlashColor: [4]float32{0.2, 0.4, 0.7, 0.25}}
			draw := func(reference bool, pixels []byte) {
				mobileRecolorShader, mobileRecolorBlendShader = newSingle, newBlend
				if reference {
					mobileRecolorShader, mobileRecolorBlendShader = oldSingle, oldBlend
				}
				dst.Clear()
				if blend {
					drawRecoloredMobileFrameBlend(dst, prev, prevMask, cur, curMask, p, c, op)
				} else {
					drawRecoloredMobile(dst, prev, prevMask, p, op)
				}
				dst.ReadPixels(pixels)
			}
			draw(true, want)
			draw(false, got)
			for i := range want {
				delta := int(got[i]) - int(want[i])
				if delta < -1 || delta > 1 {
					return fmt.Errorf("blend=%v linear=%v pixel (%d,%d) channel %d: integer=%d float=%d", blend, linear, (i/4)%1280, i/(1280*4), i%4, got[i], want[i])
				}
			}
			if os.Getenv("GOTHOOM_BENCH_RECOLOR_SHADERS") != "" {
				var times [2]time.Duration
				// Alternate order, after both shaders have compiled and warmed.
				for round := range 100 {
					for pass := range 2 {
						which := (round + pass) % 2
						start := time.Now()
						draw(which == 0, got)
						times[which] += time.Since(start)
					}
				}
				g.t.Logf("blend=%v linear=%v, 100 draws including readback: float=%s integer=%s", blend, linear, times[0], times[1])
			}
		}
	}
	return nil
}
