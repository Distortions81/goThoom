package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"os/exec"
	"sort"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestDrawOptimizationPixels(t *testing.T) {
	if os.Getenv("GOTHOOM_DRAW_COMPARE_CHILD") == "" {
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(exe, "-test.run=^TestDrawOptimizationPixels$", "-test.v")
		cmd.Env = append(os.Environ(), "GOTHOOM_DRAW_COMPARE_CHILD=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("draw comparison: %v\n%s", err, out)
		}
		if os.Getenv("GOTHOOM_DRAW_TIMING") != "" {
			t.Logf("%s", out)
		}
		return
	}
	g := &drawOptimizationComparisonGame{t: t}
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

type drawOptimizationComparisonGame struct {
	t    *testing.T
	done bool
	err  error
}

func (g *drawOptimizationComparisonGame) Layout(int, int) (int, int) { return 128, 96 }
func (g *drawOptimizationComparisonGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *drawOptimizationComparisonGame) Draw(*ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	if err := ReloadLightingShader(); err != nil {
		g.err = err
		return
	}
	canvas := ebiten.NewImageWithOptions(image.Rect(17, 23, 145, 119), nil)
	defer canvas.Deallocate()
	sprite := ebiten.NewImageWithOptions(image.Rect(5, 7, 13, 15), nil)
	defer sprite.Deallocate()
	pixels := make([]byte, 8*8*4)
	for i := range 64 {
		pixels[i*4], pixels[i*4+3] = 100, uint8(100+i*2)
	}
	sprite.WritePixels(pixels)
	referenceClear := func(src *ebiten.Image, options *ebiten.DrawImageOptions) {
		op := *options
		op.Blend = shadowCoverageClearBlend
		op.GeoM.Translate(float64(-layeredShadowOrigin.X), float64(-layeredShadowOrigin.Y))
		layeredShadowCoverage.DrawImage(src, &op)
	}
	render := func(optimized bool, linear bool) []byte {
		canvas.Fill(color.RGBA{R: 200, G: 180, B: 160, A: 255})
		beginLayeredCharacterShadowComposite(canvas.Bounds())
		clearReceiver := referenceClear
		if optimized {
			clearReceiver = clearLayeredShadowCoverageImage
		}
		// Receivers before the first shadow have no coverage to remove.
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(45, 45)
		canvas.DrawImage(sprite, op)
		clearReceiver(sprite, op)
		for group := range 4 {
			shadowOp := &ebiten.DrawImageOptions{}
			shadowOp.GeoM.Translate(float64(40+group*4), 43)
			compositeLayeredShadowImage(canvas, sprite, shadowOp, image.Rect(40+group*4, 43, 48+group*4, 51))
			for i := range 32 {
				op := &ebiten.DrawImageOptions{DisableMipmaps: true}
				if linear {
					op.Filter = ebiten.FilterLinear
				}
				op.GeoM.Scale(1.25, -0.75)
				op.GeoM.Rotate(float64(i%4) * math.Pi / 6)
				op.GeoM.Translate(float64(18+i%8*15)+0.4, float64(30+i/8*20)+0.6)
				op.ColorScale.ScaleAlpha(0.75)
				canvas.DrawImage(sprite, op)
				clearReceiver(sprite, op)
			}
		}
		// Compare coverage as well as the visible scene: final skipped clears
		// must leave the mask equivalent even without another caster.
		out := make([]byte, 128*96*4*2)
		canvas.ReadPixels(out[:128*96*4])
		mask := layeredShadowCoverage.RecyclableSubImage(image.Rect(0, 0, 128, 96))
		mask.ReadPixels(out[128*96*4:])
		mask.Recycle()
		return out
	}
	for _, linear := range []bool{false, true} {
		before, after := render(false, linear), render(true, linear)
		for i, value := range before {
			if value != after[i] {
				g.err = fmt.Errorf("draw/mask differs at byte %d, linear=%v: %d != %d", i, linear, value, after[i])
				return
			}
		}
	}
	if os.Getenv("GOTHOOM_DRAW_TIMING") != "" {
		var before, after []time.Duration
		for sample := range 9 {
			for order := range 2 {
				optimized := (sample+order)%2 == 1
				start := time.Now()
				for range 20 {
					render(optimized, false)
				}
				elapsed := time.Since(start) / 20
				if optimized {
					after = append(after, elapsed)
				} else {
					before = append(before, elapsed)
				}
			}
		}
		sort.Slice(before, func(i, j int) bool { return before[i] < before[j] })
		sort.Slice(after, func(i, j int) bool { return after[i] < after[j] })
		g.t.Logf("128 receivers / 4 shadows, including scene and mask readback: before=%s after=%s", before[4], after[4])
	}
	// The target retains its letterbox background in both direct and lighting
	// paths; lighting needs a separately initialized source with matching bounds.
	for _, composite := range []bool{false, true} {
		state := &viewportRenderState{}
		frame := viewportRenderFrame{
			request:      viewportRenderRequest{target: canvas, state: state},
			useComposite: composite, useLighting: true, renderScene: true, renderScale: 1,
		}
		frame.result.viewRect = image.Rect(30, 30, 120, 100)
		if composite {
			// A reused scratch must lose its previous frame's pixels.
			ensureViewportLightingTmp(state, frame.result.viewRect).Fill(color.RGBA{R: 255, A: 255})
		}
		renderViewportSceneStage(&frame)
		for _, target := range []*ebiten.Image{canvas, frame.scene} {
			data := make([]byte, target.Bounds().Dx()*target.Bounds().Dy()*4)
			target.ReadPixels(data)
			want := playfieldBackgroundColor()
			for i := 0; i < len(data); i += 4 {
				if got := (color.RGBA{data[i], data[i+1], data[i+2], data[i+3]}); got != want {
					g.err = fmt.Errorf("viewport background composite=%v: %v != %v", composite, got, want)
					return
				}
			}
		}
		frame.directWorld.Recycle()
		if state.lightingTmp != nil {
			state.lightingTmp.Deallocate()
		}
	}
}
