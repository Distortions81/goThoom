package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestRenderMultiViewportAtlasRegions(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_MULTI_VIEWPORT_TEST") == "" {
		t.Skip("set GOTHOOM_RENDER_MULTI_VIEWPORT_TEST=1 to verify shared viewport atlas pixels")
	}
	game := &multiViewportAtlasRenderGame{}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

type multiViewportAtlasRenderGame struct {
	done bool
	err  error
}

func (*multiViewportAtlasRenderGame) Layout(_, _ int) (int, int) { return 64, 64 }

func (g *multiViewportAtlasRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}

func (g *multiViewportAtlasRenderGame) Draw(_ *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true

	const width, height = 20, 12
	rects, size, ok := packViewportRenderRects([]image.Point{{X: width, Y: height}, {X: width, Y: height}}, ebiten.MaxImageSize())
	if !ok {
		g.err = fmt.Errorf("pack two viewport regions")
		return
	}
	var atlas viewportRenderAtlas
	if !atlas.ensure(size) {
		g.err = fmt.Errorf("allocate viewport atlas %v", size)
		return
	}
	defer atlas.scene.Deallocate()
	defer atlas.lit.Deallocate()
	if err := ReloadLightingShader(); err != nil {
		g.err = fmt.Errorf("compile lighting shader: %w", err)
		return
	}
	gs.ShaderLightStrength = 1
	gs.ShaderGlowStrength = 1

	want := []color.RGBA{{R: 230, G: 35, B: 25, A: 255}, {R: 20, G: 80, B: 220, A: 255}}
	var litPixels [2][]byte
	for index, rect := range rects {
		region := atlas.scene.RecyclableSubImage(rect)
		region.Fill(color.RGBA{R: 70, G: 80, B: 90, A: 255})
		lit := atlas.lit.RecyclableSubImage(rect)
		state := &viewportRenderState{}
		applyWorldCompositeForViewport(state, nightRenderState{}, lit, region, []lightSource{{
			X: float32(rect.Min.X + width/2), Y: float32(rect.Min.Y + height/2),
			Radius: 8, R: 1, G: 0.7, B: 0.4, Intensity: 1,
		}}, nil, 1, true)
		litPixels[index] = make([]byte, width*height*4)
		lit.ReadPixels(litPixels[index])
		lit.Recycle()
		region.Fill(want[index])
		target := ebiten.NewImage(width, height)
		frame := viewportRenderFrame{
			request:     viewportRenderRequest{target: target},
			result:      viewportRenderResult{viewRect: target.Bounds(), rendered: true},
			scene:       region,
			output:      region,
			atlasBacked: true,
		}
		publishViewportRenderFrame(&frame, nil)
		pixels := make([]byte, width*height*4)
		target.ReadPixels(pixels)
		target.Deallocate()
		got := color.RGBA{R: pixels[0], G: pixels[1], B: pixels[2], A: pixels[3]}
		if got != want[index] {
			g.err = fmt.Errorf("viewport %d first pixel = %v, want %v", index+1, got, want[index])
			return
		}
	}
	if !bytes.Equal(litPixels[0], litPixels[1]) {
		g.err = fmt.Errorf("identical lighting changed with atlas region origin")
	}
}
