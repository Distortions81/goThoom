package main

import (
	"image"
	"image/color"
	"path/filepath"
	"sync"

	"gothoom/climg"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

type startupLoadStage uint8

const (
	startupLoadImages startupLoadStage = iota
	startupLoadSounds
	startupLoadInterface
	startupLoadScripts
	startupLoadCoreDone
)

var startupLoader = struct {
	stage         startupLoadStage
	drawnFrames   uint64
	lastWorkFrame uint64
	complete      bool
	precacheRun   bool
}{}

var gameStartedOnce sync.Once

var startupShaderLoader = struct {
	lastCompileFrame  uint64
	lightingAttempted bool
	upscaleAttempted  bool
}{}

func startupShaderPending() bool {
	if lightingShader == nil && !startupShaderLoader.lightingAttempted {
		return true
	}
	if spriteUpscaleShader == nil && !startupShaderLoader.upscaleAttempted {
		return true
	}
	return optionalEffectsShaderPending()
}

func optionalEffectsShaderPending() bool {
	return gs.ReplacementEffects && replacementEffectsShaderInitializationPending()
}

func updateStartupShaders() {
	if !uiReady {
		return
	}
	if !startupShaderPending() {
		return
	}
	// Never compile before the window has presented at least one frame, and
	// never compile two shaders without a Draw between them.
	if startupLoader.drawnFrames == 0 || startupShaderLoader.lastCompileFrame == startupLoader.drawnFrames {
		return
	}
	startupShaderLoader.lastCompileFrame = startupLoader.drawnFrames

	var err error
	switch {
	case lightingShader == nil && !startupShaderLoader.lightingAttempted:
		startupShaderLoader.lightingAttempted = true
		err = ReloadLightingShader()
	case spriteUpscaleShader == nil && !startupShaderLoader.upscaleAttempted:
		startupShaderLoader.upscaleAttempted = true
		err = ReloadSpriteUpscaleShader()
	case optionalEffectsShaderPending():
		err = loadNextReplacementEffectShader()
	}
	if err != nil {
		logError("shader initialization failed: %v", err)
	}
}

// updateStartupLoading performs at most one blocking startup task for each
// frame that reached Draw. This gets a useful window on screen before reading
// the large archives or compiling the core Kage shaders.
func updateStartupLoading() bool {
	if startupLoader.complete {
		// Replacement effects remain optional and may be enabled after startup.
		if optionalEffectsShaderPending() {
			updateStartupShaders()
		}
		return false
	}
	if startupLoader.stage < startupLoadCoreDone {
		if startupLoader.drawnFrames == 0 || startupLoader.lastWorkFrame == startupLoader.drawnFrames {
			return true
		}
		startupLoader.lastWorkFrame = startupLoader.drawnFrames
		switch startupLoader.stage {
		case startupLoadImages:
			loadStartupImages()
		case startupLoadSounds:
			loadStartupSounds()
		case startupLoadInterface:
			once.Do(initGame)
		case startupLoadScripts:
			loadScripts()
		}
		startupLoader.stage++
		if startupLoader.stage < startupLoadCoreDone || startupShaderPending() {
			return true
		}
	}

	updateStartupShaders()
	if startupShaderPending() {
		return true
	}
	startupLoader.complete = true
	gameStartedOnce.Do(func() { close(gameStarted) })
	return false
}

func loadStartupImages() {
	var (
		images *climg.CLImages
		err    error
	)
	if isWASM && len(wasmCLImagesData) > 0 {
		images, err = climg.LoadBytes(wasmCLImagesData)
	} else {
		images, err = climg.Load(filepath.Join(dataDirPath, CL_ImagesFile))
	}
	if err != nil {
		logError("failed to load CL_Images: %v", err)
		return
	}
	clImages = images
	clImages.SetDenoise(gs.DenoiseImages, gs.DenoiseSharpness, gs.DenoiseAmount)
	clImages.SetGammaCorrection(gs.SpriteGammaCorrection, gs.SpriteGamma, gs.MonitorGamma)
	prepareClassicSplash()
}

func loadStartupSounds() {
	sounds, err := loadCLSoundsArchive()
	if err != nil {
		logError("failed to load CL_Sounds: %v", err)
		return
	}
	replaceCLSoundsArchive(sounds)
	if gs.PrecacheSounds && !startupLoader.precacheRun {
		startupLoader.precacheRun = true
		go precacheSounds()
	}
}

func shaderCompilationFrameDrawn() {
	startupLoader.drawnFrames++
}

func shouldDrawStartupLoadingScreen() bool {
	if startupLoader.complete {
		return false
	}
	return startupLoader.stage < startupLoadCoreDone || startupShaderPending()
}

func startupLoadingLabel() string {
	switch startupLoader.stage {
	case startupLoadImages:
		return "Loading CL_Images..."
	case startupLoadSounds:
		return "Loading CL_Sounds..."
	case startupLoadInterface:
		return "Loading interface..."
	case startupLoadScripts:
		return "Loading scripts..."
	default:
		return "Loading shaders..."
	}
}

func drawStartupLoadingScreen(screen *ebiten.Image, label string) {
	screen.Fill(dimmedScreenBG)
	if mainFontBold == nil {
		return
	}
	w, h := text.Measure(label, mainFontBold, 0)
	scale, x, y := startupLoadingTextLayout(screen.Bounds(), w, h)
	op := acquireTextDrawOpts()
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(color.White)
	text.Draw(screen, label, mainFontBold, op)
	releaseTextDrawOpts(op)
}

func startupLoadingTextLayout(bounds image.Rectangle, textWidth, textHeight float64) (scale, x, baselineY float64) {
	if textWidth <= 0 || textHeight <= 0 || bounds.Empty() {
		return 1, float64(bounds.Min.X), float64(bounds.Min.Y)
	}
	width := float64(bounds.Dx())
	height := float64(bounds.Dy())
	// Aim for text about 5.5% of the window width. The caps keep very small
	// windows readable and prevent oversized text on high-DPI or wide displays.
	targetHeight := width * 0.055
	if targetHeight < 36 {
		targetHeight = 36
	}
	if targetHeight > 144 {
		targetHeight = 144
	}
	scale = targetHeight / textHeight
	if scaledWidth := textWidth * scale; scaledWidth > width*0.8 {
		scale = width * 0.8 / textWidth
	}
	scaledWidth := textWidth * scale
	scaledHeight := textHeight * scale
	x = float64(bounds.Min.X) + (width-scaledWidth)/2
	// text.Draw positions the face on its baseline, so include the scaled text
	// height to center its visible bounds vertically.
	baselineY = float64(bounds.Min.Y) + (height+scaledHeight)/2
	return scale, x, baselineY
}
