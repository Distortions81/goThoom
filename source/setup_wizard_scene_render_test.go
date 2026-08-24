package main

import (
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
	gs.DetailedCharacterShadows = true
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
	initFont()
	for _, mode := range []setupWizardSceneMode{setupWizardSceneDay, setupWizardSceneIndoor, setupWizardSceneNight, setupWizardSceneMotion} {
		setupWizardSceneModeValue = mode
		setupWizardPage = 6
		if mode == setupWizardSceneIndoor {
			setupWizardPage = 3
		}
		setupWizardSceneStarted = time.Unix(1000, 0)
		now := setupWizardSceneStarted.Add(650 * time.Millisecond)
		var snap drawSnapshot
		prepareSetupWizardSceneSnapshot(&snap, now)
		alpha, mobileFade, pictFade := computeInterpolation(now, snap.prevTime, snap.curTime, gs.MobileBlendAmount, gs.BlendAmount)
		// Production renders into a non-zero-origin subimage of the game buffer.
		const offsetX, offsetY = 37, 29
		backing := ebiten.NewImage(gameAreaSizeX+offsetX, gameAreaSizeY+offsetY)
		canvas := backing.SubImage(image.Rect(offsetX, offsetY, offsetX+gameAreaSizeX, offsetY+gameAreaSizeY)).(*ebiten.Image)
		canvas.Fill(color.RGBA{R: 28, G: 32, B: 38, A: 255})
		nightAlphaInited = false
		havePrev = false
		prevLights = nil
		prevDarks = nil
		drawScene(canvas, 0, 0, snap, alpha, mobileFade, pictFade)
		if gs.ShaderLighting {
			addNightDarkSources(canvas.Bounds(), float32(alpha))
			applyLightingShader(canvas, frameLights, frameDarks, float32(alpha))
		}
		drawSetupWizardSceneLabel(canvas, 1)
		drawSpeechBubbles(canvas, snap, alpha, 1)
		name := fmt.Sprintf("setup_wizard_%d_%s.png", mode, setupWizardSceneName(mode))
		if err := writeSetupWizardScenePNG(filepath.Join(g.outputDir, name), canvas); err != nil {
			g.err = err
			break
		}
	}
	g.rendered = true
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
