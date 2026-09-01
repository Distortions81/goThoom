package main

import (
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"os"
	"strconv"
	"testing"

	"gothoom/climg"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestRenderMobileRecolorPixels(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_MOBILE_RECOLOR_TEST") == "" {
		t.Skip("set GOTHOOM_RENDER_MOBILE_RECOLOR_TEST=1 to verify mobile recolor pixels")
	}
	if err := ReloadSpriteUpscaleShader(); err != nil {
		t.Fatalf("compile artwork shaders: %v", err)
	}
	mobilePaletteBlendCache = make(map[mobilePalettePairKey]*mobilePaletteBlendShaderState)
	game := &mobileRecolorRenderGame{}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

func TestRenderRealMobileRecolorPixels(t *testing.T) {
	archivePath := os.Getenv("GOTHOOM_RENDER_MOBILE_RECOLOR_REAL")
	if archivePath == "" {
		t.Skip("set GOTHOOM_RENDER_MOBILE_RECOLOR_REAL to a CL_Images archive")
	}
	images, err := climg.Load(archivePath)
	if err != nil {
		t.Fatalf("load CL_Images: %v", err)
	}
	pictID := uint64(453)
	if value := os.Getenv("GOTHOOM_RENDER_MOBILE_RECOLOR_PICT"); value != "" {
		pictID, err = strconv.ParseUint(value, 10, 16)
		if err != nil {
			t.Fatalf("parse pict ID: %v", err)
		}
	}
	colorsHex := os.Getenv("GOTHOOM_RENDER_MOBILE_RECOLOR_COLORS")
	if colorsHex == "" {
		colorsHex = "14b2d608335e4e5681080f3a56ac4f81acacff6cc2"
	}
	colors, err := hex.DecodeString(colorsHex)
	if err != nil {
		t.Fatalf("parse custom colors: %v", err)
	}
	if !images.HasCustomColors(uint32(pictID)) {
		t.Fatalf("pict %d does not have custom colors", pictID)
	}

	originalSettings, originalImages := gs, clImages
	gs.ShadersEnabled = true
	gs.DenoiseImages = false
	gs.SpriteUpscaleFilter = false
	gs.SpriteUpscaleMode = artworkUpscaleOff
	clImages = images
	if err := ReloadSpriteUpscaleShader(); err != nil {
		t.Fatalf("compile artwork shaders: %v", err)
	}
	t.Cleanup(func() {
		clearCaches()
		clImages = originalImages
		gs = originalSettings
	})
	game := &realMobileRecolorRenderGame{id: uint16(pictID), colors: colors}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

type mobileRecolorRenderGame struct {
	rendered bool
	err      error
}

func testMobilePalette(key mobileKey, red, green, blue, alpha float32) *mobilePaletteShaderState {
	state := &mobilePaletteShaderState{key: key}
	state.r[0], state.g[0], state.b[0], state.a[0] = red, green, blue, alpha
	state.op.Uniforms = map[string]any{
		"PaletteR": state.r[:], "PaletteG": state.g[:],
		"PaletteB": state.b[:], "PaletteA": state.a[:],
	}
	return state
}

func testMobileInfluence() *ebiten.Image {
	// Slot one in the low five bits, no edge-blend contribution.
	return ebiten.NewImageFromImage(&onePixelRGBA{color.RGBA{R: 1, A: 255}})
}

type onePixelRGBA struct{ color color.RGBA }

func (i *onePixelRGBA) ColorModel() color.Model { return color.RGBAModel }
func (i *onePixelRGBA) Bounds() image.Rectangle { return image.Rect(0, 0, 1, 1) }
func (i *onePixelRGBA) At(_, _ int) color.Color { return i.color }

func (g *mobileRecolorRenderGame) Update() error {
	if g.rendered {
		return ebiten.Termination
	}
	return nil
}

func (g *mobileRecolorRenderGame) Draw(_ *ebiten.Image) {
	previous := ebiten.NewImage(1, 1)
	previous.Fill(color.RGBA{R: 255, A: 255})
	current := ebiten.NewImage(1, 1)
	current.Fill(color.RGBA{G: 255, A: 255})
	previousInfluence := testMobileInfluence()
	currentInfluence := testMobileInfluence()
	previousPalette := testMobilePalette(makeMobileKey(1, 0, []byte{1}), -1, 0, 1, 0)
	currentPalette := testMobilePalette(makeMobileKey(2, 0, []byte{2}), 1, 0, 0, 0)

	destination := ebiten.NewImage(1, 1)
	options := frameBlendDrawOptions{ScaleX: 1, ScaleY: 1, Red: 1, Green: 1, Blue: 1, Alpha: 1}
	if !drawRecoloredMobile(destination, previous, previousInfluence, previousPalette, options) {
		g.err = fmt.Errorf("single-frame recolor draw was not available")
		g.rendered = true
		return
	}
	pixel := make([]byte, 4)
	destination.ReadPixels(pixel)
	if err := checkFrameBlendPixel(pixel, 1, 0, 0, color.RGBA{B: 255, A: 255}); err != nil {
		g.err = fmt.Errorf("single recolor: %w", err)
		g.rendered = true
		return
	}

	destination.Clear()
	options.Fade = 0.25
	if !drawRecoloredMobileFrameBlend(destination, previous, previousInfluence, current, currentInfluence, previousPalette, currentPalette, options) {
		g.err = fmt.Errorf("blended recolor draw was not available")
		g.rendered = true
		return
	}
	destination.ReadPixels(pixel)
	if err := checkFrameBlendPixel(pixel, 1, 0, 0, color.RGBA{R: 64, G: 64, B: 191, A: 255}); err != nil {
		g.err = fmt.Errorf("blended recolor: %w", err)
	}
	g.rendered = true
}

func (g *mobileRecolorRenderGame) Layout(_, _ int) (int, int) { return 1, 1 }

type realMobileRecolorRenderGame struct {
	id       uint16
	colors   []byte
	rendered bool
	err      error
}

func (g *realMobileRecolorRenderGame) Update() error {
	if g.rendered {
		return ebiten.Termination
	}
	return nil
}

func (g *realMobileRecolorRenderGame) Draw(_ *ebiten.Image) {
	var cpu, base, influence *ebiten.Image
	var palette *mobilePaletteShaderState
	selectedState := uint8(0)
	for state := 0; state < 256 && base == nil; state++ {
		var ok bool
		base, influence, palette, ok = loadGPURecoloredMobileFrame(g.id, uint8(state), g.colors)
		if !ok {
			base = nil
		} else {
			selectedState = uint8(state)
		}
	}
	if base == nil {
		g.err = fmt.Errorf("no visible recolorable pose for pict %d", g.id)
		g.rendered = true
		return
	}
	imageMu.Lock()
	beforeSheets, beforeMobiles := len(sheetCache), len(mobileCache)
	beforeScaled, beforeMasks := len(scaledMobileCache), len(mobileRecolorMaskCache)
	imageMu.Unlock()
	secondColors := append([]byte(nil), g.colors...)
	secondColors[len(secondColors)-1] ^= 1
	secondBase, secondInfluence, _, secondOK := loadGPURecoloredMobileFrame(g.id, selectedState, secondColors)
	imageMu.Lock()
	afterSheets, afterMobiles := len(sheetCache), len(mobileCache)
	afterScaled, afterMasks := len(scaledMobileCache), len(mobileRecolorMaskCache)
	imageMu.Unlock()
	if !secondOK || secondBase != base || secondInfluence != influence ||
		beforeSheets != afterSheets || beforeMobiles != afterMobiles || beforeScaled != afterScaled || beforeMasks != afterMasks {
		g.err = fmt.Errorf("second palette created artwork variants: sheets %d/%d mobiles %d/%d scaled %d/%d masks %d/%d",
			beforeSheets, afterSheets, beforeMobiles, afterMobiles, beforeScaled, afterScaled, beforeMasks, afterMasks)
		g.rendered = true
		return
	}
	cpu = loadMobileFrame(g.id, selectedState, g.colors)
	if cpu == nil {
		g.err = fmt.Errorf("CPU-colored pose %d:%d was unavailable", g.id, selectedState)
		g.rendered = true
		return
	}
	destination := ebiten.NewImage(base.Bounds().Dx(), base.Bounds().Dy())
	if !drawRecoloredMobile(destination, base, influence, palette, frameBlendDrawOptions{
		ScaleX: 1, ScaleY: 1, Red: 1, Green: 1, Blue: 1, Alpha: 1,
	}) {
		g.err = fmt.Errorf("real mobile recolor draw was not available")
		g.rendered = true
		return
	}
	w, h := cpu.Bounds().Dx(), cpu.Bounds().Dy()
	want, got := make([]byte, w*h*4), make([]byte, w*h*4)
	cpu.ReadPixels(want)
	destination.ReadPixels(got)
	for offset := range got {
		delta := int(got[offset]) - int(want[offset])
		if delta < -1 || delta > 1 {
			pixel := offset / 4
			g.err = fmt.Errorf("real recolor differs at (%d,%d) channel %d: got %d want %d", pixel%w, pixel/w, offset%4, got[offset], want[offset])
			break
		}
	}
	g.rendered = true
}

func (g *realMobileRecolorRenderGame) Layout(_, _ int) (int, int) { return 128, 128 }
