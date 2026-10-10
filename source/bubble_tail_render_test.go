package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"os/exec"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

func TestBubbleTailPixelsAndAllocations(t *testing.T) {
	if os.Getenv("GOTHOOM_BUBBLE_TAIL_CHILD") == "" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(executable, "-test.run=^TestBubbleTailPixelsAndAllocations$", "-test.v")
		cmd.Env = append(os.Environ(), "GOTHOOM_BUBBLE_TAIL_CHILD=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("bubble tail comparison: %v\n%s", err, out)
		}
		t.Logf("%s", out)
		return
	}
	gs.AnimatedChatBubbles = false
	game := &bubbleTailComparisonGame{t: t}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
	if !game.done {
		t.Fatal("bubble tail comparison did not run")
	}
}

type bubbleTailComparisonGame struct {
	t    *testing.T
	done bool
	err  error
}

func (*bubbleTailComparisonGame) Layout(int, int) (int, int) { return 240, 180 }
func (g *bubbleTailComparisonGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *bubbleTailComparisonGame) Draw(*ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	defer clearThoughtBubbleMask()
	g.err = compareBubbleTailPixels(g.t)
}

func bubbleTailTestRequest(kind int, scale float64, index int) bubbleDrawRequest {
	alpha := [...]uint16{0xffff, 0x8888, 0}[index%3]
	return bubbleDrawRequest{
		txt: "tail", typ: kind, bubbleScale: scale,
		x: 35 + index*19, y: 110 + index%3*7,
		placement: uint8(index % 5),
		metrics:   bubbleMetrics{width: 64, height: 28, tailHeight: 20, tailHalf: max(1, int(3*scale))},
		bgCol:     color.RGBA64{R: alpha / 2, G: alpha / 3, B: alpha / 4, A: alpha},
		borderCol: color.RGBA64{R: alpha / 8, G: alpha / 8, B: alpha / 8, A: alpha},
	}
}

func drawBubbleTailTestAtPhase(screen *ebiten.Image, request bubbleDrawRequest, phase float64) {
	g, ok := prepareBubbleDraw(screen, request)
	if ok && !g.request.far && !g.noArrow {
		drawPreparedBubbleTail(screen, g, phase)
	}
}

func compareBubbleTailPixels(t *testing.T) error {
	kinds := []int{kBubbleNormal, kBubbleWhisper, kBubbleYell, kBubbleMonster, kBubbleThought, kBubblePonder}
	bounds := image.Rect(17, 23, 257, 203)
	canvas := ebiten.NewImageWithOptions(bounds, &ebiten.NewImageOptions{Unmanaged: true})
	defer canvas.Deallocate()
	reference, current := make([]byte, 240*180*4), make([]byte, 240*180*4)
	for _, scale := range []float64{0.5, 1, 1.75, 2.5} {
		for _, kind := range append(kinds, -1) {
			for _, phase := range []float64{0, 0.8, 3.6} {
				render := func(draw func(*ebiten.Image, bubbleDrawRequest, float64), pixels []byte) {
					canvas.Clear()
					for i := range 12 {
						requestKind := kind
						if requestKind == -1 {
							requestKind = kinds[i%len(kinds)]
						}
						request := bubbleTailTestRequest(requestKind, scale, i)
						// Queue differently positioned tails before readback so
						// reuse cannot overwrite geometry still waiting to draw.
						if i%2 != 0 {
							request.hasTailAnchor = true
							request.tailAnchor = image.Pt(20+i*23, 145-i*9)
						}
						if i == 9 {
							request.far = true
						}
						if i == 10 {
							request.noArrow = true
						}
						if i == 11 {
							request.txt = ""
						}
						draw(canvas, request, phase)
					}
					canvas.ReadPixels(pixels)
				}
				render(drawBubbleTailReferenceAtPhase, reference)
				render(drawBubbleTailTestAtPhase, current)
				if !bytes.Equal(reference, current) {
					return fmt.Errorf("tail type %d scale %.2f phase %.2f changed pixels", kind, scale, phase)
				}
			}
		}
	}
	for _, kind := range []int{kBubbleNormal, kBubblePonder} {
		request := bubbleTailTestRequest(kind, 1.25, 3)
		var allocations [2]float64
		for index, variant := range []struct {
			name string
			draw func(*ebiten.Image, bubbleDrawRequest)
		}{{"reference", drawBubbleTailReference}, {"current", drawBubbleTail}} {
			draw := func() {
				canvas.Clear()
				variant.draw(canvas, request)
				canvas.ReadPixels(current)
			}
			draw()
			allocs := testing.AllocsPerRun(100, draw)
			allocations[index] = allocs
			t.Logf("kind=%d %s tail including readback: %.1f allocs/op", kind, variant.name, allocs)
		}
		// Driver/readback allocations vary by backend, but path reuse should
		// eliminate at least the two small path allocations on every draw.
		if allocations[1]+2 > allocations[0] {
			return fmt.Errorf("tail type %d did not reduce allocations: %v", kind, allocations)
		}
	}
	return nil
}

