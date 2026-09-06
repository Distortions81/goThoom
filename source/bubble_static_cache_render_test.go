package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

func TestRenderStaticBubbleCache(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_STATIC_BUBBLES") == "" {
		t.Skip("set GOTHOOM_RENDER_STATIC_BUBBLES=1")
	}
	gs.AnimatedChatBubbles = false
	g := &staticBubbleCacheGame{}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type staticBubbleCacheGame struct {
	done bool
	err  error
}

func (g *staticBubbleCacheGame) Layout(_, _ int) (int, int) { return 360, 180 }
func (g *staticBubbleCacheGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *staticBubbleCacheGame) Draw(_ *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	defer clearBubbleTextCaches()
	gallery := ebiten.NewImage(720, 360)
	defer gallery.Deallocate()
	for k, kind := range []int{kBubbleNormal, kBubbleYell, kBubbleMonster, kBubblePonder, kBubbleThought, kBubbleWhisper} {
		for _, scale := range []float32{.75, 1, 1.5, 2} {
			for _, alpha := range []uint16{0x8080, 0xffff} {
				for _, gap := range []int{0, 1, 2} {
					fill := color.RGBA64{R: alpha / 2, G: uint16(uint32(alpha) * 3 / 4), B: alpha, A: alpha}
					border := color.RGBA64{R: alpha / 4, G: alpha / 3, B: alpha / 2, A: alpha}
					geometry := bubbleDrawGeometry{
						left: 45, top: 40, right: 315, bottom: 130, scale: scale, radius: 4 * scale, bubbleType: kind,
						fillColor: fill, borderColor: border,
						request: bubbleDrawRequest{typ: kind, bubbleScale: float64(scale), bgCol: fill, borderCol: border},
					}
					if gap != 0 {
						geometry.baseX, geometry.attachY = 180, geometry.bottom
						geometry.request.metrics.tailHalf = 4
						if gap == 1 {
							geometry.attachY = geometry.top
						}
					}
					if kind == kBubblePonder {
						geometry.radius = 8 * scale
					}
					direct, cached := ebiten.NewImage(360, 180), ebiten.NewImage(360, 180)
					background := color.RGBA{R: 40, G: 45, B: 50, A: 255}
					direct.Fill(background)
					cached.Fill(background)
					drawBubbleDecoration(direct, geometry, nil)
					previousMask := thoughtBubbleCompositeMask
					img, margin := cachedStaticBubbleDecoration(geometry)
					if previousMask != thoughtBubbleCompositeMask {
						g.err = fmt.Errorf("cache miss resized the world mask")
						return
					}
					if img == nil {
						g.err = fmt.Errorf("missing surface")
						return
					}
					op := &ebiten.DrawImageOptions{}
					op.GeoM.Translate(float64(geometry.left-margin), float64(geometry.top-margin))
					cached.DrawImage(img, op)
					moved := geometry
					moved.left += 3
					moved.right += 3
					moved.baseX += 3
					moved.request.txt = "another caption"
					reused, _ := cachedStaticBubbleDecoration(moved)
					if reused != img {
						g.err = fmt.Errorf("moving/changing caption rebuilt surface")
						return
					}
					a, b := make([]byte, 360*180*4), make([]byte, 360*180*4)
					direct.ReadPixels(a)
					cached.ReadPixels(b)
					largest, total := 0, 0
					for i := range a {
						delta := int(a[i]) - int(b[i])
						if delta < 0 {
							delta = -delta
						}
						largest = max(largest, delta)
						total += delta
					}
					if largest > 1 {
						g.err = fmt.Errorf("type %d scale %.2f alpha %d: max delta %d sum %d", kind, scale, alpha, largest, total)
						return
					}
					if k < 4 && scale == 1.5 && alpha == 0xffff {
						op := &ebiten.DrawImageOptions{}
						op.GeoM.Translate(float64(k%2*360), float64(k/2*180))
						gallery.DrawImage(cached, op)
					}
					direct.Deallocate()
					cached.Deallocate()
					if len(bubbleBodyImageCache) > maxBubbleBodyImages || bubbleBodyImageBytes > maxBubbleBodyImageBytes {
						g.err = fmt.Errorf("surface cache exceeded its limit")
						return
					}
				}
			}
		}
	}
	// Exercise entry-count and byte-budget eviction independently.
	clearBubbleTextCaches()
	for i := 0; i < maxBubbleBodyImages+8; i++ {
		img, _ := cacheBubbleSurface(bubbleBodyImageCacheKey{width: 16, height: 16, decorationType: i}, 0, func(*ebiten.Image) {})
		if img == nil || len(bubbleBodyImageCache) > maxBubbleBodyImages || bubbleBodyImageBytes > maxBubbleBodyImageBytes {
			g.err = fmt.Errorf("entry budget failed")
			return
		}
	}
	clearBubbleTextCaches()
	for i := 0; i < 4; i++ {
		img, _ := cacheBubbleSurface(bubbleBodyImageCacheKey{width: 2048, height: 1024, decorationType: i}, 0, func(*ebiten.Image) {})
		if img == nil || bubbleBodyImageBytes > maxBubbleBodyImageBytes {
			g.err = fmt.Errorf("byte budget failed")
			return
		}
	}
	if img, _ := cacheBubbleSurface(bubbleBodyImageCacheKey{width: 8192, height: 8192}, 0, func(*ebiten.Image) { panic("oversized cache painted") }); img != nil {
		g.err = fmt.Errorf("oversized surface cached")
		return
	}
	clearBubbleTextCaches()
	if path := os.Getenv("GOTHOOM_BUBBLE_APPEARANCE_IMAGE"); path != "" {
		if g.err = renderBubbleAppearanceGallery(path); g.err != nil {
			return
		}
	}
	if path := os.Getenv("GOTHOOM_STATIC_BUBBLE_IMAGE"); path != "" {
		pixels := image.NewRGBA(gallery.Bounds())
		gallery.ReadPixels(pixels.Pix)
		f, err := os.Create(path)
		if err != nil {
			g.err = err
			return
		}
		g.err = png.Encode(f, pixels)
		_ = f.Close()
	}
}

