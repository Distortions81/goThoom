package main

import (
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"os"
	"os/exec"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// Frozen reference for pixel comparisons after moving color math to the CPU.
//
//go:embed testdata/shaders/light_before_color_optimization.kage
var referenceLightingColorShader []byte

func TestLightingColorOptimizationPixels(t *testing.T) {
	if os.Getenv("GOTHOOM_COLOR_COMPARE_CHILD") == "" {
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(exe, "-test.run=^TestLightingColorOptimizationPixels$")
		cmd.Env = append(os.Environ(), "GOTHOOM_COLOR_COMPARE_CHILD=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("render comparison: %v\n%s", err, out)
		}
		return
	}
	game := &lightingColorComparisonGame{}
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

type lightingColorComparisonGame struct {
	done bool
	err  error
}

func (g *lightingColorComparisonGame) Layout(int, int) (int, int) { return 96, 96 }
func (g *lightingColorComparisonGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *lightingColorComparisonGame) Draw(_ *ebiten.Image) {
	if !g.done {
		g.err = compareLightingColors()
		g.done = true
	}
}

func compareLightingColors() error {
	old, err := compileLightingShaderVariants(referenceLightingColorShader)
	if err != nil {
		return err
	}
	current, err := compileLightingShaderVariants(lightShaderSrc)
	if err != nil {
		return err
	}
	defer func() {
		for _, v := range append(old, current...) {
			v.shader.Deallocate()
		}
	}()
	gs.ShaderLightStrength, gs.ShaderGlowStrength = 1, 1
	nightAlphaInited = true
	source := ebiten.NewImage(96, 96)
	defer source.Deallocate()
	source.Fill(color.RGBA{R: 110, G: 85, B: 60, A: 255})
	// Include transparency and color variation as well as the opaque background.
	source.SubImage(image.Rect(0, 0, 32, 32)).(*ebiten.Image).Fill(color.RGBA{R: 30, G: 60, B: 80, A: 128})
	frameLightCasters = []lightCaster{{X: 48, Y: 48, Radius: 5}, {X: 60, Y: 32, Radius: 4}}
	colors := [][3]float32{{1, 1, 1}, {1, 0, 0}, {0, 0, 1}, {0, 0, 0}, {0.0001, 0.0002, 0.0003}, {0.7, 0.3, 0.1}}
	for _, count := range []int{0, 1, 9, 33, 65} {
		lights := make([]lightSource, count)
		for i := range lights {
			c := colors[i%len(colors)]
			lights[i] = lightSource{X: float32(8 + i%8*10), Y: float32(8 + i/8*10), Radius: 25, R: c[0], G: c[1], B: c[2], Intensity: 0.7, Plane: int16(i % 3)}
		}
		for _, night := range []float32{0, 0.5, 1} {
			nightPrevTarget, nightCurTarget = night*shaderNightStrength, night*shaderNightStrength
			darks := []darkSource{{X: 48, Y: 48, Radius: 100, Alpha: night, Intensity: 1, Plane: 0}, {X: 75, Y: 20, Radius: 30, Alpha: 0.2, Intensity: 1, Plane: 5}}
			var pixels [2][]byte
			for i, variants := range [][]lightingShaderVariant{old, current} {
				lightingShaderVariants = variants
				lightingShader = variants[0].shader
				dst := ebiten.NewImage(96, 96)
				applyWorldComposite(dst, source, lights, darks, 1, true)
				pixels[i] = make([]byte, 96*96*4)
				dst.ReadPixels(pixels[i])
				dst.Deallocate()
			}
			for i, a := range pixels[0] {
				delta := int(a) - int(pixels[1][i])
				if delta < -1 || delta > 1 {
					return fmt.Errorf("lights=%d night=%v byte=%d: old=%d new=%d", count, night, i, a, pixels[1][i])
				}
			}
		}
	}
	return nil
}
