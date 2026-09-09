package main

import (
	"fmt"
	"image/png"
	"os"
	"strconv"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"gothoom/eui"
)

func TestRenderEmojiPicker(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_EMOJI_PICKER") == "" {
		t.Skip("set GOTHOOM_RENDER_EMOJI_PICKER=1; run alone")
	}
	gs = cloneSettings(gsdef)
	scale := float32(1)
	if value := os.Getenv("GOTHOOM_EMOJI_PICKER_SCALE"); value != "" {
		parsed, err := strconv.ParseFloat(value, 32)
		if err != nil {
			t.Fatal(err)
		}
		scale = float32(parsed)
	}
	eui.SetScreenSize(960, 720)
	eui.SetUIScale(scale)
	initFont()
	consoleWin, messagesFlow, inputFlow = newTextWindow("Chat", eui.HZoneLeft, eui.VZoneBottom, true, nil)
	consoleWin.Size = eui.Point{X: 900 / scale, Y: 180 / scale}
	consoleWin.MarkOpen()
	consoleWin.ClearZone()
	_ = consoleWin.SetPos(eui.Point{X: 20, Y: 490})
	inputText = []rune("Hello :smile:")
	inputPos = len(inputText)
	inputActive = true
	updateTextWindow(consoleWin, messagesFlow, inputFlow, []string{"Self: 😄", "Another exile: Hi!"}, 14, string(inputText), nil, false, &consoleTextWrapCache)
	consoleWin.Refresh()
	openMessageEmojiPicker(inputFlow, messageEmojiButton(inputFlow))
	_ = activeEmojiPicker.win.SetPos(eui.Point{X: 20, Y: 20})
	g := &emojiPickerRenderGame{}
	ebiten.SetWindowVisible(false)
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type emojiPickerRenderGame struct {
	done bool
	err  error
}

func (*emojiPickerRenderGame) Layout(int, int) (int, int) { return 960, 720 }
func (g *emojiPickerRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *emojiPickerRenderGame) Draw(*ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	g.err = g.verify()
}
func (*emojiPickerRenderGame) verify() error {
	canvas := ebiten.NewImage(960, 720)
	defer canvas.Deallocate()
	eui.Draw(canvas)
	button := messageEmojiButton(inputFlow)
	bar := messageInputItem(inputFlow)
	if bar.DrawRect.Y1 <= bar.DrawRect.Y0 || button.DrawRect.X0 < bar.DrawRect.X1-1 || button.DrawRect.Y1 <= button.DrawRect.Y0 {
		return fmt.Errorf("emoji input action overlaps or is hidden: bar=%v button=%v", bar.DrawRect, button.DrawRect)
	}
	p := activeEmojiPicker
	for _, row := range p.panel.Contents {
		for _, button := range row.Contents {
			if button.DrawRect.Y1 > button.DrawRect.Y0 && button.DrawRect.X1-button.DrawRect.X0 < button.GetSize().X-1 {
				return fmt.Errorf("emoji column clipped: %s %v, expected width %v", button.Text, button.DrawRect, button.GetSize().X)
			}
		}
	}
	before := p.groupButtons["Smileys & Emotion"].DrawRect
	if path := os.Getenv("GOTHOOM_EMOJI_PICKER_CAPTURE"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		if err := png.Encode(f, canvas); err != nil {
			return err
		}
	}
	p.panel.Scroll.Y = 300
	p.win.Refresh()
	canvas.Clear()
	eui.Draw(canvas)
	if p.groupButtons["Smileys & Emotion"].DrawRect != before {
		return fmt.Errorf("category sidebar moved with emoji scrolling")
	}
	p.groupButtons["Food & Drink"].Handler.Emit(eui.UIEvent{Type: eui.EventClick})
	if p.group != "Food & Drink" || p.panel.Scroll.Y != 0 {
		return fmt.Errorf("group navigation did not reset the panel")
	}
	p.search.Handler.Emit(eui.UIEvent{Type: eui.EventInputChanged, Text: "rocket"})
	canvas.Clear()
	eui.Draw(canvas)
	if len(p.panel.Contents) == 0 {
		return fmt.Errorf("search did not build results")
	}
	var rocket *eui.ItemData
	for _, row := range p.panel.Contents {
		for _, button := range row.Contents {
			if button.Text == "🚀" {
				rocket = button
			}
		}
	}
	if rocket == nil {
		return fmt.Errorf("rocket absent from search results")
	}
	rocket.Handler.Emit(eui.UIEvent{Type: eui.EventClick})
	if activeEmojiPicker != nil || string(inputText) != "Hello :smile::rocket:" || selectedMessageInput != inputFlow {
		return fmt.Errorf("emoji selection did not return to the draft: %q", string(inputText))
	}
	return nil
}
