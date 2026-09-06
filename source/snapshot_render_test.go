package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
)

// Run alone: Ebitengine pixel reads need its game loop.
func TestRenderSnapshotOptions(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_SNAPSHOT") == "" {
		t.Skip("set GOTHOOM_RENDER_SNAPSHOT=1")
	}
	initFont()
	dataDirPath = t.TempDir()
	eui.SetScreenSize(640, 400)
	eui.SetUIScale(1)
	if err := eui.LoadTheme("AccentDark"); err != nil {
		t.Fatal(err)
	}
	showSnapshotWindow()
	snapshotName.Text = "Friends in Puddleby"
	dir := os.Getenv("GOTHOOM_SNAPSHOT_RENDER_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	g := &snapshotRenderGame{path: filepath.Join(dir, "snapshot-options.png")}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type snapshotRenderGame struct {
	done bool
	path string
	err  error
}

func (g *snapshotRenderGame) Layout(_, _ int) (int, int) { return 640, 400 }
func (g *snapshotRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *snapshotRenderGame) Draw(screen *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	g.err = g.verify(screen)
}
func (g *snapshotRenderGame) verify(screen *ebiten.Image) error {
	bg := eui.NewColor(45, 48, 52, 255)
	screen.Fill(bg)
	eui.Draw(screen)
	f, err := os.Create(g.path)
	if err != nil {
		return err
	}
	err = png.Encode(f, screen)
	f.Close()
	if err != nil {
		return err
	}
	gameImage = ebiten.NewImage(16, 12)
	defer gameImage.Deallocate()
	gameImage.Fill(eui.NewColor(255, 0, 0, 255))
	worldViewRect = image.Rect(2, 3, 8, 7)
	for _, full := range []bool{false, true} {
		snapshotWin.MarkOpen()
		// Exercise the Capture button, including closing before pixel capture.
		row := snapshotWin.Contents[0].Contents[1]
		snapshotWin.Contents[0].Contents[2].Checked = !full
		if full {
			row.Contents[0].Selected = 1
			snapshotName.Text = "Full window"
		}
		snapshotWin.DefaultButton.Handler.Emit(eui.UIEvent{Type: eui.EventClick})
		if snapshotWin.Open || pendingSnapshot == nil {
			return fmt.Errorf("Capture did not close and queue snapshot")
		}
		if snapshotHidesNameTags() != full {
			return fmt.Errorf("name-tag checkbox did not reach the capture request")
		}
		screen.Fill(bg)
		eui.Draw(screen)
		capturePendingSnapshot(screen)
		if snapshotWin.Open || pendingSnapshot != nil {
			return fmt.Errorf("capture failed or remained queued")
		}
		f, err := os.Open(filepath.Join(snapshotDirectory(), snapshotName.Text+".png"))
		if err != nil {
			return err
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			return err
		}
		want := image.Rect(0, 0, 6, 4)
		if full {
			want = image.Rect(0, 0, 640, 400)
			r, green, b, _ := img.At(100, 100).RGBA()
			if r>>8 != uint32(bg.R) || green>>8 != uint32(bg.G) || b>>8 != uint32(bg.B) {
				return fmt.Errorf("full snapshot included the options window")
			}
		}
		if img.Bounds() != want {
			return fmt.Errorf("capture bounds = %v; want %v", img.Bounds(), want)
		}
	}
	return nil
}
