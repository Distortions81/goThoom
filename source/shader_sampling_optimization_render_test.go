package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"math"
	"os"
	"os/exec"
	"sort"
	"testing"
	"time"

	"gothoom/climg"

	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed testdata/shaders/frame_blend_before_sampling_optimization.kage
var referenceFrameBlendSampling []byte

//go:embed testdata/shaders/mobile_recolor_before_sampling_optimization.kage
var referenceRecolorSampling []byte

//go:embed testdata/shaders/mobile_recolor_blend_before_sampling_optimization.kage
var referenceRecolorBlendSampling []byte

func TestShaderSamplingOptimizationPixels(t *testing.T) {
	if os.Getenv("GOTHOOM_SAMPLING_CHILD") == "" {
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(exe, "-test.run=^TestShaderSamplingOptimizationPixels$", "-test.v")
		cmd.Env = append(os.Environ(), "GOTHOOM_SAMPLING_CHILD=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("shader sampling comparison: %v\n%s", err, out)
		}
		if os.Getenv("GOTHOOM_SAMPLING_TIMING") != "" {
			t.Logf("%s", out)
		}
		return
	}
	gs = gsdef
	gs.DenoiseImages, gs.SpriteUpscaleFilter = false, false
	gs.SpriteUpscaleMode = artworkUpscaleOff
	game := &shaderSamplingComparisonGame{t: t}
	ebiten.SetWindowVisible(false)
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
	if !game.done {
		t.Fatal("comparison did not run")
	}
}

type shaderSamplingComparisonGame struct {
	t    *testing.T
	done bool
	err  error
}

func (*shaderSamplingComparisonGame) Layout(int, int) (int, int) { return 768, 512 }
func (g *shaderSamplingComparisonGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *shaderSamplingComparisonGame) Draw(*ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	g.err = g.compare()
}

type samplingInputs struct {
	previous, current         *ebiten.Image
	previousMask, currentMask *ebiten.Image
	previousPal, currentPal   *mobilePaletteShaderState
}

// drawSamplingReference retains the original four-tap and two-frame submission,
// independently of the optimized Go filtering and unchanged-pose decisions.
func drawSamplingReference(dst *ebiten.Image, shader *ebiten.Shader, kind int, in samplingInputs, options frameBlendDrawOptions) {
	previousBounds, currentBounds := in.previous.Bounds(), in.current.Bounds()
	width, height := previousBounds.Dx(), previousBounds.Dy()
	if kind != 1 {
		width, height = max(width, currentBounds.Dx()), max(height, currentBounds.Dy())
	}
	previousOffsetX, previousOffsetY := (width-previousBounds.Dx())/2, (height-previousBounds.Dy())/2
	currentOffsetX, currentOffsetY := (width-currentBounds.Dx())/2, (height-currentBounds.Dy())/2
	linear := float32(0)
	if options.Linear {
		linear = 1
	}
	custom := [4]float32{options.Fade, float32(previousOffsetX - currentOffsetX), float32(previousOffsetY - currentOffsetY), linear}
	if kind == 1 {
		custom = [4]float32{linear}
	}
	bounds := image.Rect(previousBounds.Min.X-previousOffsetX, previousBounds.Min.Y-previousOffsetY, previousBounds.Min.X-previousOffsetX+width, previousBounds.Min.Y-previousOffsetY+height)
	vertices := mobileShaderVertices(bounds, options, custom)
	var op *ebiten.DrawTrianglesShaderOptions
	switch kind {
	case 0:
		op = &ebiten.DrawTrianglesShaderOptions{}
		op.Images[0], op.Images[1] = in.previous, in.current
		if options.FlashColor[3] > 0 {
			op.Uniforms = map[string]any{"FlashColor": options.FlashColor[:]}
		}
	case 1:
		op = &in.previousPal.op
		op.Images[0], op.Images[1] = in.previous, in.previousMask
		in.previousPal.flash = options.FlashColor
	case 2:
		state := mobilePaletteBlendState(in.previousPal, in.currentPal)
		op = &state.op
		op.Images[0], op.Images[1], op.Images[2], op.Images[3] = in.previous, in.previousMask, in.current, in.currentMask
		state.flash = options.FlashColor
	}
	dst.DrawTrianglesShader32(vertices[:], frameBlendIndices, shader, op)
}

