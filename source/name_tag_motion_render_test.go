package main

import (
	"bytes"
	"fmt"
	"image"
	"math"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// Run alone: exercise production interpolation, clipping, filtering and bars.
func TestRenderSmoothNameTagMotion(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_NAME_MOTION") == "" {
		t.Skip("set GOTHOOM_RENDER_NAME_MOTION=1")
	}
	oldSettings, oldImages := gs, clImages
	gs = cloneSettings(gsdef)
	gs.GameScale, gs.MotionSmoothing, gs.NameBgOpacity = 1, true, 1
	gs.HideSelfNameTag, gs.NameTagsOnHoverOnly, gs.NameTagLabelColors = false, false, false
	gs.NameHealthBarModern, gs.NameHealthBarAbove, gs.NameHealthBarThickness = true, true, 3
	clImages = nil
	clearSharedNameTagCache()
	t.Cleanup(func() { clearSharedNameTagCache(); gs, clImages = oldSettings, oldImages })
	g := &nameMotionGame{}
	ebiten.SetWindowVisible(false)
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type nameMotionGame struct {
	done bool
	err  error
}

func (*nameMotionGame) Layout(int, int) (int, int) { return 128, 80 }
func (g *nameMotionGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *nameMotionGame) Draw(*ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	g.err = g.verify()
}
func (g *nameMotionGame) verify() error {
	const width, height = 128, 80
	canvas := ebiten.NewImage(width, height)
	defer canvas.Deallocate()
	clip := canvas.RecyclableSubImage(image.Rect(7, 5, 120, 75))
	defer clip.Recycle()
	prev := frameMobile{Index: 1, H: int16(48 - fieldCenterX), V: int16(8 - fieldCenterY), Colors: uint8(kColorCodeBackBlack << 4)}
	cur := prev
	cur.H++
	desc := frameDescriptor{Index: 1, Type: kDescPlayer, Name: "Motion"}
	snap := drawSnapshot{prevMobiles: map[uint8]frameMobile{1: prev}, descriptors: map[uint8]frameDescriptor{1: desc}}
	key := makeNameTagKey(desc.Name, cur.Colors, desc.Type, 255, mobileNameStyle(cur.Colors, false), false, 1)
	// A transparent margin lets the alpha centroid measure subpixel text
	// movement independently of its separately drawn health bar.
	label := nameTagTargets.Acquire(16, 8, false)
	p := make([]byte, 16*8*4)
	for y := 1; y < 7; y++ {
		for x := 1; x < 15; x++ {
			i := (y*16 + x) * 4
			p[i], p[i+1], p[i+2], p[i+3] = 255, 255, 255, 255
		}
	}
	label.WritePixels(p)
	sharedNameTagCache[key] = &cachedNameTagImage{image: label, width: 16, height: 8, bytes: nameTagTargets.Bytes(label)}
	sharedNameTagBytes = nameTagTargets.Bytes(label)
	render := func(alpha float64) []byte {
		canvas.Clear()
		drawMobileNameTag(clip, snap, cur, alpha)
		pixels := make([]byte, width*height*4)
		canvas.ReadPixels(pixels)
		return pixels
	}
	centroid := func(pixels []byte, bar bool) float64 {
		var total, weighted float64
		for y := 0; y < height; y++ {
			if (bar && y != 34) || (!bar && y != 39) {
				continue
			}
			for x := 0; x < width; x++ {
				a := float64(pixels[(y*width+x)*4+3])
				total += a
				weighted += (float64(x) + 0.5) * a
			}
		}
		if total == 0 {
			return math.NaN()
		}
		return weighted / total
	}
	gs.SmoothNameTagMotion = false
	aligned := render(0)
	if !bytes.Equal(aligned, render(0.25)) {
		return fmt.Errorf("disabled smoothing moved a label within a pixel")
	}
	if bytes.Equal(aligned, render(1)) {
		return fmt.Errorf("disabled smoothing failed to move by a whole pixel")
	}
	gs.SmoothNameTagMotion = true
	start := render(0)
	lastDelta := [2]float64{}
	for _, alpha := range []float64{0.25, 0.5, 0.75, 1} {
		pixels := render(alpha)
		for i, bar := range []bool{false, true} {
			delta := centroid(pixels, bar) - centroid(start, bar)
			// Filtered pixel coverage can differ from the geometric phase, but
			// every quarter-step must advance, with a one-pixel total shift.
			if math.IsNaN(delta) || delta <= lastDelta[i] || math.Abs(delta-alpha) > 0.08 {
				return fmt.Errorf("bar=%v at %.2f: moved %.3f pixels after %.3f", bar, alpha, delta, lastDelta[i])
			}
			lastDelta[i] = delta
		}
	}

	// Moving the label must reuse its raster, without a cache entry per phase.
	if len(sharedNameTagCache) != 1 || sharedNameTagCache[key].image != label {
		return fmt.Errorf("fractional motion rebuilt the cached label")
	}
	return nil
}