// Paired light/dark samples use the actual layout, text, tail, and body paths.
func renderBubbleAppearanceGallery(path string) error {
	initFont()
	oldDark, oldOpacity := gs.DarkBubblesAndNames, gs.BubbleOpacity
	defer func() { gs.DarkBubblesAndNames = oldDark; gs.BubbleOpacity = oldOpacity }()
	gs.BubbleOpacity = 1
	canvas := ebiten.NewImage(760, 900)
	defer canvas.Deallocate()
	examples := []struct {
		typ           int
		name, message string
	}{
		{kBubbleNormal, "Say", "Welcome to Puddleby!"},
		{kBubbleWhisper, "Whisper", "A quiet word."},
		{kBubbleYell, "Yell", "Look out!"},
		{kBubbleMonster, "Growl", "Grrr!"},
		{kBubblePonder, "Ponder", "Where did I leave it?"},
		{kBubbleThought, "Think", "I wonder..."},
		{kBubbleRealAction, "Action", "Waves hello."},
		{kBubblePlayerAction, "Player action", "Takes a bow."},
		{kBubbleNarrate, "Narration", "The sun begins to set."},
	}
	for column, dark := range []bool{false, true} {
		gs.DarkBubblesAndNames = dark
		for row, example := range examples {
			cell := canvas.SubImage(image.Rect(column*380, row*100, (column+1)*380, (row+1)*100)).(*ebiten.Image)
			cell.Fill(color.RGBA{R: 65, G: 76, B: 67, A: 255})
			border, bg, caption := bubbleColors(example.typ)
			metrics := measureBubble(example.message, example.typ, 1.25, .75, image.Pt(320, 65))
			request := bubbleDrawRequest{txt: example.message, x: 190, y: 90, typ: example.typ, placement: bubblePosNone, borderCol: border, bgCol: bg, textCol: caption, bubbleScale: 1.25, metrics: metrics}
			drawBubbleTail(cell, request)
			drawBubbleBody(cell, request)
			face := *mainFont.(*text.GoTextFace)
			face.Size = 12
			op := &text.DrawOptions{}
			op.GeoM.Translate(float64(column*380+10), float64(row*100+3))
			op.ColorScale.ScaleWithColor(color.White)
			mode := "Light"
			if dark {
				mode = "Dark"
			}
			text.Draw(cell, mode+" / "+example.name, &face, op)
		}
	}
	pixels := image.NewRGBA(canvas.Bounds())
	canvas.ReadPixels(pixels.Pix)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, pixels)
}
