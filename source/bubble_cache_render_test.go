package main

import (
	"fmt"
	"image/color"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

func TestRenderCachedBubbleBodyMatchesDirectPaths(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_BUBBLE_CACHE_TEST") == "" {
		t.Skip("set GOTHOOM_RENDER_BUBBLE_CACHE_TEST=1 to verify cached bubble pixels")
	}
	game := &bubbleCacheRenderGame{}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

type bubbleCacheRenderGame struct {
	rendered bool
	err      error
}

func (g *bubbleCacheRenderGame) Update() error {
	if g.rendered {
		return ebiten.Termination
	}
	return nil
}

func (g *bubbleCacheRenderGame) Draw(_ *ebiten.Image) {
	const width, height = 72, 30
	const left, top = 11, 9
	const radius, stroke = float32(4), float32(1)
	fill := color.RGBA64{R: 0x9090, G: 0xb0b0, B: 0xd0d0, A: 0xc0c0}
	border := color.RGBA64{R: 0x2020, G: 0x3030, B: 0x4040, A: 0xffff}
	background := color.RGBA{R: 70, G: 90, B: 110, A: 255}
	direct := ebiten.NewImage(96, 52)
	direct.Fill(background)
	cached := ebiten.NewImage(96, 52)
	cached.Fill(background)

	body := bubbleRoundedRectPath(left, top, left+width, top+height, radius)
	fillOp := &vector.DrawPathOptions{AntiAlias: true}
	fillOp.ColorScale.ScaleWithColor(fill)
	vector.FillPath(direct, &body, nil, fillOp)
	strokeOp := &vector.StrokeOptions{Width: stroke}
	outlineOp := &vector.DrawPathOptions{AntiAlias: true}
	outlineOp.ColorScale.ScaleWithColor(border)
	vector.StrokePath(direct, &body, strokeOp, outlineOp)

	image, margin := cachedBubbleBodyImage(width, height, radius, stroke, fill, border)
	if image == nil {
		g.err = fmt.Errorf("cached bubble body was nil")
		g.rendered = true
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(left-margin), float64(top-margin))
	cached.DrawImage(image, op)

	directPixels := make([]byte, 96*52*4)
	cachedPixels := make([]byte, len(directPixels))
	direct.ReadPixels(directPixels)
	cached.ReadPixels(cachedPixels)
	for index := range directPixels {
		delta := int(directPixels[index]) - int(cachedPixels[index])
		if delta < -1 || delta > 1 {
			g.err = fmt.Errorf("cached bubble differs at byte %d: direct=%d cached=%d", index, directPixels[index], cachedPixels[index])
			break
		}
	}
	g.rendered = true
}

func (g *bubbleCacheRenderGame) Layout(_, _ int) (int, int) { return 96, 52 }
