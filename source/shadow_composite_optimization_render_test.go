package main

import (
	_ "embed"
	"fmt"
	"image"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed testdata/shaders/character_shadow_composite_before_blend.kage
var shadowCompositeBeforeBlend []byte

func TestShadowCompositeBlendPreservesPixels(t *testing.T) {
	if os.Getenv("GOTHOOM_SHADOW_BLEND_CHILD") == "" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(executable, "-test.run=^TestShadowCompositeBlendPreservesPixels$", "-test.v")
		cmd.Env = append(os.Environ(), "GOTHOOM_SHADOW_BLEND_CHILD=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("shadow blend comparison: %v\n%s", err, out)
		}
		t.Logf("%s", out)
		return
	}
	game := &shadowBlendComparisonGame{}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

type shadowBlendComparisonGame struct {
	done bool
	err  error
}

func (*shadowBlendComparisonGame) Layout(int, int) (int, int) { return 256, 256 }
func (g *shadowBlendComparisonGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *shadowBlendComparisonGame) Draw(*ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	g.err = compareShadowCompositeBlend()
}

func compareShadowCompositeBlend() error {
	before, err := ebiten.NewShader(shadowCompositeBeforeBlend)
	if err != nil {
		return err
	}
	defer before.Deallocate()
	after, err := ebiten.NewShader(layeredShadowCompositeShaderSource)
	if err != nil {
		return err
	}
	defer after.Deallocate()
	const size = 256
	bounds := image.Rect(17, 23, 17+size, 23+size)
	canvas := ebiten.NewImageWithOptions(bounds, &ebiten.NewImageOptions{Unmanaged: true})
	scene, previous, incoming := ebiten.NewImage(size, size), ebiten.NewImage(size, size), ebiten.NewImage(size, size)
	defer canvas.Deallocate()
	defer scene.Deallocate()
	defer previous.Deallocate()
	defer incoming.Deallocate()
	base, oldMask, newMask := make([]byte, size*size*4), make([]byte, size*size*4), make([]byte, size*size*4)
	want, got := make([]byte, len(base)), make([]byte, len(base))
	for y := range size {
		for x := range size {
			i := (y*size + x) * 4
			oldMask[i+3], newMask[i+3] = byte(x), byte(y)
		}
	}
	previous.WritePixels(oldMask)
	incoming.WritePixels(newMask)
	// Every pair of 8-bit coverage values, including fully opaque prior
	// coverage, against transparent, translucent and opaque scene colors.
	for _, alpha := range []byte{0, 1, 73, 128, 254, 255} {
		for i := 0; i < len(base); i += 4 {
			base[i], base[i+1], base[i+2], base[i+3] = byte((i/4)%(int(alpha)+1)), alpha/2, alpha, alpha
			if alpha == 255 {
				base[i] = byte(i / 4)
			}
		}
		scene.WritePixels(base)
		oldOp := &ebiten.DrawRectShaderOptions{Blend: ebiten.BlendCopy}
		oldOp.Images[0], oldOp.Images[1], oldOp.Images[2] = scene, previous, incoming
		oldOp.GeoM.Translate(float64(bounds.Min.X), float64(bounds.Min.Y))
		canvas.DrawRectShader(size, size, before, oldOp)
		canvas.ReadPixels(want)
		canvas.WritePixels(base)
		newOp := &ebiten.DrawRectShaderOptions{Blend: layeredShadowCompositeBlend}
		newOp.Images[0], newOp.Images[1] = previous, incoming
		newOp.GeoM = oldOp.GeoM
		canvas.DrawRectShader(size, size, after, newOp)
		canvas.ReadPixels(got)
		if err := compareShadowPixels(want, got, 1); err != nil {
			return fmt.Errorf("coverage matrix alpha=%d: %w", alpha, err)
		}
	}
	if os.Getenv("GOTHOOM_SHADOW_BLEND_TIMING") != "" {
		timeShadowCompositors(canvas, scene, previous, incoming, before, after, got)
	}

	layeredShadowCompositeShader = after
	defer func() {
		layeredShadowCompositeShader = nil
		clearCharacterShadowCache()
	}()
	// Exercise actual image submission, nonzero origins, cropped bounds,
	// opacity, flips, linear filtering and foreground coverage clearing.
	sourceBounds := image.Rect(31, 47, 95, 111)
	source := ebiten.NewImageWithOptions(sourceBounds, &ebiten.NewImageOptions{Unmanaged: true})
	defer source.Deallocate()
	pixels := make([]byte, 64*64*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i+3] = byte((i/4)*37 + 19)
		pixels[i], pixels[i+1], pixels[i+2] = pixels[i+3]/2, pixels[i+3]/3, pixels[i+3]
	}
	source.WritePixels(pixels)
	for variant := range 2 {
		canvas.WritePixels(base)
		beginLayeredCharacterShadowComposite(bounds)
		for step := range 9 {
			op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
			op.GeoM.Scale(1.4, 1.2)
			if step%2 != 0 {
				op.GeoM.Scale(-1, 1)
				op.GeoM.Translate(64*1.4, 0)
			}
			op.GeoM.Translate(float64(bounds.Min.X-18+step*13)+0.25, float64(bounds.Min.Y+step*11)+0.75)
			op.ColorScale.ScaleAlpha(float32(step+1) / 9)
			if step == 3 || step == 6 {
				canvas.DrawImage(source, op)
				clearLayeredShadowCoverageImage(source, op)
			}
			coverage := shadowCoverageReceiverBounds(source.Bounds(), op.GeoM)
			if variant == 0 {
				drawShadowCompositeReference(canvas, source, op, coverage, before)
			} else if !compositeLayeredShadowImage(canvas, source, op, coverage) {
				return fmt.Errorf("production shadow image was not composited")
			}
		}
		if variant == 0 {
			canvas.ReadPixels(want)
		} else {
			canvas.ReadPixels(got)
		}
	}
	// Repeated rounded blends can accumulate a second color step in this
	// nine-shadow sequence. Destination alpha still has to match exactly.
	return compareShadowPixels(want, got, 2)
}

