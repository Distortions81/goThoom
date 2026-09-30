package main

import (
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"os"
	"os/exec"
	"sort"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed testdata/shaders/light_before_cone_optimization.kage
var referenceLightingConeShader []byte

func TestLightingConeOptimizationPixels(t *testing.T) {
	if os.Getenv("GOTHOOM_CONE_COMPARE_CHILD") == "" {
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(exe, "-test.run=^TestLightingConeOptimizationPixels$", "-test.v")
		cmd.Env = append(os.Environ(), "GOTHOOM_CONE_COMPARE_CHILD=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cone comparison: %v\n%s", err, out)
		}
		if os.Getenv("GOTHOOM_CONE_TIMING") != "" {
			t.Logf("%s", out)
		}
		return
	}
	g := &lightingConeComparisonGame{t: t}
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

type lightingConeComparisonGame struct {
	t    *testing.T
	done bool
	err  error
}

func (g *lightingConeComparisonGame) Layout(int, int) (int, int) { return 128, 96 }
func (g *lightingConeComparisonGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *lightingConeComparisonGame) Draw(*ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	before, err := compileLightingShaderVariants(referenceLightingConeShader)
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
	if g.err == nil && os.Getenv("GOTHOOM_CONE_TIMING") != "" {
		timeLightingConeShaders(g.t, before, after)
	}
}

func coneComparisonLights(count int, width, height float32) []lightSource {
	colors := [][3]float32{{1, 1, 1}, {1, 0.2, 0}, {0, 0.4, 1}, {0.3, 1, 0.2}}
	lights := make([]lightSource, count)
	for i := range lights {
		c := colors[i%len(colors)]
		lights[i] = lightSource{
			X: width * (0.1 + float32(i%7)*0.12), Y: height * (0.15 + float32(i%5)*0.16),
			Radius: width * (0.04 + float32(i%3)*0.08), R: c[0], G: c[1], B: c[2],
			Intensity: 0.3 + float32(i%3)*0.3, Plane: int16(i % 3),
		}
	}
	return lights
}

func compareLightingConePixels(before, after []lightingShaderVariant) error {
	// A subimage exercises nonzero backing-texture origins as well as lighting
	// cones crossing the viewport, soft edges, cutoffs, and cropped shadow masks.
	backing := ebiten.NewImage(160, 128)
	defer backing.Deallocate()
	backing.Fill(color.RGBA{R: 110, G: 85, B: 60, A: 255})
	backing.SubImage(image.Rect(16, 24, 48, 48)).(*ebiten.Image).Fill(color.NRGBA{R: 30, G: 80, B: 140, A: 128})
	source := backing.SubImage(image.Rect(16, 24, 144, 120)).(*ebiten.Image)
	mask := ebiten.NewImage(16, 12)
	defer mask.Deallocate()
	mask.Fill(color.NRGBA{A: 90})
	gs.ShaderLightStrength, gs.ShaderGlowStrength = 1, 1
	for _, count := range []int{0, 1, 9, 33, 65, 128} {
		lights := coneComparisonLights(count, 128, 96)
		for i := range lights {
			lights[i].X += 16
			lights[i].Y += 24
		}
		for _, radius := range []float32{1, 7, 24} {
			for _, night := range []float32{0, 0.5, 1} {
				state := &viewportRenderState{nightTransition: nightTransitionState{
					inited: true, previous: night * shaderNightStrength, current: night * shaderNightStrength,
				}}
				state.lighting.casters = []lightCaster{
					{X: 64, Y: 72, Radius: radius}, {X: 96, Y: 48, Radius: radius}, {X: 48, Y: 104, Radius: radius},
				}
				state.lighting.detailedShadowMask = mask
				state.lighting.detailedShadowBounds = image.Rect(32, 40, 48, 52)
				darks := []darkSource{
					{X: 64, Y: 72, Radius: 100, Alpha: night, Intensity: 1},
					{X: 96, Y: 48, Radius: 30, Alpha: 0.2, Intensity: 1, Plane: 5},
				}
				var pixels [2][]byte
				for i, variants := range [][]lightingShaderVariant{before, after} {
					lightingShaderVariants, lightingShader = variants, variants[0].shader
					dst := ebiten.NewImageWithOptions(source.Bounds(), nil)
					applyWorldCompositeForViewport(state, nightRenderState{}, dst, source, lights, darks, 1, true)
					pixels[i] = make([]byte, 128*96*4)
					dst.ReadPixels(pixels[i])
					dst.Deallocate()
				}
				for i, a := range pixels[0] {
					if d := int(a) - int(pixels[1][i]); d < -1 || d > 1 {
						return fmt.Errorf("lights=%d radius=%v night=%v byte=%d: before=%d after=%d", count, radius, night, i, a, pixels[1][i])
					}
				}
			}
		}
	}
	return nil
}

// Opt-in timings include submission and synchronous readback. They compare
// identical warmed draws in alternating order, rather than timing queued work.
func timeLightingConeShaders(t *testing.T, before, after []lightingShaderVariant) {
	const width, height = 768, 512
	source := ebiten.NewImage(width, height)
	defer source.Deallocate()
	source.Fill(color.RGBA{R: 110, G: 85, B: 60, A: 255})
	dst := ebiten.NewImage(width, height)
	defer dst.Deallocate()
	readback := dst.SubImage(image.Rect(width/2, height/2, width/2+1, height/2+1)).(*ebiten.Image)
	pixels := make([]byte, 4)
	for _, count := range []int{8, 32, 64} {
		lights := coneComparisonLights(count, width, height)
		state := &viewportRenderState{nightTransition: nightTransitionState{
			inited: true, previous: 0.5 * shaderNightStrength, current: 0.5 * shaderNightStrength,
		}}
		state.lighting.casters = []lightCaster{{X: 330, Y: 270, Radius: 7}, {X: 500, Y: 160, Radius: 12}}
		darks := []darkSource{{X: 384, Y: 256, Radius: 500, Alpha: 0.5, Intensity: 1}}
		measure := func(variants []lightingShaderVariant) time.Duration {
			lightingShaderVariants, lightingShader = variants, variants[0].shader
			start := time.Now()
			for range 100 {
				applyWorldCompositeForViewport(state, nightRenderState{}, dst, source, lights, darks, 1, true)
			}
			readback.ReadPixels(pixels)
			return time.Since(start) / 100
		}
		measure(before)
		measure(after)
		var baseline, optimized []time.Duration
		for sample := range 7 {
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
		t.Logf("768x512 lights=%d shadows=%d median per composition (100 draws + readback): before=%s after=%s change=%.1f%%", count, len(state.lighting.shadows), a, b, 100*(float64(b)/float64(a)-1))
	}
}
