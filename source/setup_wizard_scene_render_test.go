package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"gothoom/climg"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestRenderSetupWizardSyntheticScenes(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_SETUP_WIZARD_SCENES") == "" {
		t.Skip("set GOTHOOM_RENDER_SETUP_WIZARD_SCENES=1 to render setup wizard scenes")
	}
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate setup wizard scene test")
	}
	images, err := climg.Load(filepath.Join(filepath.Dir(sourceFile), "data", "CL_Images"))
	if err != nil {
		t.Fatal(err)
	}
	outputDir := filepath.Join(filepath.Dir(sourceFile), "data", "Screenshots", "setup-wizard-scenes")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}

	originalSettings := gs
	originalImages := clImages
	originalNight := captureMovieNightState()
	originalPage := setupWizardPage
	gs.GameScale = 1
	gs.CharacterShadows = true
	gs.ShadersEnabled = true
	gs.ShaderLighting = true
	gs.ShaderLightStrength = 1
	gs.ShaderGlowStrength = 1
	clImages = images
	t.Cleanup(func() {
		clearCaches()
		clImages = originalImages
		gs = originalSettings
		setupWizardPage = originalPage
		restoreMovieNightState(originalNight)
	})

	game := &setupWizardSceneRenderGame{outputDir: outputDir}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

type setupWizardSceneRenderGame struct {
	outputDir string
	rendered  bool
	err       error
}

func (g *setupWizardSceneRenderGame) Update() error {
	if g.rendered {
		return ebiten.Termination
	}
	return nil
}

func (g *setupWizardSceneRenderGame) Draw(_ *ebiten.Image) {
	if lightingShader == nil {
		if err := ReloadLightingShader(); err != nil {
			g.err = fmt.Errorf("compile lighting shader: %w", err)
			g.rendered = true
			return
		}
	}
	if err := verifyLightingCoordinates(); err != nil {
		g.err = err
		g.rendered = true
		return
	}
	initFont()
	for _, mode := range []setupWizardSceneMode{setupWizardSceneDay, setupWizardSceneIndoor, setupWizardSceneNight, setupWizardSceneMotion} {
		setupWizardSceneModeValue = mode
		setupWizardPage = 5
		switch mode {
		case setupWizardSceneIndoor:
			setupWizardPage = 2
		case setupWizardSceneNight:
			setupWizardPage = 6
		case setupWizardSceneMotion:
			setupWizardPage = 4
		}
		setupWizardSceneStarted = time.Unix(1000, 0)
		now := setupWizardSceneStarted.Add(650 * time.Millisecond)
		var snap drawSnapshot
		prepareSetupWizardSceneSnapshot(&snap, now)
		if mode == setupWizardSceneDay && !setupWizardSceneHasObscuringPicture(snap) {
			g.err = fmt.Errorf("daylight scene has no foreground picture obscuring the moving traveler")
			break
		}
		if mode == setupWizardSceneDay {
			probe := ebiten.NewImage(gameAreaSizeX, gameAreaSizeY)
			probe.Fill(color.RGBA{R: 120, G: 130, B: 140, A: 255})
			// Exercise the opt-in batched path separately. The production draw
			// below keeps the default draw-order-correct layered path.
			gs.FasterCharacterShadows = true
			drawMobileShadows(probe, 0, 0, snap.mobiles, snap.descriptors, snap.prevMobiles, snap.picShiftX, snap.picShiftY, 1, maxMobileInterpPixels)
			if frameDetailedShadowMask == nil || frameDetailedShadowBounds.Empty() {
				g.err = fmt.Errorf("daylight scene did not produce a detailed shadow mask")
				break
			}
			before := make([]byte, 4*gameAreaSizeX*gameAreaSizeY)
			probe.ReadPixels(before)
			applyDetailedCharacterShadow(probe)
			after := make([]byte, len(before))
			probe.ReadPixels(after)
			if bytes.Equal(before, after) {
				g.err = fmt.Errorf("cropped detailed shadow pass did not darken the scene")
				break
			}
			gs.FasterCharacterShadows = false
		}
		alpha, mobileFade, pictFade := computeInterpolation(now, snap.prevTime, snap.curTime, gs.MobileBlendAmount, gs.BlendAmount)
		// Production renders into a non-zero-origin subimage of the game buffer.
		const offsetX, offsetY = 37, 29
		backing := ebiten.NewImage(gameAreaSizeX+offsetX, gameAreaSizeY+offsetY)
		canvas := backing.SubImage(image.Rect(offsetX, offsetY, offsetX+gameAreaSizeX, offsetY+gameAreaSizeY)).(*ebiten.Image)
		canvas.Fill(color.RGBA{R: 28, G: 32, B: 38, A: 255})
		nightAlphaInited = false
		drawScene(canvas, 0, 0, snap, alpha, mobileFade, pictFade)
		if shaderLightingEnabled() {
			if sceneMayNeedLighting(snap) {
				addNightDarkSources(canvas.Bounds(), float32(alpha))
				applyLightingShader(canvas, frameLights, frameDarks, float32(alpha))
			} else {
				applyDetailedCharacterShadow(canvas)
			}
		}
		drawSpeechBubbles(canvas, snap, alpha, 1)
		name := fmt.Sprintf("setup_wizard_%d_%s.png", mode, setupWizardSceneName(mode))
		if err := writeSetupWizardScenePNG(filepath.Join(g.outputDir, name), canvas); err != nil {
			g.err = err
			break
		}
	}
	g.rendered = true
}

