package main

import (
	"fmt"
	"image/color"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestRenderTownPuddleReflectionScratchReuse(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_PUDDLE_REFLECTION_TEST") == "" {
		t.Skip("set GOTHOOM_RENDER_PUDDLE_REFLECTION_TEST=1 to verify puddle reflection pixels")
	}
	game := &townPuddleReflectionRenderGame{}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

type townPuddleReflectionRenderGame struct {
	rendered bool
	err      error
}

func (g *townPuddleReflectionRenderGame) Update() error {
	if g.rendered {
		return ebiten.Termination
	}
	return nil
}

func (g *townPuddleReflectionRenderGame) Draw(_ *ebiten.Image) {
	const width, height = 84, 20
	shader, err := ebiten.NewShader(townPuddleShaderSource)
	if err != nil {
		g.err = err
		g.rendered = true
		return
	}
	defer shader.Deallocate()
	scratch := ebiten.NewImage(width, height)
	defer scratch.Deallocate()
	red, green := ebiten.NewImage(width, height), ebiten.NewImage(width, height)
	defer red.Deallocate()
	defer green.Deallocate()
	state := replacementEffectShaderState{size: [2]float32{width, height}}
	state.uniforms = map[string]any{
		"Size":          state.size[:],
		"Phase":         float32(0),
		"Alpha":         float32(1),
		"HasReflection": float32(1),
		"RippleFeet":    state.rippleFeet[:],
		"RippleMotion":  state.rippleMotion[:],
		"RippleSeeds":   state.rippleSeeds[:],
		"RippleAges":    state.rippleAges[:],
		"RippleTime":    float32(0),
	}
	state.op.Uniforms = state.uniforms
	state.op.Images[0], state.op.Images[1] = scratch, scratch
	scratch.Fill(color.RGBA{R: 255, A: 255})
	drawReplacementEffectShader(red, 0, 0, width, height, shader, &state)
	scratch.Fill(color.RGBA{G: 255, A: 255})
	drawReplacementEffectShader(green, 0, 0, width, height, shader, &state)
	redPixel, greenPixel := make([]byte, width*height*4), make([]byte, width*height*4)
	red.ReadPixels(redPixel)
	green.ReadPixels(greenPixel)
	center := (height/2*width + width/2) * 4
	if int(redPixel[center])-int(greenPixel[center]) < 12 || int(greenPixel[center+1])-int(redPixel[center+1]) < 12 {
		g.err = fmt.Errorf("reused reflection scratch did not preserve separate mobile colors: red=%v green=%v", redPixel[center:center+4], greenPixel[center:center+4])
	}
	if redPixel[3] != 0 || greenPixel[3] != 0 {
		g.err = fmt.Errorf("reflections leaked outside puddle silhouette: red alpha=%d green alpha=%d", redPixel[3], greenPixel[3])
	}
	// A moving foot brightens an expanding ring; a stationary foot does not.
	still, moving := ebiten.NewImage(width, height), ebiten.NewImage(width, height)
	defer still.Deallocate()
	defer moving.Deallocate()
	state.uniforms["HasReflection"] = float32(0)
	state.rippleFeet[0], state.rippleFeet[1] = width/2, height/2
	drawReplacementEffectShader(still, 0, 0, width, height, shader, &state)
	state.rippleMotion[0] = 1
	drawReplacementEffectShader(moving, 0, 0, width, height, shader, &state)
	stillPixel, movingPixel := make([]byte, width*height*4), make([]byte, width*height*4)
	still.ReadPixels(stillPixel)
	moving.ReadPixels(movingPixel)
	contactRing := (height/2*width + width/2 + 2) * 4
	if int(movingPixel[contactRing+1])-int(stillPixel[contactRing+1]) < 12 {
		g.err = fmt.Errorf("moving foot did not brighten puddle ring: still=%v moving=%v", stillPixel[contactRing:contactRing+4], movingPixel[contactRing:contactRing+4])
	}
	if movingPixel[3] != 0 {
		g.err = fmt.Errorf("foot ripple leaked outside puddle silhouette: alpha=%d", movingPixel[3])
	}
	// The GPU flip must carry a mobile above the puddle toward the far rim and
	// a mobile below it toward the near rim, not reverse ground-plane movement.
	sprite := ebiten.NewImage(8, 8)
	defer sprite.Deallocate()
	sprite.Fill(color.White)
	positions := [2]float64{-10, 10}
	var centroids [2]float64
	for index, offset := range positions {
		target := ebiten.NewImage(width, height)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(1, -float64(height)*0.82/8)
		op.GeoM.Translate(38, townPuddleReflectionVerticalTop(offset, 0, height, 0.9))
		target.DrawImage(sprite, op)
		pixels := make([]byte, width*height*4)
		target.ReadPixels(pixels)
		var weightedY, alphaSum float64
		for y := range height {
			for x := range width {
				a := float64(pixels[(y*width+x)*4+3])
				weightedY += float64(y) * a
				alphaSum += a
			}
		}
		target.Deallocate()
		if alphaSum == 0 {
			g.err = fmt.Errorf("flipped reflection at offset %v was not drawn", offset)
			break
		}
		centroids[index] = weightedY / alphaSum
	}
	if g.err == nil && centroids[1] <= centroids[0]+2 {
		g.err = fmt.Errorf("flipped reflection did not follow vertical movement: above=%v below=%v", centroids[0], centroids[1])
	}
	g.rendered = true
}

func (g *townPuddleReflectionRenderGame) Layout(_, _ int) (int, int) { return 84, 20 }