func compareShadowPixels(want, got []byte, tolerance int) error {
	maxDelta, changed := 0, 0
	for i, expected := range want {
		delta := int(got[i]) - int(expected)
		if delta < 0 {
			delta = -delta
		}
		maxDelta = max(maxDelta, delta)
		if delta != 0 {
			changed++
		}
		// Fixed-function blending can round an 8-bit color one step differently
		// from fragment multiplication. Alpha must remain exactly unchanged.
		if i%4 == 3 && delta != 0 {
			return fmt.Errorf("channel %d = %d, want %d", i, got[i], expected)
		}
	}
	if maxDelta > tolerance {
		return fmt.Errorf("max color difference %d; %d of %d channels changed", maxDelta, changed, len(want))
	}
	return nil
}

func timeShadowCompositors(canvas, scene, previous, incoming *ebiten.Image, before, after *ebiten.Shader, pixels []byte) {
	bounds := canvas.Bounds()
	oldOp := &ebiten.DrawRectShaderOptions{Blend: ebiten.BlendCopy}
	oldOp.Images[0], oldOp.Images[1], oldOp.Images[2] = scene, previous, incoming
	oldOp.GeoM.Translate(float64(bounds.Min.X), float64(bounds.Min.Y))
	newOp := &ebiten.DrawRectShaderOptions{Blend: layeredShadowCompositeBlend}
	newOp.Images[0], newOp.Images[1] = previous, incoming
	newOp.GeoM = oldOp.GeoM
	copyOp := &ebiten.DrawImageOptions{Blend: ebiten.BlendCopy}
	updateOp := &ebiten.DrawImageOptions{Blend: shadowMaskBlend}
	draw := func(reference bool) {
		if reference {
			scene.DrawImage(canvas, copyOp)
			canvas.DrawRectShader(bounds.Dx(), bounds.Dy(), before, oldOp)
		} else {
			canvas.DrawRectShader(bounds.Dx(), bounds.Dy(), after, newOp)
		}
		// Match the production coverage update, including its render-target
		// switch, so the optimized batch cannot merge unrelated shadows.
		previous.DrawImage(incoming, updateOp)
	}
	for _, reference := range []bool{true, false} {
		draw(reference)
		canvas.ReadPixels(pixels)
	}
	const draws = 40
	for sample := range 3 {
		for turn := range 2 {
			reference := (sample+turn)%2 == 0
			start := time.Now()
			for range draws {
				draw(reference)
			}
			canvas.ReadPixels(pixels)
			fmt.Printf("shadow compositor reference=%v sample=%d synchronized-ns/draw=%d\n", reference, sample, time.Since(start).Nanoseconds()/draws)
		}
	}
}

func drawShadowCompositeReference(screen, source *ebiten.Image, op *ebiten.DrawImageOptions, bounds image.Rectangle, shader *ebiten.Shader) {
	bounds = bounds.Intersect(screen.Bounds())
	if bounds.Empty() {
		return
	}
	if !overlapsLayeredShadowCoverage(bounds) {
		compositeLayeredShadowImage(screen, source, op, bounds)
		return
	}
	size := bounds.Size()
	incoming, scene := ebiten.NewImage(size.X, size.Y), ebiten.NewImage(size.X, size.Y)
	defer incoming.Deallocate()
	defer scene.Deallocate()
	incomingOp := *op
	incomingOp.Blend = shadowMaskBlend
	incomingOp.GeoM.Translate(float64(-bounds.Min.X), float64(-bounds.Min.Y))
	incoming.DrawImage(source, &incomingOp)
	screenSource := screen.RecyclableSubImage(bounds)
	defer screenSource.Recycle()
	scene.DrawImage(screenSource, &ebiten.DrawImageOptions{Blend: ebiten.BlendCopy})
	coverageRect := bounds.Sub(layeredShadowOrigin)
	coverageSource := layeredShadowCoverage.RecyclableSubImage(coverageRect)
	defer coverageSource.Recycle()
	shaderOp := &ebiten.DrawRectShaderOptions{Blend: ebiten.BlendCopy}
	shaderOp.Images[0], shaderOp.Images[1], shaderOp.Images[2] = scene, coverageSource, incoming
	shaderOp.GeoM.Translate(float64(bounds.Min.X), float64(bounds.Min.Y))
	screen.DrawRectShader(size.X, size.Y, shader, shaderOp)
	updateOp := &ebiten.DrawImageOptions{Blend: shadowMaskBlend}
	updateOp.GeoM.Translate(float64(coverageRect.Min.X), float64(coverageRect.Min.Y))
	layeredShadowCoverage.DrawImage(incoming, updateOp)
	recordLayeredShadowCoverage(bounds)
}
