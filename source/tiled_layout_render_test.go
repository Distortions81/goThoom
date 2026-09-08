package main

import (
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"gothoom/eui"
)

// Run alone; use simple labeled panes to inspect workspace geometry without a login.
func TestRenderTiledLayoutChoices(t *testing.T) {
	dir := os.Getenv("GOTHOOM_RENDER_TILED_LAYOUTS")
	if dir == "" {
		t.Skip("set GOTHOOM_RENDER_TILED_LAYOUTS to an output directory")
	}
	initFont()
	eui.SetScreenSize(1920, 1080)
	if err := eui.LoadTheme("AccentDark"); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	g := &tiledLayoutRenderGame{dir: dir}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type tiledLayoutRenderGame struct {
	dir  string
	done bool
	err  error
}

func (g *tiledLayoutRenderGame) Layout(_, _ int) (int, int) { return 1920, 1080 }
func (g *tiledLayoutRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *tiledLayoutRenderGame) Draw(screen *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	g.err = g.render(screen)
}
func (g *tiledLayoutRenderGame) render(screen *ebiten.Image) error {
	windows := make([]*eui.WindowData, 0, 5)
	for _, title := range []string{"Game", "Inventory", "Players", "Console", "Chat"} {
		win := eui.NewWindow()
		win.Title, win.Resizable = title, true
		win.SetDocked(true)
		label, _ := eui.NewText()
		label.Text = title + " pane"
		label.Size = eui.Point{X: 150, Y: 30}
		win.AddItem(label)
		win.AddWindow(false)
		windows = append(windows, win)
		defer win.RemoveWindow()
	}
	for _, scale := range []float32{1, 2} {
		eui.SetUIScale(scale)
		for layout := TiledLayoutCenter; layout <= TiledLayoutFullMessagesAbove; layout++ {
			for _, stacked := range []bool{false, true} {
				for _, combined := range []bool{false, true} {
					gs = gsdef
					gs.TiledMessagesStacked = stacked
					gs.TiledWindows, gs.TiledLayout, gs.MessagesToConsole = true, layout, combined
					applyTiledWindowStates()
					states := []*WindowState{&gs.GameWindow, &gs.InventoryWindow, &gs.PlayersWindow, &gs.MessagesWindow, &gs.ChatWindow}
					for i, state := range states {
						applyWindowState(windows[i], state)
						windows[i].Refresh()
					}
					configureTiledWorkspaceDividers()
					screen.Clear()
					eui.Draw(screen)
					for i, state := range states {
						if !state.Open {
							continue
						}
						pos, size := windows[i].GetPos(), windows[i].GetSize()
						if math.Abs(float64(pos.X)-state.Position.X*1920) > 1.1 || math.Abs(float64(pos.Y)-state.Position.Y*1080) > 1.1 || math.Abs(float64(size.X)-state.Size.X*1920) > 1.1 || math.Abs(float64(size.Y)-state.Size.Y*1080) > 1.1 {
							return fmt.Errorf("%s native pane did not fit assigned rectangle (layout=%d stacked=%t scale=%g)", windows[i].Title, layout, stacked, scale)
						}
						if pos.X < 0 || pos.Y < 0 || pos.X+size.X > 1920.1 || pos.Y+size.Y > 1080.1 {
							return fmt.Errorf("%s pane escaped workspace", windows[i].Title)
						}
					}
					if err := g.save(screen, fmt.Sprintf("layout-%d-combined-%t-stacked-%t-%gx.png", layout, combined, stacked, scale)); err != nil {
						return err
					}
				}
			}
		}
	}
	for _, win := range windows {
		win.Close()
	}
	eui.SetTileDividers(nil)
	makeTileLayoutWindow()
	tileLayoutWin.MarkOpen()
	for _, scale := range []float32{1, 2} {
		eui.SetUIScale(scale)
		tileLayoutWin.Refresh()
		screen.Clear()
		eui.Draw(screen)
		if h, v := tileLayoutWin.RequiresScroll(); h || v {
			return fmt.Errorf("layout chooser requires scrolling at %gx", scale)
		}
		if err := checkRenderedControlText(tileLayoutWin.Contents, scale); err != nil {
			return err
		}
		if err := g.save(screen, fmt.Sprintf("chooser-%gx.png", scale)); err != nil {
			return err
		}
	}
	return nil
}
func (g *tiledLayoutRenderGame) save(screen *ebiten.Image, name string) error {
	f, err := os.Create(filepath.Join(g.dir, name))
	if err != nil {
		return err
	}
	err = png.Encode(f, screen)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}
