package main

import (
	"image"
	"image/color"
	"math"
	"path/filepath"
	"sync"
	"time"

	"gothoom/climg"
	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
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
	nextWork      time.Time
	complete      bool
	precacheRun   bool
}{}

var startupLoadingDelay time.Duration

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
	return replacementEffectsEnabled() && replacementEffectsShaderInitializationPending()
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
	if startupLoader.drawnFrames > 0 && startupLoadingDelayPending(time.Now()) {
		return true
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

func startupLoadingDelayPending(now time.Time) bool {
	if startupLoadingDelay <= 0 {
		startupLoader.nextWork = time.Time{}
		return false
	}
	if startupLoader.nextWork.IsZero() {
		startupLoader.nextWork = now.Add(startupLoadingDelay)
		return true
	}
	if now.Before(startupLoader.nextWork) {
		return true
	}
	startupLoader.nextWork = time.Time{}
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
		return "Loading artwork"
	case startupLoadSounds:
		return "Loading sounds"
	case startupLoadInterface:
		return "Building interface"
	case startupLoadScripts:
		return "Loading scripts"
	default:
		switch {
		case !startupShaderLoader.lightingAttempted:
			return "Preparing lighting"
		case !startupShaderLoader.upscaleAttempted:
			return "Preparing artwork renderer"
		default:
			return "Preparing graphics"
		}
	}
}

type startupLoadingLine struct {
	text   string
	active bool
}

func startupLoadingLogLines() []startupLoadingLine {
	stages := []struct {
		stage    startupLoadStage
		active   string
		finished string
	}{
		{startupLoadImages, "Reading artwork archive", "Artwork archive ready"},
		{startupLoadSounds, "Reading sound archive", "Sound archive ready"},
		{startupLoadInterface, "Building interface", "Interface ready"},
		{startupLoadScripts, "Loading scripts", "Scripts loaded"},
	}

	lines := make([]startupLoadingLine, 0, 7)
	for _, entry := range stages {
		switch {
		case startupLoader.stage > entry.stage:
			lines = append(lines, startupLoadingLine{text: entry.finished})
		case startupLoader.stage == entry.stage:
			lines = append(lines, startupLoadingLine{text: entry.active, active: true})
			return lines
		default:
			return lines
		}
	}

	if startupShaderLoader.lightingAttempted {
		lines = append(lines, startupLoadingLine{text: "Lighting renderer ready"})
	} else {
		return append(lines, startupLoadingLine{text: "Preparing lighting renderer", active: true})
	}
	if startupShaderLoader.upscaleAttempted {
		lines = append(lines, startupLoadingLine{text: "Artwork renderer ready"})
	} else {
		return append(lines, startupLoadingLine{text: "Preparing artwork renderer", active: true})
	}
	if optionalEffectsShaderPending() {
		return append(lines, startupLoadingLine{text: "Preparing visual effects", active: true})
	}
	if replacementEffectsEnabled() {
		lines = append(lines, startupLoadingLine{text: "Visual effects ready"})
	}
	return lines
}

func visibleStartupLoadingLogLines(maxLines int) []startupLoadingLine {
	if maxLines <= 0 {
		return nil
	}
	lines := startupLoadingLogLines()
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines
}

func startupLoadingProgress() float64 {
	total := 6
	completed := min(int(startupLoader.stage), int(startupLoadCoreDone))
	if startupShaderLoader.lightingAttempted {
		completed++
	}
	if startupShaderLoader.upscaleAttempted {
		completed++
	}
	if replacementEffectsEnabled() {
		total++
		if !optionalEffectsShaderPending() {
			completed++
		}
	}
	if completed < 0 {
		completed = 0
	}
	if completed > total {
		completed = total
	}
	return float64(completed) / float64(total)
}

func startupLoadingPanelLayout(bounds image.Rectangle) image.Rectangle {
	if bounds.Empty() {
		return bounds
	}
	margin := max(16, min(bounds.Dx(), bounds.Dy())/18)
	panelWidth := min(840, bounds.Dx()-2*margin)
	panelHeight := min(430, bounds.Dy()-2*margin)
	panelWidth = max(1, panelWidth)
	panelHeight = max(1, panelHeight)
	left := bounds.Min.X + (bounds.Dx()-panelWidth)/2
	top := bounds.Min.Y + (bounds.Dy()-panelHeight)/2
	return image.Rect(left, top, left+panelWidth, top+panelHeight)
}

