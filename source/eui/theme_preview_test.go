package eui

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font/gofont/goregular"
)

// Run alone: Ebitengine permits only one RunGame call per process.
func TestRenderThemeGallery(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_THEME_GALLERY") == "" {
		t.Skip("set GOTHOOM_RENDER_THEME_GALLERY=1 to render theme states")
	}
	isolateThemeTest(t)
	if err := EnsureFontSource(goregular.TTF); err != nil {
		t.Fatal(err)
	}
	SetScreenSize(1600, 2400)
	SetUIScale(1)
	outputDir := os.Getenv("GOTHOOM_THEME_GALLERY_DIR")
	if outputDir == "" {
		outputDir = t.TempDir()
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	g := &themePreview{outputDir: outputDir}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
	t.Logf("Theme galleries: %s", outputDir)
}

type themePreview struct {
	done      bool
	outputDir string
	err       error
}

func (g *themePreview) Layout(_, _ int) (int, int) { return 1600, 2400 }
func (g *themePreview) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *themePreview) Draw(_ *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	g.err = g.render()
}

func (g *themePreview) render() error {
	if err := verifyPreviewMenuPixels(); err != nil {
		return err
	}
	if err := verifyPaletteSwitchPixels(); err != nil {
		return err
	}
	// Border pixels must survive the exact clipping used by controls.
	for _, radius := range []float32{0, 3, 8} {
		for _, width := range []float32{1, 2} {
			target := ebiten.NewImage(48, 32)
			drawRoundRect(target, &roundRect{Size: point{48, 32}, Fillet: radius, Border: width, Color: ColorWhite})
			for _, p := range []image.Point{{24, 0}, {24, 31}, {0, 16}, {47, 16}} {
				_, _, _, a := target.At(p.X, p.Y).RGBA()
				if a < 0xf000 {
					return fmt.Errorf("radius %v width %v lost border at %v", radius, width, p)
				}
			}
			target.Deallocate()
		}
	}
	renderNow = time.Now()
	for _, group := range []string{"palettes", "styles"} {
		var names []string
		var err error
		if group == "palettes" {
			names, err = ListThemes()
		} else {
			names, err = ListStyles()
		}
		if err != nil {
			return err
		}
		filtered := names[:0]
		for _, n := range names {
			if n != "Example" {
				filtered = append(filtered, n)
			}
		}
		names = filtered
		sheet := ebiten.NewImage(1440, 340*((len(names)+2)/3))
		for n, name := range names {
			if group == "palettes" {
				err = LoadTheme(name)
			} else {
				err = LoadTheme("AccentLight")
				if err == nil {
					err = LoadStyle(name)
				}
			}
			if err != nil {
				return err
			}
			card := ebiten.NewImage(480, 340)
			card.Fill(currentTheme.Window.BGColor)
			win := NewWindow()
			draw := func(it *itemData, x, y, w, h float32) {
				it.Size = point{w, h}
				it.ParentWindow = win
				it.drawItemInternal(point{x, y}, point{}, point{w, h}, rect{x, y, x + w, y + h}, card)
			}
			title, _ := NewText()
			title.Text = name
			title.FontSize = 18
			draw(title, 16, 12, 450, 30)
			for i, label := range []string{"Normal", "Hover", "Disabled"} {
				it, _ := NewButton()
				it.Text = label
				it.Hovered = i == 1
				it.Disabled = i == 2
				draw(it, float32(16+i*152), 52, 140, 32)
			}
			pressed, _ := NewButton()
			pressed.Text = "Pressed"
			pressed.Clicked = renderNow
			draw(pressed, 16, 96, 140, 32)
			input, _ := NewInput()
			input.Text = "Focused input"
			input.Focused = true
			draw(input, 168, 96, 140, 32)
			cb, _ := NewCheckbox()
			cb.Text = "Checked"
			cb.Checked = true
			draw(cb, 320, 102, 140, 24)
			selected, _ := NewText()
			selected.Text = "Selected text stays readable"
			selected.FontSize = 14
			selected.SelectableText = true
			selected.SelectStart = 0
			selected.SelectEnd = 13
			draw(selected, 16, 144, 448, 28)
			dd, _ := NewDropdown()
			dd.Options = []string{"Normal option", "Selected option", "Hovered option"}
			dd.Selected = 1
			dd.HoverIndex = 2
			dd.Open = true
			draw(dd, 16, 188, 248, 28)
			drawDropdownOptions(dd, point{16, 188}, rect{0, 0, 480, 340}, card)
			caption, _ := NewText()
			caption.Text = "Custom white accent"
			caption.FontSize = 12
			draw(caption, 286, 192, 180, 24)
			SetAccentSaturation(0)
			SetAccentColor(ColorWhite)
			extreme, _ := NewButton()
			extreme.Text = "Readable"
			extreme.Clicked = renderNow
			draw(extreme, 286, 220, 160, 32)
			// Verify the actual renderer applies contrasting icon color on a white press.
			icon := ebiten.NewImage(16, 16)
			icon.Fill(ColorWhite)
			mask, _ := NewButton()
			mask.Image = icon
			mask.TintImage = true
			mask.Filled = true
			mask.Clicked = renderNow
			draw(mask, 286, 266, 32, 32)
			pixel := card.At(302, 282)
			r, gg, b, _ := pixel.RGBA()
			if textContrast(NewColor(uint8(r>>8), uint8(gg>>8), uint8(b>>8), 255), ColorWhite) < 4.5 {
				return fmt.Errorf("%s icon disappeared on white highlight", name)
			}
			icon.Deallocate()
			f, err := os.Create(filepath.Join(g.outputDir, "gothoom-"+group+"-"+name+".png"))
			if err != nil {
				return err
			}
			if err = png.Encode(f, card); err != nil {
				return err
			}
			f.Close()
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(n%3*480), float64(n/3*340))
			sheet.DrawImage(card, op)
			card.Deallocate()
		}
		f, err := os.Create(filepath.Join(g.outputDir, "gothoom-"+group+"-gallery.png"))
		if err != nil {
			return err
		}
		if err = png.Encode(f, sheet); err != nil {
			return err
		}
		f.Close()
		sheet.Deallocate()
	}
	return nil
}

