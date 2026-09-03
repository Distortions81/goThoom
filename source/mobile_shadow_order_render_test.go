package main

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestRenderLayeredCharacterShadowOrder(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_SHADOW_ORDER_TEST") == "" {
		t.Skip("set GOTHOOM_RENDER_SHADOW_ORDER_TEST=1 to verify shadow painter order")
	}
	originalSettings := gs
	gs = gsdef
	gs.GameScale = 1
	gs.ShadersEnabled = true
	gs.FasterCharacterShadows = false
	gs.CharacterShadowDarkness = 1
	if err := ReloadLightingShader(); err != nil {
		t.Fatalf("compile layered shadow shader: %v", err)
	}
	t.Cleanup(func() {
		gs = originalSettings
		resetLayeredCharacterShadows()
	})

	game := &shadowOrderRenderGame{}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

type shadowOrderRenderGame struct {
	rendered bool
	err      error
}

func (g *shadowOrderRenderGame) Update() error {
	if g.rendered {
		return ebiten.Termination
	}
	return nil
}

func (g *shadowOrderRenderGame) Draw(_ *ebiten.Image) {
	canvas := ebiten.NewImage(16, 8)
	canvas.Fill(color.RGBA{R: 200, G: 200, B: 200, A: 255})
	shadowTexture := ebiten.NewImage(8, 8)
	shadowTexture.Fill(color.White)
	texture := characterShadowTexture{image: shadowTexture, contentSize: 8, footY: 8}
	projection := characterShadowProjection{contrast: 1}
	command := characterShadowDraw{
		texture:    texture,
		size:       8,
		x:          8,
		y:          4,
		alpha:      1,
		projection: projection,
		quad:       mobileSunShadowQuad(texture, 8, 8, 4, projection, false),
	}
	beginLayeredCharacterShadowComposite(canvas.Bounds())
	queueLayeredCharacterShadow(3, command)

	// The shadow is submitted at its caster's position in painter order. A
	// foreground object submitted afterward must retain its original color.
	drawLayeredCharacterShadow(canvas, 3)
	foreground := ebiten.NewImage(4, 8)
	foreground.Fill(color.RGBA{R: 240, G: 40, B: 20, A: 255})
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(8, 0)
	canvas.DrawImage(foreground, op)

	pixels := make([]byte, 16*8*4)
	canvas.ReadPixels(pixels)
	shadowed := shadowOrderPixel(pixels, 16, 6, 4)
	if shadowed.R >= 200 || shadowed.G >= 200 || shadowed.B >= 200 {
		g.err = fmt.Errorf("ground pixel was not darkened by layered shadow: %#v", shadowed)
	} else if got := shadowOrderPixel(pixels, 16, 10, 4); got != (color.RGBA{R: 240, G: 40, B: 20, A: 255}) {
		g.err = fmt.Errorf("later foreground pixel was darkened by earlier shadow: %#v", got)
	}
	if g.err == nil {
		g.err = verifyLayeredShadowMaximum(command)
	}
	if g.err == nil {
		g.err = verifyLayeredShadowOffsetCoordinates(texture, projection)
	}
	if g.err == nil {
		g.err = verifyLayeredPictureShadowMaximum(texture, projection)
	}
	g.rendered = true
}

func verifyLayeredShadowMaximum(command characterShadowDraw) error {
	canvas := ebiten.NewImage(16, 8)
	canvas.Fill(color.RGBA{R: 200, G: 200, B: 200, A: 255})
	beginLayeredCharacterShadowComposite(canvas.Bounds())
	queueLayeredCharacterShadow(1, command)
	drawLayeredCharacterShadow(canvas, 1)
	first := make([]byte, 16*8*4)
	canvas.ReadPixels(first)
	firstGround := shadowOrderPixel(first, 16, 6, 4)

	// A newly painted receiver clears the prior coverage only under its own
	// alpha. The later shadow should affect it, while the still-visible ground
	// must remain at the existing maximum darkness.
	foreground := ebiten.NewImage(4, 8)
	foreground.Fill(color.RGBA{R: 240, G: 40, B: 20, A: 255})
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(8, 0)
	canvas.DrawImage(foreground, op)
	clearLayeredShadowCoverageImage(foreground, op)
	queueLayeredCharacterShadow(2, command)
	drawLayeredCharacterShadow(canvas, 2)
	second := make([]byte, 16*8*4)
	canvas.ReadPixels(second)
	secondGround := shadowOrderPixel(second, 16, 6, 4)
	if !shadowOrderColorNear(firstGround, secondGround) {
		return fmt.Errorf("overlapping shadows multiplied ground darkness: first %#v, second %#v", firstGround, secondGround)
	}
	if receiver := shadowOrderPixel(second, 16, 10, 4); receiver.R >= 240 || receiver.G >= 40 || receiver.B >= 20 {
		return fmt.Errorf("later shadow did not darken newly painted receiver: %#v", receiver)
	}
	return nil
}