func drawStartupLoadingScreen(screen *ebiten.Image, label string) {
	bounds := screen.Bounds()
	drawStartupLoadingBackdrop(screen)
	panel := startupLoadingPanelLayout(bounds)
	if panel.Empty() {
		return
	}

	panelX := float32(panel.Min.X)
	panelY := float32(panel.Min.Y)
	panelW := float32(panel.Dx())
	panelH := float32(panel.Dy())
	vector.FillRect(screen, panelX-1, panelY-1, panelW+2, panelH+2, color.RGBA{94, 145, 142, 150}, false)
	vector.FillRect(screen, panelX, panelY, panelW, panelH, color.RGBA{10, 17, 23, 238}, false)
	vector.FillRect(screen, panelX, panelY, 5, panelH, color.RGBA{111, 184, 173, 255}, false)

	padding := math.Max(22, math.Min(34, float64(panel.Dx())*0.05))
	left := float64(panel.Min.X) + padding
	right := float64(panel.Max.X) - padding
	top := float64(panel.Min.Y)

	brandFace := startupLoadingFace(36, true)
	subtitleFace := startupLoadingFace(12, false)
	stageFace := startupLoadingFace(23, true)
	logFace := startupLoadingFace(15, false)
	if brandFace == nil || stageFace == nil || logFace == nil {
		return
	}
	drawStartupText(screen, "goThoom", brandFace, left, top+54, color.RGBA{239, 244, 242, 255})
	if subtitleFace != nil {
		drawStartupText(screen, "CLAN LORD CLIENT  /  STARTING", subtitleFace, left, top+102, color.RGBA{148, 166, 168, 255})
	}

	stageBaseline := top + 150
	drawStartupText(screen, label, stageFace, left, stageBaseline, color.RGBA{239, 244, 242, 255})
	progress := startupLoadingProgress()
	progressTop := float32(top + 181)
	progressWidth := float32(right - left)
	vector.FillRect(screen, float32(left), progressTop, progressWidth, 8, color.RGBA{36, 50, 57, 255}, false)
	if progress > 0 {
		vector.FillRect(screen, float32(left), progressTop, progressWidth*float32(progress), 8, color.RGBA{111, 184, 173, 255}, false)
	}

	activityTop := top + 227
	if subtitleFace != nil {
		drawStartupText(screen, "STARTUP ACTIVITY", subtitleFace, left, activityTop, color.RGBA{111, 184, 173, 255})
	}
	lineTop := activityTop + 33
	lineHeight := 29.0
	maxLines := int((float64(panel.Max.Y) - padding - lineTop) / lineHeight)
	if maxLines < 1 {
		return
	}
	lines := visibleStartupLoadingLogLines(maxLines)
	for i, line := range lines {
		lineColor := color.RGBA{171, 184, 185, 255}
		prefix := "[ok]"
		if line.active {
			lineColor = color.RGBA{225, 239, 235, 255}
			prefix = "[..]"
		}
		drawStartupText(screen, prefix+"  "+line.text, logFace, left, lineTop+float64(i)*lineHeight, lineColor)
	}
}

func startupLoadingFace(size float64, bold bool) text.Face {
	source := eui.FontSource()
	if bold {
		source = eui.BoldFontSource()
	}
	if source == nil {
		return nil
	}
	return &text.GoTextFace{Source: source, Size: size}
}

func drawStartupText(dst *ebiten.Image, value string, face text.Face, x, baseline float64, tint color.Color) {
	op := acquireTextDrawOpts()
	op.GeoM.Translate(x, baseline)
	op.ColorScale.ScaleWithColor(tint)
	text.Draw(dst, value, face, op)
	releaseTextDrawOpts(op)
}

func startupLoadingBackdropImage() *ebiten.Image {
	return embeddedSplashImg
}

func drawStartupLoadingBackdrop(screen *ebiten.Image) {
	bounds := screen.Bounds()
	screen.Fill(color.RGBA{9, 16, 21, 255})
	backdrop := startupLoadingBackdropImage()
	if backdrop != nil && !bounds.Empty() {
		iw := backdrop.Bounds().Dx()
		ih := backdrop.Bounds().Dy()
		if iw > 0 && ih > 0 {
			scale := math.Max(float64(bounds.Dx())/float64(iw), float64(bounds.Dy())/float64(ih))
			drawnWidth := float64(iw) * scale
			drawnHeight := float64(ih) * scale
			op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear, DisableMipmaps: true}
			op.GeoM.Scale(scale, scale)
			op.GeoM.Translate(
				float64(bounds.Min.X)+(float64(bounds.Dx())-drawnWidth)/2,
				float64(bounds.Min.Y)+(float64(bounds.Dy())-drawnHeight)/2,
			)
			screen.DrawImage(backdrop, op)
		}
	}
	vector.FillRect(screen, float32(bounds.Min.X), float32(bounds.Min.Y), float32(bounds.Dx()), float32(bounds.Dy()), color.RGBA{4, 9, 13, 165}, false)
}
