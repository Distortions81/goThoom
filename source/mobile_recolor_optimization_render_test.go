package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"os"
	"os/exec"
	"sort"
	"testing"
	"time"

	"gothoom/climg"

	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed testdata/shaders/mobile_recolor_before_zero_weight.kage
var referenceZeroWeightRecolorShader []byte

//go:embed testdata/shaders/mobile_recolor_blend_before_zero_weight.kage
var referenceZeroWeightRecolorBlendShader []byte

// GOTHOOM_RECOLOR_STUDY_IMAGES=/path/to/CL_Images adds real-artwork comparisons
// and GPU timing batches. The ordinary regression needs no installed assets.
func TestMobileRecolorOptimizationPixels(t *testing.T) {
	if os.Getenv("GOTHOOM_RECOLOR_COMPARE_CHILD") == "" {
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(exe, "-test.run=^TestMobileRecolorOptimizationPixels$", "-test.v")
		cmd.Env = append(os.Environ(), "GOTHOOM_RECOLOR_COMPARE_CHILD=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("recolor comparison: %v\n%s", err, out)
		}
		if os.Getenv("GOTHOOM_RECOLOR_STUDY_IMAGES") != "" {
			t.Logf("%s", out)
		}
		return
	}
	gs = gsdef
	gs.DenoiseImages, gs.SpriteUpscaleFilter = false, false
	gs.SpriteUpscaleMode = artworkUpscaleOff
	if path := os.Getenv("GOTHOOM_RECOLOR_STUDY_IMAGES"); path != "" {
		var err error
		clImages, err = climg.Load(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	g := &zeroWeightRecolorComparisonGame{t: t}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
	if !g.done {
		t.Fatal("comparison did not run")
	}
}

type zeroWeightRecolorComparisonGame struct {
	t    *testing.T
	done bool
	err  error
}

func (g *zeroWeightRecolorComparisonGame) Layout(int, int) (int, int) { return 768, 512 }
func (g *zeroWeightRecolorComparisonGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *zeroWeightRecolorComparisonGame) Draw(*ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	g.err = compareZeroWeightRecolorShaders(g.t)
}

func syntheticRecolorComparisonInputs() (*ebiten.Image, *ebiten.Image, *mobilePaletteShaderState, *mobilePaletteShaderState) {
	base, mask := image.NewNRGBA(image.Rect(0, 0, 64, 32)), image.NewNRGBA(image.Rect(0, 0, 64, 32))
	weights := [...]uint8{0, 0, 1, 64, 127, 128, 254, 255}
	for y := range 32 {
		for x := range 64 {
			base.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 4), G: uint8(y * 8), B: 100, A: uint8(128 + x%128)})
			// Cover every legal slot, zero-weight pixels with nonzero neighbor
			// slots, and all byte boundaries used by the packed edge weight.
			packed := uint32(x%31) | uint32((x+y)%31)<<5 | uint32((x+2*y)%31)<<10 | uint32(weights[(x+y)%len(weights)])<<15
			mask.SetNRGBA(x, y, color.NRGBA{R: uint8(packed), G: uint8(packed >> 8), B: uint8(packed >> 16), A: 255})
		}
	}
	previous := testMobilePalette(makeMobileKey(1, 0, []byte{1}), 0, 0, 0, 0)
	current := testMobilePalette(makeMobileKey(1, 0, []byte{2}), 0, 0, 0, 0)
	for i := range maxColors {
		previous.palette[i*4] = float32(i%5-2) * 0.08
		previous.palette[i*4+1] = float32(i%7-3) * 0.06
		previous.palette[i*4+2] = float32(i%3-1) * 0.1
		previous.palette[i*4+3] = float32(i%3-1) * 0.03
		for channel := range 4 {
			current.palette[i*4+channel] = -previous.palette[i*4+channel]
		}
	}
	return ebiten.NewImageFromImage(base), ebiten.NewImageFromImage(mask), previous, current
}

