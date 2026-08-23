package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"gothoom/climg"

	"github.com/hajimehoshi/ebiten/v2"
)

// TestRenderCharacterShadowImages is an opt-in visual diagnostic. It uses the
// production shadow transform and blend path with real CL_Images mobiles.
func TestRenderCharacterShadowImages(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_SHADOW_TESTS") == "" {
		t.Skip("set GOTHOOM_RENDER_SHADOW_TESTS=1 to render shadow diagnostics")
	}

	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate shadow render test source")
	}
	outputDir := filepath.Join(filepath.Dir(sourceFile), "data", "Screenshots", "shadow-tests")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	images, err := climg.Load(filepath.Join(filepath.Dir(sourceFile), "data", "CL_Images"))
	if err != nil {
		t.Fatalf("load CL_Images: %v", err)
	}

	originalSettings := gs
	originalImages := clImages
	gs.GameScale = 1
	gs.DetailedCharacterShadows = true
	clImages = images
	t.Cleanup(func() {
		clearCaches()
		clImages = originalImages
		gs = originalSettings
	})

	game := &shadowRenderGame{outputDir: outputDir}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if !game.rendered {
		t.Fatal("Ebitengine exited before rendering the shadow diagnostics")
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

const (
	shadowTestCanvasSize = 800
	shadowTestDrawSize   = 48
	shadowTestCenterX    = shadowTestCanvasSize / 2
	shadowTestCenterY    = shadowTestCanvasSize / 2
)

var shadowTestCharacters = []struct {
	name   string
	pictID uint16
}{
	{name: "neutral", pictID: 22},
	{name: "male", pictID: 447},
	{name: "female", pictID: 456},
}

type shadowRenderGame struct {
	outputDir string
	rendered  bool
	err       error
}

func (g *shadowRenderGame) Update() error {
	if g.rendered {
		return ebiten.Termination
	}
	return nil
}

func (g *shadowRenderGame) Draw(_ *ebiten.Image) {
	g.err = g.renderImages()
	g.rendered = true
}

func (g *shadowRenderGame) Layout(_, _ int) (int, int) {
	return shadowTestCanvasSize, shadowTestCanvasSize
}

func (g *shadowRenderGame) renderImages() error {
	if err := verifyCharacterShadowSourceDeterminism(); err != nil {
		return err
	}
	if err := g.renderContactShadows(); err != nil {
		return err
	}
	for _, character := range shadowTestCharacters {
		if err := g.renderWalkCycle(character.name, character.pictID); err != nil {
			return err
		}
		visibleSprite := loadMobileFrame(character.pictID, 0, nil)
		if visibleSprite == nil {
			return fmt.Errorf("load visible mobile pict %d", character.pictID)
		}
		for _, azimuth := range []int{0, 30, 60, 90, 120, 150, 180, 210, 240, 270, 300, 330} {
			shadowState, casts := chooseUprightShadowPose(0, azimuth)
			if !casts {
				continue
			}
			shadowSprite := loadMobileFrame(character.pictID, shadowState, nil)
			if shadowSprite == nil {
				return fmt.Errorf("load shadow mobile pict %d state %d", character.pictID, shadowState)
			}
			shadowTexture := characterShadowTextureFor(shadowSprite)

			projection := newCharacterShadowProjection(azimuth)
			canvas := ebiten.NewImageFromImage(shadowTestBackground(shadowTestCanvasSize))
			mask := ebiten.NewImage(shadowTestCanvasSize, shadowTestCanvasSize)
			drawCharacterShadow(mask, shadowTexture, shadowTestDrawSize, shadowTestCenterX, shadowTestCenterY, 0.75, projection, true, shadowMaskBlend)
			maskOp := &ebiten.DrawImageOptions{Blend: shadowDarkenBlend}
			canvas.DrawImage(mask, maskOp)

			op := &ebiten.DrawImageOptions{}
			op.Filter = ebiten.FilterLinear
			op.GeoM.Scale(float64(shadowTestDrawSize)/float64(visibleSprite.Bounds().Dx()), float64(shadowTestDrawSize)/float64(visibleSprite.Bounds().Dy()))
			op.GeoM.Translate(shadowTestCenterX-shadowTestDrawSize/2, shadowTestCenterY-shadowTestDrawSize/2)
			canvas.DrawImage(visibleSprite, op)

			name := fmt.Sprintf("shadow_%s_%d_sa_%03d_elev_%02.0f_len_%.2f.png", character.name, character.pictID, azimuth, characterShadowSunHeight(azimuth), projection.length)
			if err := writeShadowTestPNG(filepath.Join(g.outputDir, name), canvas); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *shadowRenderGame) renderContactShadows() error {
	canvas := ebiten.NewImageFromImage(shadowTestBackground(shadowTestCanvasSize))
	spacing := shadowTestCanvasSize / (len(shadowTestCharacters) + 1)
	for i, character := range shadowTestCharacters {
		x := spacing * (i + 1)
		y := shadowTestCenterY
		visibleSprite := loadMobileFrame(character.pictID, 0, nil)
		if visibleSprite == nil {
			return fmt.Errorf("load contact-shadow mobile pict %d", character.pictID)
		}
		drawContactShadow(canvas, shadowTestDrawSize, x, y, contactShadowOpacity, shadowDarkenBlend)
		op := &ebiten.DrawImageOptions{}
		op.Filter = ebiten.FilterLinear
		op.GeoM.Scale(float64(shadowTestDrawSize)/float64(visibleSprite.Bounds().Dx()), float64(shadowTestDrawSize)/float64(visibleSprite.Bounds().Dy()))
		op.GeoM.Translate(float64(x-shadowTestDrawSize/2), float64(y-shadowTestDrawSize/2))
		canvas.DrawImage(visibleSprite, op)
	}
	return writeShadowTestPNG(filepath.Join(g.outputDir, "shadow_contact.png"), canvas)
}

func (g *shadowRenderGame) renderWalkCycle(name string, pictID uint16) error {
	const azimuth = 90
	canvas := ebiten.NewImageFromImage(shadowTestBackground(shadowTestCanvasSize))
	for visibleState := uint8(0); visibleState < 4; visibleState++ {
		visibleSprite := loadMobileFrame(pictID, visibleState, nil)
		shadowState, casts := chooseUprightShadowPose(visibleState, azimuth)
		if visibleSprite == nil || !casts {
			return fmt.Errorf("load walk-cycle mobile pict %d state %d", pictID, visibleState)
		}
		shadowSprite := loadMobileFrame(pictID, shadowState, nil)
		if shadowSprite == nil {
			return fmt.Errorf("load walk-cycle shadow pict %d state %d", pictID, shadowState)
		}
		shadowTexture := characterShadowTextureFor(shadowSprite)
		x := 100 + int(visibleState)*200
		y := 440
		mask := ebiten.NewImage(shadowTestCanvasSize, shadowTestCanvasSize)
		drawCharacterShadow(mask, shadowTexture, shadowTestDrawSize, x, y, 0.75, newCharacterShadowProjection(azimuth), true, shadowMaskBlend)
		canvas.DrawImage(mask, &ebiten.DrawImageOptions{Blend: shadowDarkenBlend})

		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		op.GeoM.Scale(float64(shadowTestDrawSize)/float64(visibleSprite.Bounds().Dx()), float64(shadowTestDrawSize)/float64(visibleSprite.Bounds().Dy()))
		op.GeoM.Translate(float64(x-shadowTestDrawSize/2), float64(y-shadowTestDrawSize/2))
		canvas.DrawImage(visibleSprite, op)
	}
	return writeShadowTestPNG(filepath.Join(g.outputDir, fmt.Sprintf("shadow_%s_%d_walk_cycle.png", name, pictID)), canvas)
}

func verifyCharacterShadowSourceDeterminism() error {
	const pictID uint16 = 447
	const state uint8 = 0
	var reference []byte
	for attempt := 0; attempt < 6; attempt++ {
		clearCaches()
		start := make(chan struct{})
		var loaders sync.WaitGroup
		for worker := 0; worker < 4; worker++ {
			loaders.Add(1)
			go func() {
				defer loaders.Done()
				<-start
				loadSheet(pictID, nil, true)
			}()
		}
		close(start)
		// Exercise the production failure case: quality settings can clear
		// caches while background precache workers are decoding sheets.
		clearCaches()
		loaders.Wait()
		sprite := loadMobileFrame(pictID, state, nil)
		if sprite == nil {
			return fmt.Errorf("load deterministic shadow mobile pict %d", pictID)
		}
		texture := characterShadowTextureFor(sprite)
		pixels := make([]byte, 4*texture.image.Bounds().Dx()*texture.image.Bounds().Dy())
		texture.image.ReadPixels(pixels)
		if attempt == 0 {
			reference = pixels
			continue
		}
		if !bytes.Equal(reference, pixels) {
			return fmt.Errorf("character shadow changed after cache clear on attempt %d", attempt+1)
		}
	}
	return nil
}

func writeShadowTestPNG(path string, canvas *ebiten.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	pixels := make([]byte, 4*shadowTestCanvasSize*shadowTestCanvasSize)
	canvas.ReadPixels(pixels)
	result := &image.RGBA{
		Pix:    pixels,
		Stride: 4 * shadowTestCanvasSize,
		Rect:   image.Rect(0, 0, shadowTestCanvasSize, shadowTestCanvasSize),
	}
	if err := png.Encode(file, result); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func shadowTestBackground(size int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 188, G: 178, B: 148, A: 255}}, image.Point{}, draw.Src)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if x%32 == 0 || y%32 == 0 {
				img.SetRGBA(x, y, color.RGBA{R: 156, G: 148, B: 124, A: 255})
			}
		}
	}
	return img
}
