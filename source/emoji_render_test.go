package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"gothoom/eui"
)

func TestRenderEmojiChatAndBubble(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_EMOJI") == "" {
		t.Skip("set GOTHOOM_RENDER_EMOJI=1; run alone")
	}
	if err := eui.Init(); err != nil {
		t.Fatal(err)
	}
	initFont()
	g := &emojiRenderGame{}
	ebiten.SetWindowVisible(false)
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type emojiRenderGame struct {
	done bool
	err  error
}

func (*emojiRenderGame) Layout(int, int) (int, int) { return 640, 220 }
func (g *emojiRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *emojiRenderGame) Draw(*ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	g.err = g.verify()
}
func (*emojiRenderGame) verify() error {
	canvas := ebiten.NewImage(640, 220)
	defer canvas.Deallocate()
	canvas.Fill(color.White)
	oldChat := chatWin
	win, list, _ := newTextWindow("Chat", eui.HZoneLeft, eui.VZoneTop, false, nil)
	chatWin = win
	defer func() { win.RemoveWindow(); chatWin = oldChat }()
	message := formatTimedMessage(timedMessage{Text: "Self: :smile: :heart: :woman_technologist: :rocket:"}, "", false)
	var cache textWindowWrapCache
	updateTextWindow(win, list, nil, []string{message}, 22, "", nil, false, &cache)
	face := list.Contents[0].Face
	for _, emoji := range []string{"😄", "❤️", "👩‍💻", "🚀"} {
		glyphs := text.AppendGlyphs(nil, emoji, face, nil)
		colored := 0
		for _, glyph := range glyphs {
			if glyph.Colored && glyph.Image != nil {
				colored++
			}
		}
		if colored != 1 {
			return fmt.Errorf("%s produced %d colored glyphs, want one composed emoji", emoji, colored)
		}
	}
	op := &text.DrawOptions{}
	op.ColorScale.ScaleWithColor(color.Black)
	op.GeoM.Translate(20, 10)
	text.Draw(canvas, message, face, op)
	bubbleMessage := "Another exile: :smile: :heart: :woman_technologist:"
	m := measureBubble(bubbleMessage, kBubbleNormal, 1, 1, image.Pt(580, 150))
	img := cachedBubbleTextImage(bubbleMessage, m.face, m.maxLineWidth, m.baseWidth, m.lineHeight, m.lines, color.Black)
	if img == nil {
		return fmt.Errorf("bubble text image missing")
	}
	var draw ebiten.DrawImageOptions
	draw.GeoM.Translate(20, 90)
	canvas.DrawImage(img, &draw)
	pixels := make([]byte, 640*220*4)
	canvas.ReadPixels(pixels)
	for _, region := range []image.Rectangle{image.Rect(0, 0, 640, 80), image.Rect(0, 80, 640, 220)} {
		colored := 0
		for y := region.Min.Y; y < region.Max.Y; y++ {
			for x := region.Min.X; x < region.Max.X; x++ {
				i := (y*640 + x) * 4
				if int(pixels[i]) > int(pixels[i+2])+20 && int(pixels[i+1]) > int(pixels[i+2])+20 {
					colored++
				}
			}
		}
		if colored < 30 {
			return fmt.Errorf("region %v has only %d colored emoji pixels", region, colored)
		}
	}
	// Mixed literal emoji and shortcodes keep the same face when toggled.
	// The image cache must still separate their two displayed forms.
	oldExpansion := gs.ExpandEmojiNames
	defer func() { gs.ExpandEmojiNames = oldExpansion }()
	const mixed = ":smile: 😄"
	var first *ebiten.Image
	for _, enabled := range []bool{true, false, true} {
		gs.ExpandEmojiNames = enabled
		img := cachedBubbleTextImage(mixed, face, 300, 300, 40, []string{displayEmojiText(mixed)}, color.Black)
		if first == nil {
			first = img
		} else if (img == first) != enabled {
			return fmt.Errorf("bubble image cache did not respect emoji expansion=%v", enabled)
		}
	}
	if path := os.Getenv("GOTHOOM_EMOJI_CAPTURE"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		return png.Encode(f, &image.RGBA{Pix: pixels, Stride: 640 * 4, Rect: image.Rect(0, 0, 640, 220)})
	}
	return nil
}