func compareZeroWeightRecolorShaders(t *testing.T) error {
	var shaders [2][2]*ebiten.Shader
	sources := [2][2][]byte{
		{referenceZeroWeightRecolorShader, referenceZeroWeightRecolorBlendShader},
		{mobileRecolorShaderSource, mobileRecolorBlendShaderSource},
	}
	defer func() {
		for _, pair := range shaders {
			for _, shader := range pair {
				if shader != nil {
					shader.Deallocate()
				}
			}
		}
	}()
	for i, pair := range sources {
		for j, source := range pair {
			var err error
			shaders[i][j], err = ebiten.NewShader(source)
			if err != nil {
				return err
			}
		}
	}
	base, mask, palette, other := syntheticRecolorComparisonInputs()
	defer base.Deallocate()
	defer mask.Deallocate()
	if clImages != nil {
		mobileRecolorShader, mobileRecolorBlendShader = shaders[1][0], shaders[1][1]
		colors := []byte{0x5e, 0x89, 0x08, 0x33, 0x5e, 0xbd, 0x5a, 0xae, 0x8c, 0x8c, 0x8d, 0x2e, 0x83, 0x74, 0x60, 0xaa, 0x2b, 0x81, 0x3d, 0x86}
		var ok bool
		base, mask, palette, ok = loadGPURecoloredMobileFrame(451, 0, colors)
		if !ok {
			return fmt.Errorf("real mobile 451 unavailable")
		}
		otherColors := bytes.Clone(colors)
		otherColors[0] ^= 3
		other = mobilePaletteState(451, otherColors)
	}
	width, height := base.Bounds().Dx(), base.Bounds().Dy()
	dst := ebiten.NewImage(768, 512)
	defer dst.Deallocate()
	readback := dst.SubImage(image.Rect(0, 0, 1, 1)).(*ebiten.Image)
	pixel := make([]byte, 4)
	for _, blended := range []bool{false, true} {
		for _, linear := range []bool{false, true} {
			options := frameBlendDrawOptions{ScaleX: 2, ScaleY: 2, Red: 1, Green: 1, Blue: 1, Alpha: 1, Fade: 0.4, Linear: linear}
			draw := func(pair [2]*ebiten.Shader, tiled bool) {
				mobileRecolorShader, mobileRecolorBlendShader = pair[0], pair[1]
				n := 1
				if tiled {
					n = (768 / (2 * width)) * (512 / (2 * height))
				}
				for i := range n {
					options.Left = float64((i%(768/(2*width)))*2*width) + 0.37
					options.Top = float64((i/(768/(2*width)))*2*height) + 0.61
					if blended {
						drawRecoloredMobileFrameBlend(dst, base, mask, base, mask, palette, other, options)
					} else {
						drawRecoloredMobile(dst, base, mask, palette, options)
					}
				}
			}
			for _, flash := range [][4]float32{{}, {1, 0.2, 0.4, 0.65}} {
				options.FlashColor = flash
				var pixels [2][]byte
				for i, pair := range shaders {
					dst.Clear()
					draw(pair, false)
					pixels[i] = make([]byte, 768*512*4)
					dst.ReadPixels(pixels[i])
				}
				for i, a := range pixels[0] {
					if d := int(a) - int(pixels[1][i]); d < -1 || d > 1 {
						return fmt.Errorf("blend=%v linear=%v flash=%v byte=%d: before=%d after=%d", blended, linear, flash, i, a, pixels[1][i])
					}
				}
			}
			if clImages == nil {
				continue
			}
			options.FlashColor = [4]float32{}
			// Batch identical sprite grids, then synchronize once so readback
			// latency does not dominate the comparison of actual shader work.
			measure := func(pair [2]*ebiten.Shader) time.Duration {
				start := time.Now()
				for range 30 {
					draw(pair, true)
				}
				readback.ReadPixels(pixel)
				return time.Since(start) / 30
			}
			measure(shaders[0])
			measure(shaders[1])
			var before, after []time.Duration
			for i := range 9 {
				if i%2 == 0 {
					before = append(before, measure(shaders[0]))
					after = append(after, measure(shaders[1]))
				} else {
					after = append(after, measure(shaders[1]))
					before = append(before, measure(shaders[0]))
				}
			}
			sort.Slice(before, func(i, j int) bool { return before[i] < before[j] })
			sort.Slice(after, func(i, j int) bool { return after[i] < after[j] })
			t.Logf("real sprite 451, 768x512 grid, blend=%v linear=%v: before=%v after=%v change=%.1f%%", blended, linear, before[4], after[4], 100*(float64(after[4])/float64(before[4])-1))
		}
	}
	return nil
}
