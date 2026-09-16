package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"gothoom/eui"
)

func TestRenderHDPictureReplacesOriginalWithoutReupscale(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_HDIMG_TEST") == "" {
		t.Skip("set GOTHOOM_RENDER_HDIMG_TEST=1 to verify the HD picture override")
	}
	game := &hdPictureRenderGame{}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

type hdPictureRenderGame struct {
	done bool
	err  error
}

func (g *hdPictureRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}

func (g *hdPictureRenderGame) Draw(_ *ebiten.Image) {
	defer func() { g.done = true }()
	for _, id := range []uint16{23, 635, 1068, 1279} {
		img := loadImageFrame(id, 0)
		if img == nil || !isHDPictureImage(id, img) || img.Bounds().Dx() != 168 || img.Bounds().Dy() != 168 {
			g.err = fmt.Errorf("picture %d did not load its 168x168 HD replacement", id)
			return
		}
		if got := getScaledPictureFrame(id, 0, img); got != img {
			g.err = fmt.Errorf("HD picture %d was processed by the normal artwork upscaler", id)
			return
		}
		if got := loadImageFrame(id, 0); got != img {
			g.err = fmt.Errorf("HD picture %d was decoded again instead of reused", id)
			return
		}
		if location, count := reloadHDPictures(); !strings.HasSuffix(location, "data/hdimg") || count < 2 {
			g.err = fmt.Errorf("HD reload source = %q (%d images), want checked-out data/hdimg", location, count)
			return
		}
		if reloaded := loadImageFrame(id, 0); reloaded == nil || reloaded == img || !isHDPictureImage(id, reloaded) {
			g.err = fmt.Errorf("HD picture %d was not decoded afresh after reload", id)
			return
		}
	}
}

func (g *hdPictureRenderGame) Layout(_, _ int) (int, int) { return 42, 42 }

func TestRenderHDPicturePreviewWindow(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_HDIMG_PREVIEW_TEST") == "" {
		t.Skip("set GOTHOOM_RENDER_HDIMG_PREVIEW_TEST=1 to verify the HD sprite preview")
	}
	initFont()
	eui.SetScreenSize(640, 400)
	game := &hdPicturePreviewRenderGame{}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

type hdPicturePreviewRenderGame struct {
	opened bool
	done   bool
	err    error
}

func (g *hdPicturePreviewRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	if !g.opened {
		openHDPicturePreview()
		g.opened = true
	}
	return nil
}

func (g *hdPicturePreviewRenderGame) Draw(screen *ebiten.Image) {
	if g.done {
		return
	}
	defer func() { g.done = true }()
	if hdPicturePreviewWin == nil || !hdPicturePreviewWin.IsOpen() {
		g.err = fmt.Errorf("HD sprite preview did not open")
		return
	}
	if len(hdPicturePreviewPicker.Options) < 3 || hdPicturePreviewNew.Image == nil {
		g.err = fmt.Errorf("HD sprite preview did not populate its image picker")
		return
	}
	screen.Clear()
	eui.Draw(screen)
}

func (g *hdPicturePreviewRenderGame) Layout(_, _ int) (int, int) { return 640, 400 }