func verifyLayeredShadowOffsetCoordinates(texture characterShadowTexture, projection characterShadowProjection) error {
	const originX, originY = 5, 3
	backing := ebiten.NewImage(24, 14)
	canvas := backing.SubImage(image.Rect(originX, originY, originX+16, originY+8)).(*ebiten.Image)
	canvas.Fill(color.RGBA{R: 200, G: 200, B: 200, A: 255})
	command := characterShadowDraw{
		texture:    texture,
		size:       8,
		x:          originX + 8,
		y:          originY + 4,
		alpha:      1,
		projection: projection,
	}
	command.quad = mobileSunShadowQuad(texture, command.size, command.x, command.y, projection, false)
	beginLayeredCharacterShadowComposite(canvas.Bounds())
	queueLayeredCharacterShadow(4, command)
	drawLayeredCharacterShadow(canvas, 4)
	first := make([]byte, 16*8*4)
	canvas.ReadPixels(first)
	firstGround := shadowOrderPixel(first, 16, 6, 4)
	queueLayeredCharacterShadow(5, command)
	drawLayeredCharacterShadow(canvas, 5)
	second := make([]byte, 16*8*4)
	canvas.ReadPixels(second)
	if secondGround := shadowOrderPixel(second, 16, 6, 4); !shadowOrderColorNear(firstGround, secondGround) {
		return fmt.Errorf("offset-origin shadows multiplied darkness: first %#v, second %#v", firstGround, secondGround)
	}
	foreground := ebiten.NewImage(4, 8)
	foreground.Fill(color.RGBA{R: 240, G: 40, B: 20, A: 255})
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(originX+8, originY)
	canvas.DrawImage(foreground, op)
	pixels := make([]byte, 16*8*4)
	canvas.ReadPixels(pixels)
	if got := shadowOrderPixel(pixels, 16, 10, 4); got != (color.RGBA{R: 240, G: 40, B: 20, A: 255}) {
		return fmt.Errorf("offset-origin foreground was not drawn over its shadow: %#v", got)
	}
	return nil
}

func verifyLayeredPictureShadowMaximum(texture characterShadowTexture, projection characterShadowProjection) error {
	canvas := ebiten.NewImage(12, 8)
	canvas.Fill(color.RGBA{R: 200, G: 200, B: 200, A: 255})
	shadowPicture := ebiten.NewImage(8, 8)
	shadowPicture.Fill(color.White)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(2, 0)
	op.ColorScale.Scale(0, 0, 0, 0.5)
	bounds := image.Rect(2, 0, 10, 8)
	beginLayeredCharacterShadowComposite(canvas.Bounds())
	if !compositeLayeredShadowImage(canvas, shadowPicture, op, bounds) {
		return fmt.Errorf("explicit picture shadow did not use layered compositor")
	}
	first := make([]byte, 12*8*4)
	canvas.ReadPixels(first)
	firstGround := shadowOrderPixel(first, 12, 6, 4)
	if firstGround.R >= 200 || firstGround.G >= 200 || firstGround.B >= 200 {
		return fmt.Errorf("explicit picture shadow did not darken ground: %#v", firstGround)
	}

	if !compositeLayeredShadowImage(canvas, shadowPicture, op, bounds) {
		return fmt.Errorf("second explicit picture shadow did not use layered compositor")
	}
	second := make([]byte, 12*8*4)
	canvas.ReadPixels(second)
	if secondGround := shadowOrderPixel(second, 12, 6, 4); !shadowOrderColorNear(firstGround, secondGround) {
		return fmt.Errorf("overlapping picture shadows multiplied darkness: first %#v, second %#v", firstGround, secondGround)
	}

	// Character shadow opacity includes the detailed-core multiplier. Choose
	// its input alpha so its final 50% coverage matches the picture shadow.
	command := characterShadowDraw{
		texture: texture,
		size:    8,
		x:       6,
		y:       4,
		alpha:   2.0 / 3.0,
		projection: characterShadowProjection{
			contrast: projection.contrast,
		},
	}
	command.quad = mobileSunShadowQuad(texture, command.size, command.x, command.y, command.projection, false)
	compositeLayeredCharacterShadow(canvas, command)
	third := make([]byte, 12*8*4)
	canvas.ReadPixels(third)
	if thirdGround := shadowOrderPixel(third, 12, 6, 4); !shadowOrderColorNear(firstGround, thirdGround) {
		return fmt.Errorf("picture and character shadows multiplied darkness: picture %#v, combined %#v", firstGround, thirdGround)
	}
	return nil
}

func (g *shadowOrderRenderGame) Layout(_, _ int) (int, int) { return 16, 8 }

func shadowOrderPixel(pixels []byte, width, x, y int) color.RGBA {
	offset := (y*width + x) * 4
	return color.RGBA{R: pixels[offset], G: pixels[offset+1], B: pixels[offset+2], A: pixels[offset+3]}
}

func shadowOrderColorNear(a, b color.RGBA) bool {
	for _, pair := range [][2]uint8{{a.R, b.R}, {a.G, b.G}, {a.B, b.B}, {a.A, b.A}} {
		delta := int(pair[0]) - int(pair[1])
		if delta < -1 || delta > 1 {
			return false
		}
	}
	return true
}
