package main

import (
	"image"
	"image/color"
	"math"
	"os"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestAdjustBubbleRectNoTail(t *testing.T) {
	sw, sh := 100, 100
	width, height := 20, 20
	tailHeight := 10
	x, y := 50, 90
	_, _, _, bottom := adjustBubbleRect(x, y, width, height, tailHeight, sw, sh, true)
	if bottom != y {
		t.Fatalf("expected bottom %d, got %d", y, bottom)
	}
}

func TestBubbleCompositeRegionIsBoundedToBubble(t *testing.T) {
	g := bubbleDrawGeometry{
		left: 100, top: 80, right: 260, bottom: 140,
		tailX: 180, tailY: 170, attachY: 140,
		scale: 1,
		request: bubbleDrawRequest{
			typ:         kBubbleThought,
			bubbleScale: 1,
			metrics:     bubbleMetrics{tailHalf: 6},
		},
	}
	region := bubbleCompositeRegion(g, true)
	if !region.In(image.Rect(0, 0, 400, 240)) {
		t.Fatalf("bounded region escaped test screen: %v", region)
	}
	if region.Dx() >= 400 || region.Dy() >= 240 {
		t.Fatalf("bubble region unexpectedly covers the full screen: %v", region)
	}
}

func TestThoughtBubbleMaskUsesMaximumAlpha(t *testing.T) {
	if thoughtBubbleMaskBlend.BlendOperationRGB != ebiten.BlendOperationMax {
		t.Fatal("thought-bubble mask does not combine color coverage by maximum")
	}
	if thoughtBubbleMaskBlend.BlendOperationAlpha != ebiten.BlendOperationMax {
		t.Fatal("thought-bubble mask does not combine alpha coverage by maximum")
	}
}

func TestRenderCompositeThoughtBubbleBackgroundOnlyTouchesRegion(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_BUBBLE_COMPOSITE_TEST") == "" {
		t.Skip("set GOTHOOM_RENDER_BUBBLE_COMPOSITE_TEST=1 to verify bubble-composite pixels")
	}
	game := &bubbleCompositeRenderGame{}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.inside.A == 0 {
		t.Fatalf("pixel inside composite region was not drawn: %v", game.inside)
	}
	if game.outside.A != 0 {
		t.Fatalf("pixel outside composite region was modified: %v", game.outside)
	}
}

type bubbleCompositeRenderGame struct {
	rendered bool
	inside   color.RGBA
	outside  color.RGBA
}

func (g *bubbleCompositeRenderGame) Update() error {
	if g.rendered {
		return ebiten.Termination
	}
	return nil
}

func (g *bubbleCompositeRenderGame) Draw(screen *ebiten.Image) {
	mask := ebiten.NewImage(80, 60)
	mask.Fill(color.White)
	compositeThoughtBubbleBackground(screen, mask, color.RGBA{R: 80, G: 120, B: 160, A: 255}, image.Rect(17, 11, 31, 29))
	pixels := make([]byte, 80*60*4)
	screen.ReadPixels(pixels)
	pixel := func(x, y int) color.RGBA {
		offset := (y*80 + x) * 4
		return color.RGBA{R: pixels[offset], G: pixels[offset+1], B: pixels[offset+2], A: pixels[offset+3]}
	}
	g.inside = pixel(20, 20)
	g.outside = pixel(5, 5)
	g.rendered = true
}

func (g *bubbleCompositeRenderGame) Layout(_, _ int) (int, int) { return 80, 60 }

func TestPonderBubbleAnimationAdvances(t *testing.T) {
	if got := ponderBubblePhase(250 * time.Millisecond); math.Abs(got-1) > 1e-9 {
		t.Fatalf("ponder animation phase after 250ms = %v, want 1", got)
	}
	if first, next := ponderWaveOffset(0, 0, 6), ponderWaveOffset(math.Pi/2, 0, 6); first == next {
		t.Fatalf("ponder wave offset did not change between phases: %v", first)
	}
}

func TestDecoratedBubbleOverlapMargins(t *testing.T) {
	const scale = 2.0
	tests := []struct {
		name string
		typ  int
		want int
	}{
		{name: "normal", typ: kBubbleNormal, want: 0},
		{name: "sunstone", typ: kBubbleThought, want: 6},
		{name: "ponder", typ: kBubblePonder, want: 21},
		{name: "yell", typ: kBubbleYell, want: 13},
		{name: "growl", typ: kBubbleMonster, want: 12},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := bubbleOverlapMargin(test.typ, scale); got != test.want {
				t.Fatalf("bubbleOverlapMargin(%d, %.1f) = %d, want %d", test.typ, scale, got, test.want)
			}
		})
	}
}

func TestBubbleAnimationCanBeDisabled(t *testing.T) {
	original := gs.AnimatedChatBubbles
	t.Cleanup(func() { gs.AnimatedChatBubbles = original })

	gs.AnimatedChatBubbles = false
	if phase := bubbleAnimationPhase(4); phase != 0 {
		t.Fatalf("disabled bubble animation phase = %v, want 0", phase)
	}
}
