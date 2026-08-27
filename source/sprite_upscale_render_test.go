package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gothoom/climg"

	"github.com/hajimehoshi/ebiten/v2"
)

// TestRenderSpriteUpscaleImages is an opt-in comparison using real CL_Images
// sprites. Each output places raw nearest-neighbor scaling first, followed by
// the Crisp, Balanced, Smooth, and Ultra Smooth Kage modes.
func TestRenderSpriteUpscaleImages(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_UPSCALE_TESTS") == "" {
		t.Skip("set GOTHOOM_RENDER_UPSCALE_TESTS=1 to render upscale diagnostics")
	}
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate upscale render test source")
	}
	outputDir := filepath.Join(filepath.Dir(sourceFile), "data", "Screenshots", "upscale-tests")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	images, err := climg.Load(filepath.Join(filepath.Dir(sourceFile), "data", "CL_Images"))
	if err != nil {
		t.Fatalf("load CL_Images: %v", err)
	}

	originalSettings, originalImages := gs, clImages
	gs.PotatoGPU = false
	clImages = images
	t.Cleanup(func() {
		clearCaches()
		clImages = originalImages
		gs = originalSettings
	})

	game := &spriteUpscaleRenderGame{outputDir: outputDir}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

const (
	spriteUpscaleCanvasW  = 1280
	spriteUpscaleCanvasH  = 256
	spriteUpscaleTestZoom = 4
)

type spriteUpscaleRenderGame struct {
	outputDir string
	rendered  bool
	err       error
}

func (g *spriteUpscaleRenderGame) Update() error {
	if g.rendered {
		return ebiten.Termination
	}
	return nil
}

func (g *spriteUpscaleRenderGame) Draw(_ *ebiten.Image) {
	g.err = g.renderComparisons()
	g.rendered = true
}

func (g *spriteUpscaleRenderGame) Layout(_, _ int) (int, int) {
	return spriteUpscaleCanvasW, spriteUpscaleCanvasH
}

func (g *spriteUpscaleRenderGame) renderComparisons() error {
	characters := []struct {
		name   string
		pictID uint16
	}{
		{name: "neutral", pictID: 22},
		{name: "male", pictID: 447},
		{name: "female", pictID: 456},
	}
	for _, character := range characters {
		sprite := loadMobileFrame(character.pictID, 0, nil)
		if sprite == nil {
			return fmt.Errorf("load mobile pict %d", character.pictID)
		}
		canvas := ebiten.NewImageFromImage(spriteUpscaleBackground())

		nearestOp := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
		nearestOp.GeoM.Scale(spriteUpscaleTestZoom, spriteUpscaleTestZoom)
		nearestOp.GeoM.Translate(16, 32)
		canvas.DrawImage(sprite, nearestOp)

		for i, mode := range []int{artworkUpscaleCrisp, artworkUpscaleBalanced, artworkUpscaleSmooth, artworkUpscaleUltraSmooth} {
			setArtworkUpscaleMode(mode)
			upscaled := upscaleTransientSpriteImageWithMode(sprite, spriteUpscaleTestZoom, artworkUpscaleMode())
			shaderOp := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
			shaderOp.GeoM.Translate(float64(272+i*256), 32)
			canvas.DrawImage(upscaled, shaderOp)
			upscaled.Deallocate()
		}

		name := fmt.Sprintf("upscale_%s_%d_%dx.png", character.name, character.pictID, spriteUpscaleTestZoom)
		if err := writeSpriteUpscalePNG(filepath.Join(g.outputDir, name), canvas); err != nil {
			return err
		}
		canvas.Deallocate()
	}
	return nil
}

func spriteUpscaleBackground() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, spriteUpscaleCanvasW, spriteUpscaleCanvasH))
	for y := 0; y < spriteUpscaleCanvasH; y++ {
		for x := 0; x < spriteUpscaleCanvasW; x++ {
			shade := uint8(176)
			if (x/16+y/16)%2 == 0 {
				shade = 200
			}
			img.SetRGBA(x, y, color.RGBA{R: shade, G: shade, B: shade, A: 255})
		}
	}
	return img
}

func writeSpriteUpscalePNG(path string, canvas *ebiten.Image) error {
	pixels := make([]byte, spriteUpscaleCanvasW*spriteUpscaleCanvasH*4)
	canvas.ReadPixels(pixels)
	img := &image.RGBA{
		Pix: pixels, Stride: spriteUpscaleCanvasW * 4,
		Rect: image.Rect(0, 0, spriteUpscaleCanvasW, spriteUpscaleCanvasH),
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(file, img); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
