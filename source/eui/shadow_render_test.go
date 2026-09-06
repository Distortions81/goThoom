package eui

import (
	"fmt"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// Run alone; pixel reads require Ebitengine's game loop.
func TestRenderWindowEffects(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_WINDOW_EFFECTS") == "" {
		t.Skip("set GOTHOOM_RENDER_WINDOW_EFFECTS=1")
	}
	isolateThemeTest(t)
	old := windowShadows
	windowShadows = true
	t.Cleanup(func() { windowShadows = old })
	g := &windowEffectsGame{}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type windowEffectsGame struct {
	done bool
	err  error
}

func (g *windowEffectsGame) Layout(_, _ int) (int, int) { return 200, 160 }
func (g *windowEffectsGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *windowEffectsGame) Draw(_ *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	g.err = verifyWindowEffects()
}

func verifyWindowEffects() error {
	if err := LoadTheme("SeaGlass"); err != nil {
		return err
	}
	for _, noCache := range []bool{false, true} {
		win := NewWindow()
		win.NoCache = noCache
		win.Size = point{80, 60}
		win.Position = point{40, 40}
		win.TitleHeight = 0
		win.ShadowSize = 16
		win.ShadowColor = NewColor(30, 180, 210, 180)
		win.ShadowFalloff = 2
		canvas := ebiten.NewImage(180, 140)
		draw := func() uint32 {
			canvas.Clear()
			var menus []openDropdown
			win.Draw(canvas, &menus)
			_, _, _, a := canvas.At(34, 65).RGBA()
			return a
		}
		full := draw()
		if full == 0 {
			return fmt.Errorf("NoCache=%v shadow was clipped to window bounds", noCache)
		}
		for _, p := range [][2]int{{125, 65}, {80, 34}, {80, 105}, {36, 36}, {123, 36}, {36, 103}, {123, 103}} {
			_, _, _, alpha := canvas.At(p[0], p[1]).RGBA()
			if alpha == 0 {
				return fmt.Errorf("NoCache=%v missing shadow edge or corner at %v", noCache, p)
			}
		}
		if second := draw(); second != full {
			return fmt.Errorf("cached frame lost shadow")
		}
		win.Opacity = 0.5
		half := draw()
		if half == 0 || half >= full {
			return fmt.Errorf("shadow ignored window opacity")
		}
		win.Opacity = 1
		win.ShadowFalloff = 5
		if draw() >= full {
			return fmt.Errorf("falloff did not steepen the fade")
		}
		win.ShadowSize = 0
		if draw() != 0 {
			return fmt.Errorf("zero size did not disable effect")
		}
		win.ShadowSize = 16
		win.Docked = true
		if draw() != 0 {
			return fmt.Errorf("docked pane drew outer effect")
		}
		win.Docked = false
		win.NoBGColor = true
		if draw() != 0 {
			return fmt.Errorf("backgroundless window drew outer effect")
		}
		win.NoBGColor = false
		windowShadows = false
		if draw() != 0 {
			return fmt.Errorf("global toggle left glow enabled")
		}
		windowShadows = true
		win.Opacity = 0
		if draw() != 0 {
			return fmt.Errorf("invisible window drew a glow")
		}
		canvas.Deallocate()
		win.deallocate()
	}
	return nil
}
