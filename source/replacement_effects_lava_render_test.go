package main

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestRenderLavaPoolKeepsSourceFootprint(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_LAVA_TEST") == "" {
		t.Skip("set GOTHOOM_RENDER_LAVA_TEST=1 to verify lava pool pixels")
	}
	game := &lavaPoolFootprintRenderGame{}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

type lavaPoolFootprintRenderGame struct {
	rendered bool
	err      error
}

func (g *lavaPoolFootprintRenderGame) Update() error {
	if g.rendered {
		return ebiten.Termination
	}
	return nil
}

func (g *lavaPoolFootprintRenderGame) Draw(_ *ebiten.Image) {
	const nativeW, nativeH = 9, 7
	shader, err := ebiten.NewShader(lavaPoolShaderSource)
	if err != nil {
		g.err, g.rendered = err, true
		return
	}
	defer shader.Deallocate()
	sheet := ebiten.NewImage(nativeW+4, nativeH+6)
	defer sheet.Deallocate()
	outline := sheet.SubImage(image.Rect(2, 3, 2+nativeW, 3+nativeH)).(*ebiten.Image)
	maskPixels := make([]byte, nativeW*nativeH*4)
	for y := 0; y < nativeH; y++ {
		for x := 0; x < nativeW; x++ {
			if y == 3 || x == 4 && (y == 0 || y == 6) || x >= 2 && x <= 6 && y >= 1 && y <= 5 && !(x == 4 && y == 2) {
				maskPixels[(y*nativeW+x)*4+3] = 255
			}
		}
	}
	outline.WritePixels(maskPixels)
	for _, scale := range []int{1, 2} {
		width, height := nativeW*scale, nativeH*scale
		state := replacementEffectShaderState{
			size:        [2]float32{float32(width), float32(height)},
			outlineSize: [2]float32{nativeW, nativeH},
		}
		state.uniforms = map[string]any{
			"Size": state.size[:], "Phase": float32(0), "Alpha": float32(1),
			"HasPoolOutline": float32(1), "OutlineSize": state.outlineSize[:],
		}
		state.op.Uniforms = state.uniforms
		state.op.Images[0] = outline
		var first []byte
		for _, phase := range []float32{0, 1.2} {
			state.uniforms["Phase"] = phase
			canvas := ebiten.NewImage(width, height)
			drawReplacementEffectShader(canvas, 0, 0, width, height, shader, &state)
			pixels := make([]byte, width*height*4)
			canvas.ReadPixels(pixels)
			canvas.Deallocate()
			if first == nil {
				first = pixels
				continue
			}
			changedColor := false
			for i := 0; i < len(first); i += 4 {
				if first[i+3] != pixels[i+3] {
					g.err = fmt.Errorf("lava shoreline changed at scale %d, pixel %d: alpha %d to %d", scale, i/4, first[i+3], pixels[i+3])
					break
				}
				if first[i+3] != 0 && (first[i] != pixels[i] || first[i+1] != pixels[i+1]) {
					changedColor = true
				}
			}
			if g.err == nil && !changedColor {
				g.err = fmt.Errorf("lava interior did not animate at scale %d", scale)
			}
		}
		if g.err != nil {
			break
		}
		// The original's extremal pixels must remain visible at native size and
		// at 200%, with the same transparent cutout in its irregular interior.
		for _, point := range [][2]int{{0, 3 * scale}, {width - 1, 3 * scale}, {4 * scale, 0}, {4 * scale, height - 1}} {
			if first[(point[1]*width+point[0])*4+3] == 0 {
				g.err = fmt.Errorf("lava footprint shrank at scale %d, point %v", scale, point)
				break
			}
		}
		if first[(2*scale*width+4*scale)*4+3] != 0 {
			g.err = fmt.Errorf("lava leaked into a transparent source cutout at scale %d", scale)
		}
		if g.err != nil {
			break
		}
	}
	if g.err == nil {
		g.checkInlineFloorOrder(shader, outline)
	}
	g.rendered = true
}

func (g *lavaPoolFootprintRenderGame) checkInlineFloorOrder(shader *ebiten.Shader, outline *ebiten.Image) {
	oldEnabled, oldShader, oldDraws, oldNow, oldStarted := gs.ReplacementEffects, lavaPoolShader, replacementEffectDraws, drawFrameNow, replacementEffectsStarted
	defer func() {
		gs.ReplacementEffects, lavaPoolShader, replacementEffectDraws, drawFrameNow, replacementEffectsStarted = oldEnabled, oldShader, oldDraws, oldNow, oldStarted
	}()
	gs.ReplacementEffects = true
	lavaPoolShader = shader
	drawFrameNow = time.Unix(1_000, 0)
	replacementEffectsStarted = drawFrameNow
	key := replacementEffectWorldKey(replacementEffectLavaPool, 0, 0, 0)
	replacementEffectDraws = map[uint64]replacementEffectDraw{
		key: {pictID: 597, kind: replacementEffectLavaPool, width: 32, height: 32, contentWidth: 32, contentHeight: 32,
			alpha: 1, started: drawFrameNow, lastSeen: drawFrameNow, seen: true, maskImage: outline},
	}
	canvas := ebiten.NewImage(32, 32)
	defer canvas.Deallocate()
	drawReplacementEffectsLayerSelected(canvas, 0, 0, nil, nil, nil, 0, 0, 1, 0, true, nil, key)
	if !replacementEffectDraws[key].drawnInline {
		g.err = fmt.Errorf("inline floor effect was not marked as already drawn")
		return
	}
	pixels := make([]byte, 32*32*4)
	canvas.ReadPixels(pixels)
	center := (16*32 + 16) * 4
	if pixels[center+3] == 0 {
		g.err = fmt.Errorf("inline floor effect did not draw")
		return
	}
	canvas.Fill(color.RGBA{G: 255, A: 255}) // A later scenery picture.
	drawReplacementEffectsLayerSelected(canvas, 0, 0, nil, nil, nil, 0, 0, 1, 0, true, nil, 0)
	canvas.ReadPixels(pixels)
	if got := pixels[center : center+4]; got[0] != 0 || got[1] != 255 || got[2] != 0 || got[3] != 255 {
		g.err = fmt.Errorf("later scenery was covered by the floor pass: %v", got)
	}
}

func (g *lavaPoolFootprintRenderGame) Layout(_, _ int) (int, int) { return 32, 32 }
