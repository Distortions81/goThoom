package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"os"
	"os/exec"
	"slices"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed testdata/shaders/town_puddle_before_ripple_cull.kage
var townPuddleBeforeRippleCull []byte

func TestTownPuddleCullingPixels(t *testing.T) {
	// RunGame can run only once per process. Keep this pixel comparison usable
	// alongside the other render tests in the normal package suite.
	if os.Getenv("GOTHOOM_EFFECT_CULLING_CHILD") == "" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(executable, "-test.run=^TestTownPuddleCullingPixels$", "-test.v")
		cmd.Env = append(os.Environ(), "GOTHOOM_EFFECT_CULLING_CHILD=1")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("effect culling pixel comparison: %v\n%s", err, output)
		}
		t.Logf("%s", output)
		return
	}
	game := &effectCullingRenderGame{t: t}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
	if !game.done {
		t.Fatal("effect culling pixel comparison did not run")
	}
}

type effectCullingRenderGame struct {
	t    *testing.T
	done bool
	err  error
}

func (*effectCullingRenderGame) Layout(int, int) (int, int) { return 120, 40 }
func (g *effectCullingRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *effectCullingRenderGame) Draw(*ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	g.err = g.comparePuddles()
}

func effectCullingShaders(before, after []byte) ([2]*ebiten.Shader, error) {
	var shaders [2]*ebiten.Shader
	for i, source := range [][]byte{before, after} {
		shader, err := ebiten.NewShader(source)
		if err != nil {
			if shaders[0] != nil {
				shaders[0].Deallocate()
			}
			return shaders, err
		}
		shaders[i] = shader
	}
	return shaders, nil
}

func compareEffectCullingPixels(name string, shaders [2]*ebiten.Shader, width, height int, state *replacementEffectShaderState) error {
	var pixels [2][]byte
	for i, shader := range shaders {
		canvas := ebiten.NewImageWithOptions(image.Rect(0, 0, width, height), &ebiten.NewImageOptions{Unmanaged: true})
		drawReplacementEffectShader(canvas, 0, 0, width, height, shader, state)
		pixels[i] = make([]byte, width*height*4)
		canvas.ReadPixels(pixels[i])
		canvas.Deallocate()
	}
	if !bytes.Equal(pixels[0], pixels[1]) {
		for i, value := range pixels[0] {
			if value != pixels[1][i] {
				p := i / 4 * 4
				return fmt.Errorf("%s pixel (%d,%d): before %v, after %v", name, i/4%width, i/4/width, pixels[0][p:p+4], pixels[1][p:p+4])
			}
		}
	}
	return nil
}

func (g *effectCullingRenderGame) comparePuddles() error {
	shaders, err := effectCullingShaders(townPuddleBeforeRippleCull, townPuddleShaderSource)
	if err != nil {
		return err
	}
	defer shaders[0].Deallocate()
	defer shaders[1].Deallocate()
	for _, size := range [][2]int{{60, 20}, {84, 20}, {120, 40}} {
		w, h := size[0], size[1]
		source := ebiten.NewImage(w, h)
		reflection := ebiten.NewImage(w, h)
		reflection.Fill(color.RGBA{R: 96, G: 160, B: 32, A: 192})
		state := replacementEffectShaderState{uniforms: map[string]any{
			"Size": []float32{float32(w), float32(h)}, "Alpha": float32(0.81),
			"RippleSeeds": []float32{0, 0.3, 1.7, 3.2, 8.1, 13.4},
		}}
		state.op.Uniforms = state.uniforms
		state.op.Images[0], state.op.Images[1] = source, reflection
		for _, variant := range []float32{0, 1} {
			state.uniforms["PuddleVariant"] = variant
			for _, reflected := range []float32{0, 1} {
				state.uniforms["HasReflection"] = reflected
				for frame, age := range []float32{0, 0.1999, 0.20, 0.7799, 0.98} {
					state.uniforms["Phase"], state.uniforms["RippleTime"] = float32(frame)*0.71, float32(frame)*1.13
					state.uniforms["RippleAges"] = []float32{age, age, age, age, age, age}
					state.uniforms["RippleMotion"] = []float32{1, 0.7, 1, 1, 0.5, 0}
					// Center, shoreline and offscreen feet exercise the whole
					// ring, including both sides of the conservative cutoff.
					unit := float32(w) / 60
					state.uniforms["RippleFeet"] = []float32{
						float32(w) * 0.5, float32(h) * 0.5,
						float32(w) * 0.2, float32(h) * 0.8,
						float32(w)*0.5 - 17.2*unit, float32(h) * 0.5,
						float32(w)*0.5 + 18.001*unit, float32(h) * 0.5,
						float32(w) * 1.2, float32(h) * -0.2,
						float32(w) * 0.5, float32(h) * 0.5,
					}
					label := fmt.Sprintf("puddle size=%v variant=%g reflection=%g age=%g", size, variant, reflected, age)
					if err := compareEffectCullingPixels(label, shaders, w, h, &state); err != nil {
						source.Deallocate()
						reflection.Deallocate()
						return err
					}
				}
			}
		}
		if os.Getenv("GOTHOOM_EFFECT_CULLING_TIMING") != "" {
			for _, count := range []int{0, 1, 6} {
				motion := make([]float32, 6)
				for i := range count {
					motion[i] = 1
				}
				state.uniforms["RippleMotion"] = motion
				state.uniforms["RippleAges"] = []float32{0.3, 0.4, 0.5, 0.6, 0.7, 0.8}
				timeEffectCullingShaders(g.t, fmt.Sprintf("puddle-%dx%d-feet%d", w, h, count), shaders, w, h, &state)
			}
		}
		source.Deallocate()
		reflection.Deallocate()
	}
	return nil
}

// The opt-in timing includes synchronized GPU completion and alternates shader
// order. It measures a grid of identical effects, with no shader compilation
// or texture allocation in the timed section.
func timeEffectCullingShaders(t *testing.T, name string, shaders [2]*ebiten.Shader, width, height int, state *replacementEffectShaderState) {
	canvas := ebiten.NewImage(768, 512)
	defer canvas.Deallocate()
	readback := canvas.SubImage(image.Rect(0, 0, 1, 1)).(*ebiten.Image)
	pixel := make([]byte, 4)
	columns, rows := max(1, 768/width), max(1, 512/height)
	measure := func(version int) time.Duration {
		start := time.Now()
		for range 20 {
			for y := range rows {
				for x := range columns {
					drawReplacementEffectShader(canvas, float64(x*width), float64(y*height), width, height, shaders[version], state)
				}
			}
		}
		readback.ReadPixels(pixel)
		return time.Since(start) / 20
	}
	measure(0)
	measure(1)
	var times [2][]time.Duration
	for sample := range 9 {
		for _, version := range []int{sample % 2, 1 - sample%2} {
			times[version] = append(times[version], measure(version))
		}
	}
	slices.Sort(times[0])
	slices.Sort(times[1])
	t.Logf("%s grid=%d before=%v after=%v change=%.1f%%", name, columns*rows, times[0][4], times[1][4], 100*(float64(times[1][4])/float64(times[0][4])-1))
}