func (g *shaderSamplingComparisonGame) compare() error {
	sources := [2][3][]byte{
		{referenceFrameBlendSampling, referenceRecolorSampling, referenceRecolorBlendSampling},
		{frameBlendShaderSource, mobileRecolorShaderSource, mobileRecolorBlendShaderSource},
	}
	var shaders [2][3]*ebiten.Shader
	for version, sourceSet := range sources {
		for kind, source := range sourceSet {
			shader, err := ebiten.NewShader(source)
			if err != nil {
				return err
			}
			defer shader.Deallocate()
			shaders[version][kind] = shader
		}
	}
	base, mask, palette, other := syntheticRecolorComparisonInputs()
	defer base.Deallocate()
	defer mask.Deallocate()
	baseBacking, maskBacking := ebiten.NewImage(80, 48), ebiten.NewImage(80, 48)
	defer baseBacking.Deallocate()
	defer maskBacking.Deallocate()
	var upload ebiten.DrawImageOptions
	upload.GeoM.Translate(5, 3)
	baseBacking.DrawImage(base, &upload)
	maskBacking.DrawImage(mask, &upload)
	previous := baseBacking.SubImage(image.Rect(3, 1, 71, 37)).(*ebiten.Image)
	previousMask := maskBacking.SubImage(previous.Bounds()).(*ebiten.Image)
	cropped := baseBacking.SubImage(image.Rect(7, 5, 67, 33)).(*ebiten.Image)
	croppedMask := maskBacking.SubImage(cropped.Bounds()).(*ebiten.Image)
	inputs := []samplingInputs{
		{previous, previous, previousMask, previousMask, palette, palette},
		{previous, cropped, previousMask, croppedMask, palette, other},
		{previous, previous, previousMask, previousMask, palette, other},
		{previous, previous, previousMask, croppedMask, palette, palette},
	}
	draw := func(dst *ebiten.Image, version, kind int, in samplingInputs, op frameBlendDrawOptions) {
		frameBlendShader, mobileRecolorShader, mobileRecolorBlendShader = shaders[version][0], shaders[version][1], shaders[version][2]
		if version == 0 {
			drawSamplingReference(dst, shaders[0][kind], kind, in, op)
			return
		}
		switch kind {
		case 0:
			drawFrameBlend(dst, in.previous, in.current, op)
		case 1:
			drawRecoloredMobile(dst, in.previous, in.previousMask, in.previousPal, op)
		case 2:
			drawRecoloredMobileFrameBlend(dst, in.previous, in.previousMask, in.current, in.currentMask, in.previousPal, in.currentPal, op)
		}
	}
	dst := ebiten.NewImage(256, 192)
	defer dst.Deallocate()
	want, got := make([]byte, 256*192*4), make([]byte, 256*192*4)
	checked := 0
	frameBlendShader, mobileRecolorShader, mobileRecolorBlendShader = shaders[1][0], shaders[1][1], shaders[1][2]
	empty := base.SubImage(image.Rectangle{}).(*ebiten.Image)
	if drawRecoloredMobileFrameBlend(dst, empty, empty, empty, empty, palette, palette, frameBlendDrawOptions{}) {
		return fmt.Errorf("empty unchanged frames reported a draw")
	}
	for kind := range 3 {
		for inputIndex, in := range inputs {
			for _, linear := range []bool{false, true} {
				for _, scale := range [][2]float64{{0.5, 0.5}, {0.75, 1.25}, {1, 1}, {1.25, 1.25}, {2, 2}, {3, 3}, {-1, 1}, {-2, -2}, {math.Nextafter(1, 0), 1}, {1, math.Nextafter(2, 3)}} {
					for _, offset := range [][2]float64{{80, 8}, {80.37, 8.61}, {80, 8.37}, {160, 120}, {0, 0}, {-1, -1}, {math.Nextafter(80, 0), 8}, {80, math.Nextafter(8, 9)}} {
						for _, flash := range [][4]float32{{}, {0.2, 0.4, 0.7, 0.65}} {
							op := frameBlendDrawOptions{Left: offset[0], Top: offset[1], ScaleX: scale[0], ScaleY: scale[1], Fade: 0.375, Red: 0.8, Green: 0.9, Blue: 1, Alpha: 0.73, Linear: linear, FlashColor: flash}
							dst.Clear()
							draw(dst, 0, kind, in, op)
							dst.ReadPixels(want)
							dst.Clear()
							draw(dst, 1, kind, in, op)
							dst.ReadPixels(got)
							if !bytes.Equal(want, got) {
								for i := range want {
									if want[i] != got[i] {
										return fmt.Errorf("kind=%d input=%d linear=%v scale=%v offset=%v flash=%v byte=%d: before=%d after=%d", kind, inputIndex, linear, scale, offset, flash, i, want[i], got[i])
									}
								}
							}
							checked++
						}
					}
				}
			}
		}
	}
	g.t.Logf("exact pixels match across %d sampling/pose/palette/flash cases", checked)
	// The unchanged-frame path also removes mix(a, a, fade). Check multiple
	// animation positions independently of the larger geometry matrix so any
	// floating-point rounding difference remains visible in the final pixels.
	fadeChecked := 0
	for _, fade := range []float32{0, 0.1, 0.4, 0.5, 0.99, 1} {
		for _, transform := range []struct {
			scale, offset float64
		}{{1, 0}, {1, 0.37}, {2, 0}, {1.25, 0.37}} {
			for _, flash := range [][4]float32{{}, {0.2, 0.4, 0.7, 0.65}} {
				op := frameBlendDrawOptions{Left: 80 + transform.offset, Top: 8 + transform.offset, ScaleX: transform.scale, ScaleY: transform.scale, Fade: fade, Red: 0.8, Green: 0.9, Blue: 1, Alpha: 0.73, Linear: true, FlashColor: flash}
				dst.Clear()
				draw(dst, 0, 2, inputs[0], op)
				dst.ReadPixels(want)
				dst.Clear()
				draw(dst, 1, 2, inputs[0], op)
				dst.ReadPixels(got)
				if !bytes.Equal(want, got) {
					for i := range want {
						if want[i] != got[i] {
							return fmt.Errorf("unchanged frames fade=%g scale=%g offset=%g flash=%v byte=%d: before=%d after=%d", fade, transform.scale, transform.offset, flash, i, want[i], got[i])
						}
					}
				}
				fadeChecked++
			}
		}
	}
	g.t.Logf("exact pixels match across %d unchanged-frame animation cases", fadeChecked)
	if os.Getenv("GOTHOOM_SAMPLING_TIMING") == "" {
		return nil
	}
	if path := os.Getenv("GOTHOOM_SAMPLING_IMAGES"); path != "" {
		var err error
		clImages, err = climg.Load(path)
		if err != nil {
			return err
		}
		colors := []byte{0x5e, 0x89, 0x08, 0x33, 0x5e, 0xbd, 0x5a, 0xae, 0x8c, 0x8c, 0x8d, 0x2e, 0x83, 0x74, 0x60, 0xaa, 0x2b, 0x81, 0x3d, 0x86}
		var ok bool
		base, mask, palette, ok = loadGPURecoloredMobileFrame(451, 0, colors)
		if !ok {
			return fmt.Errorf("real mobile 451 unavailable")
		}
		colors[0] ^= 3
		other = mobilePaletteState(451, colors)
	}
	canvas := ebiten.NewImage(768, 512)
	defer canvas.Deallocate()
	readback := canvas.SubImage(image.Rect(0, 0, 1, 1)).(*ebiten.Image)
	pixel := make([]byte, 4)
	for kind := range 4 {
		in := samplingInputs{base, base, mask, mask, palette, other}
		if kind == 3 {
			in.currentPal = palette
		}
		for _, transform := range []struct {
			name          string
			scale, offset float64
		}{{"Aligned1x", 1, 0}, {"Fractional1x", 1, 0.37}, {"Aligned2x", 2, 0}, {"Fractional1.25x", 1.25, 0.37}} {
			op := frameBlendDrawOptions{ScaleX: transform.scale, ScaleY: transform.scale, Fade: 0.375, Red: 1, Green: 1, Blue: 1, Alpha: 1, Linear: true}
			w, h := float64(base.Bounds().Dx())*transform.scale, float64(base.Bounds().Dy())*transform.scale
			columns, rows := int(768/w), int(512/h)
			measure := func(version int) time.Duration {
				start := time.Now()
				for range 6 {
					for y := range rows {
						for x := range columns {
							op.Left, op.Top = float64(x)*w+transform.offset, float64(y)*h+transform.offset
							draw(canvas, version, min(kind, 2), in, op)
						}
					}
				}
				readback.ReadPixels(pixel)
				return time.Since(start) / 6
			}
			measure(0)
			measure(1)
			var times [2][]time.Duration
			for n := range 7 {
				for _, version := range []int{n % 2, 1 - n%2} {
					times[version] = append(times[version], measure(version))
				}
			}
			for version := range times {
				sort.Slice(times[version], func(i, j int) bool { return times[version][i] < times[version][j] })
			}
			g.t.Logf("kind=%d %s grid=%d before=%v after=%v change=%.1f%%", kind, transform.name, columns*rows, times[0][3], times[1][3], 100*(float64(times[1][3])/float64(times[0][3])-1))
		}
	}
	return nil
}
