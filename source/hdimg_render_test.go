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
	originalEnabled := gs.UseSpritePackFiles
	gs.UseSpritePackFiles = true
	t.Cleanup(func() { gs.UseSpritePackFiles = originalEnabled })
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
	for _, id := range []uint16{23, 41, 194, 208, 210, 302, 307, 317, 417, 509, 601, 603, 626, 635, 738, 873, 985, 1068, 1279, 1404, 2245, 2252, 3785, 3786, 4495, 5764} {
		img := loadImageFrame(id, 0)
		wantWidth, wantHeight := 168, 168
		if id == 302 || id == 307 || id == 317 || id == 5764 {
			wantWidth, wantHeight = 800, 800
		} else if id == 873 {
			wantWidth, wantHeight = 836, 924
		} else if id == 985 {
			wantWidth, wantHeight = 720, 640
		} else if id == 601 {
			wantWidth, wantHeight = 1468, 584
		} else if id == 603 {
			wantWidth, wantHeight = 584, 1468
		} else if id == 2245 {
			wantWidth, wantHeight = 304, 92
		} else if id == 3785 || id == 3786 {
			wantWidth, wantHeight = 156, 108
		} else if id == 41 {
			wantWidth, wantHeight = 96, 96
		} else if id == 194 {
			wantWidth, wantHeight = 1, 1
		} else if id == 1404 {
			wantWidth, wantHeight = 1, 1
		} else if id == 509 {
			wantWidth, wantHeight = 876, 928
		}
		if img == nil || !isHDPictureImage(id, img) || img.Bounds().Dx() != wantWidth || img.Bounds().Dy() != wantHeight {
			g.err = fmt.Errorf("picture %d did not load its %dx%d HD replacement", id, wantWidth, wantHeight)
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
		if location, count := reloadHDPictures(); !strings.Contains(location, "hdimg") || count < 2 {
			g.err = fmt.Errorf("HD reload source = %q (%d images), want an external hdimg folder", location, count)
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
	originalEnabled := gs.UseSpritePackFiles
	gs.UseSpritePackFiles = true
	t.Cleanup(func() { gs.UseSpritePackFiles = originalEnabled })
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
