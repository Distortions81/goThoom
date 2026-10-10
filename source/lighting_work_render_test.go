package main

import (
	_ "embed"
	"image"
	"image/color"
	"os"
	"os/exec"
	"sort"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed testdata/shaders/light_before_loop_optimization.kage
var referenceLightingLoopShader []byte

func TestLightingLoopOptimizationPixels(t *testing.T) {
	compareLightingWorkShaders(t, referenceLightingLoopShader, os.Getenv("GOTHOOM_LIGHTING_LOOP_TIMING") != "")
}

func compareLightingWorkShaders(t *testing.T, reference []byte, timing bool) {
	t.Helper()
	if os.Getenv("GOTHOOM_LIGHTING_WORK_CHILD") == "" {
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(exe, "-test.run=^"+t.Name()+"$", "-test.v")
		cmd.Env = append(os.Environ(), "GOTHOOM_LIGHTING_WORK_CHILD=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("lighting GPU comparison: %v\n%s", err, out)
		}
		if timing {
			t.Logf("%s", out)
		}
		return
	}
	g := &lightingWorkComparisonGame{t: t, reference: reference, timing: timing}
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

type lightingWorkComparisonGame struct {
	t         *testing.T
	reference []byte
	timing    bool
	done      bool
	err       error
}

func (*lightingWorkComparisonGame) Layout(int, int) (int, int) { return 128, 96 }
func (g *lightingWorkComparisonGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *lightingWorkComparisonGame) Draw(*ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	before, err := compileLightingShaderVariants(g.reference)
	if err != nil {
		g.err = err
		return
	}
	after, err := compileLightingShaderVariants(lightShaderSrc)
	if err != nil {
		g.err = err
		return
	}
	defer func() {
		for _, variants := range [][]lightingShaderVariant{before, after} {
			for _, variant := range variants {
				variant.shader.Deallocate()
			}
		}
	}()
	g.err = compareLightingConePixels(before, after)
	if g.err == nil && g.timing {
		timeLightingWorkShaders(g.t, before, after)
	}
}

// Opt-in timings include a synchronous readback so queued shader submission
// cannot be mistaken for the cost of executing the fragments.
func timeLightingWorkShaders(t *testing.T, before, after []lightingShaderVariant) {
	const width, height = 512, 384
	const draws = 40
	source := ebiten.NewImageWithOptions(image.Rect(0, 0, width, height), &ebiten.NewImageOptions{Unmanaged: true})
	defer source.Deallocate()
	source.Fill(color.RGBA{R: 110, G: 85, B: 60, A: 255})
	dst := ebiten.NewImageWithOptions(source.Bounds(), &ebiten.NewImageOptions{Unmanaged: true})
	defer dst.Deallocate()
	readback := dst.SubImage(image.Rect(width/2, height/2, width/2+1, height/2+1)).(*ebiten.Image)
	pixels := make([]byte, 4)
	for _, scene := range []struct {
		lights, darks int
		night         float32
		cones         bool
	}{
		{1, 1, 0.5, false}, {8, 1, 0.5, true}, {33, 1, 0.5, true},
		{46, 5, 0.5, false}, {46, 5, 0.5, true}, {65, 1, 0.5, true},
		{46, 5, 0, false}, {46, 5, 0, true},
	} {
		lights := coneComparisonLights(scene.lights, width, height)
		state := &viewportRenderState{nightTransition: nightTransitionState{
			inited: true, previous: scene.night * shaderNightStrength, current: scene.night * shaderNightStrength,
		}}
		if scene.cones {
			state.lighting.casters = []lightCaster{{X: 220, Y: 205, Radius: 7}, {X: 335, Y: 125, Radius: 12}}
		}
		darks := make([]darkSource, scene.darks)
		for i := range darks {
			darks[i] = darkSource{X: 256 + float32(i)*20, Y: 192, Radius: 350, Alpha: 0.5, Intensity: 1}
		}
		measure := func(variants []lightingShaderVariant) time.Duration {
			lightingShaderVariants, lightingShader = variants, variants[0].shader
			start := time.Now()
			for range draws {
				applyWorldCompositeForViewport(state, nightRenderState{}, dst, source, lights, darks, 1, true)
			}
			readback.ReadPixels(pixels)
			return time.Since(start) / draws
		}
		measure(before)
		measure(after)
		var baseline, optimized []time.Duration
		for sample := range 5 {
			if sample%2 == 0 {
				baseline = append(baseline, measure(before))
				optimized = append(optimized, measure(after))
			} else {
				optimized = append(optimized, measure(after))
				baseline = append(baseline, measure(before))
			}
		}
		sort.Slice(baseline, func(i, j int) bool { return baseline[i] < baseline[j] })
		sort.Slice(optimized, func(i, j int) bool { return optimized[i] < optimized[j] })
		a, b := baseline[len(baseline)/2], optimized[len(optimized)/2]
		t.Logf("512x384 lights=%d darks=%d shadows=%d night=%v before=%s after=%s change=%.1f%%", scene.lights, scene.darks, len(state.lighting.shadows), scene.night, a, b, 100*(float64(b)/float64(a)-1))
	}
}
