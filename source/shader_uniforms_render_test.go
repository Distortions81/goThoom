package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"os"
	"os/exec"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestShaderScalarUniformPixels(t *testing.T) {
	if os.Getenv("GOTHOOM_SCALAR_UNIFORM_CHILD") == "" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(executable, "-test.run=^TestShaderScalarUniformPixels$")
		cmd.Env = append(os.Environ(), "GOTHOOM_SCALAR_UNIFORM_CHILD=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("scalar uniform pixel comparison: %v\n%s", err, output)
		}
		return
	}
	game := &shaderScalarComparisonGame{}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
	if !game.done {
		t.Fatal("scalar uniform pixel comparison did not run")
	}
}

type shaderScalarComparisonGame struct {
	done bool
	err  error
}

func (*shaderScalarComparisonGame) Layout(int, int) (int, int) { return 48, 32 }
func (g *shaderScalarComparisonGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *shaderScalarComparisonGame) Draw(*ebiten.Image) {
	if !g.done {
		g.done = true
		g.err = compareShaderScalarUniforms()
	}
}

func compareShaderScalarUniforms() error {
	source := ebiten.NewImage(48, 32)
	defer source.Deallocate()
	source.Fill(color.RGBA{R: 130, G: 80, B: 30, A: 200})
	variants, err := compileLightingShaderVariants(lightShaderSrc)
	if err != nil {
		return err
	}
	defer func() {
		for _, variant := range variants {
			variant.shader.Deallocate()
		}
	}()
	copy(packedLightGeometry[:], []float32{24, 16, 1.0 / 400, 0.8})
	copy(packedLightColor[:], []float32{1, 0.5, 0.2, 0})
	copy(packedLightEqualizedColor[:], []float32{1, 0.5, 0.2, 0})
	copy(packedDarkGeometry[:], []float32{20, 12, 1.0 / 225, 0.5})
	copy(packedDarkParameters[:], []float32{1, 0})
	characterShadowMin, characterShadowMax = [2]float32{4, 4}, [2]float32{28, 20}
	for _, variant := range variants {
		variant.uniforms["LightCount"], variant.uniforms["DarkCount"] = 1, 1
		state := replacementEffectShaderState{op: variant.op, uniforms: variant.uniforms}
		state.op.Images[0], state.op.Images[1] = source, source
		if err := compareBufferedAndBoxedShaderDraws(variant.shader, &state); err != nil {
			return fmt.Errorf("lighting capacity %d: %w", variant.maxLights, err)
		}
	}
	if err := ReloadReplacementEffectsShader(); err != nil {
		return err
	}
	for kind := replacementEffectHealing; kind <= replacementEffectCoinReward; kind++ {
		state := &replacementEffectShaderStates[kind]
		state.size, state.canvasSize, state.outlineSize = [2]float32{48, 32}, [2]float32{48, 32}, [2]float32{48, 32}
		state.coinDigits = [4]float32{2, 4}
		state.rippleFeet = [12]float32{24, 16}
		state.rippleMotion, state.rippleAges = [6]float32{0.8}, [6]float32{0.3}
		state.op.Images[0], state.op.Images[1] = source, source
		if err := compareBufferedAndBoxedShaderDraws(replacementEffectShader(kind), state); err != nil {
			return fmt.Errorf("replacement effect %d: %w", kind, err)
		}
	}
	return nil
}

func compareBufferedAndBoxedShaderDraws(shader *ebiten.Shader, state *replacementEffectShaderState) error {
	var canvases [2][2]*ebiten.Image
	defer func() {
		for _, pair := range canvases {
			for _, canvas := range pair {
				if canvas != nil {
					canvas.Deallocate()
				}
			}
		}
	}()
	for frame := range canvases {
		// Submit both frames before reading either back. Later mutations of the
		// bound scalar slices must not change values in the first queued draw.
		boxed := make(map[string]any, len(state.uniforms))
		for name, value := range state.uniforms {
			if values, ok := value.([]float32); ok && len(values) == 1 {
				setShaderFloat32(state.uniforms, name, 0.2+float32(frame)*0.6)
				boxed[name] = values[0]
			} else {
				boxed[name] = value
			}
		}
		for variant := range canvases[frame] {
			canvas := ebiten.NewImageWithOptions(image.Rect(0, 0, 48, 32), &ebiten.NewImageOptions{Unmanaged: true})
			canvases[frame][variant] = canvas
			op := state.op
			if variant == 0 {
				op.Uniforms = boxed
			}
			draw := replacementEffectShaderState{op: op}
			drawReplacementEffectShader(canvas, 0, 0, 48, 32, shader, &draw)
		}
	}
	for frame, pair := range canvases {
		want, got := make([]byte, 48*32*4), make([]byte, 48*32*4)
		pair[0].ReadPixels(want)
		pair[1].ReadPixels(got)
		if !bytes.Equal(want, got) {
			return fmt.Errorf("buffered scalar uniforms changed frame %d pixels", frame)
		}
	}
	return nil
}