func setupWizardSceneHasObscuringPicture(snap drawSnapshot) bool {
	for _, pictures := range [][]framePicture{snap.picsNeg, snap.picsZero, snap.picsPos} {
		for _, picture := range pictures {
			if picture.obscuredPrev || picture.obscuredNow {
				return true
			}
		}
	}
	return false
}

func verifyLightingCoordinates() error {
	render := func(offsetX, offsetY, lightX, lightY int, interpolation float32) []byte {
		const width, height = 128, 96
		backing := ebiten.NewImage(width+offsetX, height+offsetY)
		bounds := image.Rect(offsetX, offsetY, offsetX+width, offsetY+height)
		canvas := backing.SubImage(bounds).(*ebiten.Image)
		canvas.Fill(color.RGBA{R: 32, G: 40, B: 48, A: 255})

		frameLightCasters = frameLightCasters[:0]
		applyLightingShader(canvas, []lightSource{{
			X:         float32(bounds.Min.X + lightX),
			Y:         float32(bounds.Min.Y + lightY),
			Radius:    36,
			R:         1,
			G:         0.5,
			B:         0.2,
			Intensity: 1,
		}}, nil, interpolation)

		pixels := make([]byte, 4*width*height)
		canvas.ReadPixels(pixels)
		return pixels
	}

	zeroOrigin := render(0, 0, 43, 37, 1)
	offsetOrigin := render(37, 29, 43, 37, 1)
	if !bytes.Equal(zeroOrigin, offsetOrigin) {
		differences, maximumDelta := 0, 0
		firstIndex := -1
		for index := range zeroOrigin {
			delta := int(zeroOrigin[index]) - int(offsetOrigin[index])
			if delta < 0 {
				delta = -delta
			}
			if delta == 0 {
				continue
			}
			if firstIndex < 0 {
				firstIndex = index
			}
			differences++
			maximumDelta = max(maximumDelta, delta)
		}
		if maximumDelta > 1 {
			pixel := firstIndex / 4
			return fmt.Errorf(
				"lighting changed when only the destination subimage origin changed: %d channel differences, maximum delta %d, first at (%d, %d) channel %d: %d != %d",
				differences, maximumDelta, pixel%128, pixel/128, firstIndex%4, zeroOrigin[firstIndex], offsetOrigin[firstIndex],
			)
		}
	}

	render(0, 0, 24, 48, 1)
	moved := render(0, 0, 104, 48, 0)
	brightest := 0
	for i := 4; i < len(moved); i += 4 {
		if int(moved[i])+int(moved[i+1])+int(moved[i+2]) > int(moved[brightest])+int(moved[brightest+1])+int(moved[brightest+2]) {
			brightest = i
		}
	}
	pixel := brightest / 4
	brightestX, brightestY := pixel%128, pixel/128
	if brightestX < 96 || brightestX > 112 || brightestY < 40 || brightestY > 56 {
		return fmt.Errorf("moving light remained near a stale position: brightest pixel is (%d, %d)", brightestX, brightestY)
	}
	return nil
}

func (g *setupWizardSceneRenderGame) Layout(_, _ int) (int, int) {
	return gameAreaSizeX, gameAreaSizeY
}

func writeSetupWizardScenePNG(path string, canvas *ebiten.Image) error {
	bounds := canvas.Bounds()
	pixels := make([]byte, 4*bounds.Dx()*bounds.Dy())
	canvas.ReadPixels(pixels)
	result := &image.RGBA{Pix: pixels, Stride: 4 * bounds.Dx(), Rect: image.Rect(0, 0, bounds.Dx(), bounds.Dy())}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(file, result); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