// Reference preserves the original independently allocated tail and side paths.
func drawBubbleTailReference(screen *ebiten.Image, request bubbleDrawRequest) {
	drawBubbleTailReferenceAtPhase(screen, request, bubbleAnimationPhase(ponderBubbleAnimationSpeed))
}

func drawBubbleTailReferenceAtPhase(screen *ebiten.Image, request bubbleDrawRequest, ponderPhase float64) {
	g, ok := prepareBubbleDraw(screen, request)
	if !ok || g.request.far || g.noArrow {
		return
	}
	fx, fy := float32(g.offsetX), float32(g.offsetY)
	tailHalf := g.request.metrics.tailHalf
	var tail vector.Path
	if g.bubbleType == kBubblePonder {
		r1 := float32(tailHalf)
		offset1 := r1 * 0.3 * float32(math.Sin(ponderPhase))
		cx1 := float32(g.baseX) + fx
		cy1 := float32(g.attachY) + float32(g.tailY-g.attachY)*0.25 - offset1 + fy
		tail.MoveTo(cx1+r1, cy1)
		tail.Arc(cx1, cy1, r1, 0, 2*math.Pi, vector.Clockwise)
		tail.Close()
		rMid := r1 * 0.6
		offsetMid := rMid * 0.5 * float32(math.Sin(ponderPhase+math.Pi/4))
		cxMid := float32(g.baseX+g.tailX)/2 + fx
		cyMid := float32(g.attachY) + float32(g.tailY-g.attachY)*0.65 - offsetMid + fy
		tail.MoveTo(cxMid+rMid, cyMid)
		tail.Arc(cxMid, cyMid, rMid, 0, 2*math.Pi, vector.Clockwise)
		tail.Close()
		r2 := float32(tailHalf) / 2
		offset2 := r2 * 0.6 * float32(math.Sin(ponderPhase+math.Pi/2))
		cx2 := float32(g.tailX) + fx
		cy2 := float32(g.tailY) - offset2 + fy
		tail.MoveTo(cx2+r2, cy2)
		tail.Arc(cx2, cy2, r2, 0, 2*math.Pi, vector.Clockwise)
		tail.Close()
	} else {
		tail.MoveTo(float32(g.baseX-tailHalf)+fx, float32(g.attachY)+fy)
		tail.LineTo(float32(g.tailX)+fx, float32(g.tailY)+fy)
		tail.LineTo(float32(g.baseX+tailHalf)+fx, float32(g.attachY)+fy)
		tail.Close()
	}

	compositeRegion := bubbleCompositeRegion(g, true).Intersect(screen.Bounds())
	backgroundTarget, fillColor, backgroundBlend, compositeThought := bubbleBackgroundTarget(screen, g.bubbleType, g.fillColor, compositeRegion)
	tailOp := &vector.DrawPathOptions{AntiAlias: true, Blend: backgroundBlend}
	tailOp.ColorScale.ScaleWithColor(fillColor)
	vector.FillPath(backgroundTarget, &tail, nil, tailOp)
	if g.bubbleType != kBubblePonder && g.bubbleType != kBubbleThought {
		strokeOp := &vector.StrokeOptions{Width: max(1, g.scale)}
		drawOp := &vector.DrawPathOptions{AntiAlias: true}
		drawOp.ColorScale.ScaleWithColor(g.borderColor)
		var sides vector.Path
		sides.MoveTo(float32(g.baseX-tailHalf)+fx, float32(g.attachY)+fy)
		sides.LineTo(float32(g.tailX)+fx, float32(g.tailY)+fy)
		sides.LineTo(float32(g.baseX+tailHalf)+fx, float32(g.attachY)+fy)
		vector.StrokePath(screen, &sides, strokeOp, drawOp)
	}
	if compositeThought {
		compositeThoughtBubbleBackground(screen, backgroundTarget, g.request.bgCol, compositeRegion)
	}
}