// Compare controls surviving a palette change with freshly constructed controls.
// This catches copied colors that otherwise look correct in a static gallery.
func verifyPaletteSwitchPixels() error {
	if err := LoadTheme("NeonNight"); err != nil {
		return err
	}
	win := NewWindow()
	oldWindows := windows
	windows = []*windowData{win}
	defer func() { windows = oldWindows }()
	constructors := []func() (*itemData, *EventHandler){NewButton, NewInput, NewProgressBar}
	for _, makeControl := range constructors {
		item, _ := makeControl()
		win.AddItem(item)
	}
	names, err := ListThemes()
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := LoadTheme(name); err != nil {
			return err
		}
		for i, makeControl := range constructors {
			fresh, _ := makeControl()
			var pixels [2][]byte
			for j, item := range []*itemData{win.Contents[i], fresh} {
				item.Text = "Palette change"
				item.Size = point{200, 32}
				item.ParentWindow = win
				item.MinValue, item.MaxValue, item.Value = 0, 100, 60
				canvas := ebiten.NewImage(200, 32)
				canvas.Fill(win.backgroundColor())
				item.drawItemInternal(point{}, point{}, point{200, 32}, rect{0, 0, 200, 32}, canvas)
				pixels[j] = make([]byte, 200*32*4)
				canvas.ReadPixels(pixels[j])
				canvas.Deallocate()
			}
			if !bytes.Equal(pixels[0], pixels[1]) {
				return fmt.Errorf("%s control %v kept stale colors after switching", name, fresh.ItemType)
			}
		}
	}
	return nil
}

func verifyPreviewMenuPixels() error {
	if err := LoadTheme("AccentLight"); err != nil {
		return err
	}
	dd, _ := NewDropdown()
	dd.Open = true
	dd.Label = "Style"
	dd.Options = []string{"Current", "Preview", "Third"}
	dd.Selected, dd.HoverIndex = 0, 1
	dd.Size = point{200, 28}
	dd.OnHover = func(int) {}
	var pixels [2][]byte
	for i, offset := range []point{{20, 20}, {100, 100}} {
		if i == 1 {
			// The rest of the UI adopts new typography and padding during preview.
			dd.FontSize = 32
			dd.Size = point{320, 64}
			dd.BorderPad = 20
			next := *currentStyle
			next.TextPadding = 24
			currentStyle = &next
		}
		canvas := ebiten.NewImage(640, 480)
		drawDropdownOptions(dd, offset, rect{0, 0, 640, 480}, canvas)
		pixels[i] = make([]byte, 640*480*4)
		canvas.ReadPixels(pixels[i])
		canvas.Deallocate()
	}
	if !bytes.Equal(pixels[0], pixels[1]) {
		return fmt.Errorf("preview changed the open menu's text size, spacing or position")
	}
	return nil
}
