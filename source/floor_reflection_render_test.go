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

func TestRenderFloorReflectionTilesWithoutSeams(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_FLOOR_REFLECTION_TEST") == "" {
		t.Skip("set GOTHOOM_RENDER_FLOOR_REFLECTION_TEST=1 to verify reflection pixels")
	}
	if err := ReloadSpriteUpscaleShader(); err != nil {
		t.Fatalf("compile mobile recolor shader: %v", err)
	}
	game := &floorReflectionRenderGame{}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

type floorReflectionRenderGame struct {
	done bool
	err  error
}

func (g *floorReflectionRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}

func (g *floorReflectionRenderGame) Draw(_ *ebiten.Image) {
	defer func() { g.done = true }()
	sprite := ebiten.NewImage(2, 4)
	defer sprite.Deallocate()
	pixels := make([]byte, 2*4*4)
	for y := 0; y < 4; y++ {
		for x := 0; x < 2; x++ {
			i := (y*2 + x) * 4
			if y < 2 {
				pixels[i] = 255 // The source's head is red.
			} else {
				pixels[i+2] = 255 // Its feet are blue.
			}
			pixels[i+3] = 255
		}
	}
	sprite.WritePixels(pixels)
	pose := mobileReflectionPose{image: sprite, footRow: 1}
	options := frameBlendDrawOptions{
		Left: 3, Top: 6, ScaleX: 1, ScaleY: -1,
		Red: 0.55, Green: 0.76, Blue: 0.90, Alpha: 0.32,
	}
	const width, height = 8, 8
	background := color.RGBA{R: 50, G: 50, B: 50, A: 255}
	tiled, whole := ebiten.NewImage(width, height), ebiten.NewImage(width, height)
	defer tiled.Deallocate()
	defer whole.Deallocate()
	tiled.Fill(background)
	whole.Fill(background)
	for _, bounds := range []image.Rectangle{image.Rect(0, 0, 4, 8), image.Rect(4, 0, 8, 8)} {
		clip := tiled.RecyclableSubImage(bounds)
		drawMobileReflectionSprite(clip, pose, options)
		clip.Recycle()
	}
	drawMobileReflectionSprite(whole, pose, options)
	a, b := make([]byte, width*height*4), make([]byte, width*height*4)
	tiled.ReadPixels(a)
	whole.ReadPixels(b)
	if !bytes.Equal(a, b) {
		g.err = fmt.Errorf("reflection changed across adjacent tile clips")
		return
	}
	// An upside-down reflection puts the source feet nearest their upright
	// contact point and its head farther into the floor, at lower screen Y.
	nearFoot := (3*width + 3) * 4
	farHead := (5*width + 3) * 4
	if a[nearFoot+2] <= a[nearFoot] || a[farHead] <= a[farHead+2] {
		g.err = fmt.Errorf("reflected mobile was not vertically mirrored: feet=%v head=%v", a[nearFoot:nearFoot+4], a[farHead:farHead+4])
		return
	}
	if a[nearFoot+3] != 255 || a[nearFoot+2] <= background.B || a[nearFoot+2] >= 255 {
		g.err = fmt.Errorf("mobile reflection is not translucent: %v", a[nearFoot:nearFoot+4])
		return
	}
	contact := (2*width + 3) * 4
	aboveContact := (width + 3) * 4
	if a[contact+2] >= a[nearFoot+2] || !bytes.Equal(a[aboveContact:aboveContact+4], []byte{background.R, background.G, background.B, background.A}) {
		g.err = fmt.Errorf("reflection did not soften and crop at the feet: above=%v contact=%v near=%v", a[aboveContact:aboveContact+4], a[contact:contact+4], a[nearFoot:nearFoot+4])
		return
	}
	// Palette-recolored mobiles take the shader path, but must retain the same
	// clipped silhouette and gentle join at the feet.
	influence := ebiten.NewImage(2, 4)
	defer influence.Deallocate()
	influence.Fill(color.RGBA{R: 1, A: 255})
	palette := testMobilePalette(makeMobileKey(1, 0, []byte{1}), 0, 0, 0, 0)
	pose.influence, pose.palette, pose.gpuRecolor = influence, palette, true
	gpu := ebiten.NewImage(width, height)
	defer gpu.Deallocate()
	gpu.Fill(background)
	drawMobileReflectionSprite(gpu, pose, options)
	c := make([]byte, width*height*4)
	gpu.ReadPixels(c)
	if c[nearFoot+2] <= background.B || c[contact+2] >= c[nearFoot+2] || !bytes.Equal(c[aboveContact:aboveContact+4], []byte{background.R, background.G, background.B, background.A}) {
		g.err = fmt.Errorf("recolored reflection did not soften and crop at the feet: above=%v contact=%v near=%v", c[aboveContact:aboveContact+4], c[contact:contact+4], c[nearFoot:nearFoot+4])
		return
	}
	stripe := ebiten.NewImage(8, 8)
	defer stripe.Deallocate()
	stripePixels := make([]byte, 8*8*4)
	for y := 0; y < 8; y++ {
		for x := 3; x <= 4; x++ {
			i := (y*8 + x) * 4
			stripePixels[i], stripePixels[i+1], stripePixels[i+2], stripePixels[i+3] = 255, 255, 255, 255
		}
	}
	stripe.WritePixels(stripePixels)
	stripeInfluence := ebiten.NewImage(8, 8)
	defer stripeInfluence.Deallocate()
	stripeInfluence.Fill(color.RGBA{R: 1, A: 255})
	for _, recolored := range []bool{false, true} {
		var far, near [2]float64
		for facingIndex, state := range []uint8{0, 16} { // east and west
			pose := mobileReflectionPose{image: stripe, footRow: 1, state: state, gpuRecolor: recolored}
			if recolored {
				pose.influence, pose.palette = stripeInfluence, palette
			}
			target := ebiten.NewImage(32, 24)
			drawMobileReflectionSprite(target, pose, frameBlendDrawOptions{
				Left: 4, Top: 20, ScaleX: 3, ScaleY: -2,
				Red: 1, Green: 1, Blue: 1, Alpha: 1,
			})
			pixels := make([]byte, 32*24*4)
			target.ReadPixels(pixels)
			target.Deallocate()
			var ok bool
			far[facingIndex], ok = reflectionRowCentroid(pixels, 32, 18)
			if !ok {
				g.err = fmt.Errorf("pose-skew reflection lost its upper body (recolored=%v, pose=%d)", recolored, state)
				return
			}
			near[facingIndex], ok = reflectionRowCentroid(pixels, 32, 6)
			if !ok {
				g.err = fmt.Errorf("pose-skew reflection lost its foot fade (recolored=%v, pose=%d)", recolored, state)
				return
			}
		}
		if far[0] <= far[1]+1 || near[0] >= near[1]+1 {
			g.err = fmt.Errorf("pose skew did not lean the upper body while anchoring feet (recolored=%v, far=%v, near=%v)", recolored, far, near)
			return
		}
	}
}

func (g *floorReflectionRenderGame) Layout(_, _ int) (int, int) { return 8, 8 }

func reflectionRowCentroid(pixels []byte, width, row int) (float64, bool) {
	var weightedX, totalAlpha float64
	for x := 0; x < width; x++ {
		alpha := float64(pixels[(row*width+x)*4+3])
		weightedX += float64(x) * alpha
		totalAlpha += alpha
	}
	if totalAlpha == 0 {
		return 0, false
	}
	return weightedX / totalAlpha, true
}
