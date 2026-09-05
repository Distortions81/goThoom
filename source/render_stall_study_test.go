package main

import (
	"encoding/json"
	"image"
	"image/png"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
)

// Run alone with GOTHOOM_RENDER_STUDY=/tmp/render.json on a hardware-backed
// display. This deliberately changes process-wide rendering state. It measures
// a warmed, fixed movie scene, not movie parsing, uploads, or GPU timestamps.
func TestRenderStallStudy(t *testing.T) {
	output := os.Getenv("GOTHOOM_RENDER_STUDY")
	if output == "" {
		t.Skip("opt-in hardware rendering study")
	}
	scale, _ := strconv.Atoi(os.Getenv("GOTHOOM_RENDER_SCALE"))
	if scale != 2 && scale != 3 && scale != 4 {
		scale = 2
	}
	artworkScale := scale
	if raw := os.Getenv("GOTHOOM_RENDER_ARTWORK_SCALE"); raw != "" {
		var err error
		artworkScale, err = strconv.Atoi(raw)
		if err != nil || artworkScale < 2 || artworkScale > 4 {
			t.Fatalf("invalid artwork scale %q", raw)
		}
	}
	minimumSamples := 600
	if raw := os.Getenv("GOTHOOM_RENDER_SAMPLES"); raw != "" {
		var err error
		minimumSamples, err = strconv.Atoi(raw)
		if err != nil || minimumSamples < 120 {
			t.Fatalf("GOTHOOM_RENDER_SAMPLES must be at least 120")
		}
	}
	seconds := 0.0
	if raw := os.Getenv("GOTHOOM_RENDER_SECONDS"); raw != "" {
		var err error
		seconds, err = strconv.ParseFloat(raw, 64)
		if err != nil || seconds <= 0 || seconds > 120 {
			t.Fatalf("invalid GOTHOOM_RENDER_SECONDS %q: use (0,120]", raw)
		}
	}
	if raw := os.Getenv("GOTHOOM_RENDER_CASES"); raw != "" {
		wanted := strings.Split(raw, ",")
		for _, name := range wanted {
			found := false
			for _, known := range renderStallCases {
				if name == known {
					found = true
				}
			}
			if !found {
				t.Fatalf("unknown render case %q", name)
			}
		}
		renderStallCases = wanted
	}
	if os.Getenv("GOTHOOM_RENDER_REVERSE") == "1" {
		for i, j := 1, len(renderStallCases)-2; i < j; i, j = i+1, j-1 {
			renderStallCases[i], renderStallCases[j] = renderStallCases[j], renderStallCases[i]
		}
	}
	fullscreen := os.Getenv("GOTHOOM_RENDER_FULLSCREEN") == "1"
	vsync := os.Getenv("GOTHOOM_RENDER_VSYNC") == "1"
	docked := os.Getenv("GOTHOOM_RENDER_DOCKED") != "0"
	fixture := loadTourPerformanceFixture(t)
	gs = gsdef
	gs.GameScale = float64(scale)
	gs.SpriteUpscale = artworkScale
	gs.UIScale = float64(scale) / 2
	gs.VSync = vsync
	gs.PowerSaveBackground = false
	gs.PowerSaveAlways = false
	gs.TiledWindows = docked
	gs.MessagesToConsole = false
	gs.forceNightLevel = 50
	gNight.Shadows = 50
	gNight.Azimuth = 135
	clImages = fixture.images
	movieMode, playingMovie = true, true
	blockSound, blockMusic, blockTTS = true, true, true
	playerName = fixture.playerName
	dataDirPath = t.TempDir()
	restoreDrawState(fixture.busyScene)
	stateMu.Lock()
	prepareRenderCacheLocked()
	stateMu.Unlock()
	worldStateGeneration.Add(1)
	initFont()
	if err := ReloadLightingShader(); err != nil {
		t.Fatal(err)
	}
	if err := ReloadSpriteUpscaleShader(); err != nil {
		t.Fatal(err)
	}
	startupLoader.complete = true
	ebiten.SetVsyncEnabled(vsync)
	ebiten.SetTPS(ebiten.SyncWithFPS)
	ebiten.SetWindowSize(960, 540)
	ebiten.SetFullscreen(fullscreen)
	eui.SetScreenSize(960*scale, 540*scale)
	eui.SetUIScale(float32(scale) / 2)
	gameWin = newGameRenderWindow()
	gameWin.Padding, gameWin.TitleHeight = 0, 0
	gameWin.Size = eui.Point{X: float32(gameAreaSizeX * scale), Y: float32(gameAreaSizeY * scale)}
	gameWin.Position = eui.Point{X: float32(200 * scale)}
	gameWin.AddWindow(true)
	gameWin.MarkOpen()
	makeConsoleWindow()
	makeChatWindow()
	makeInventoryWindow()
	makePlayersWindow()
	panes := []*eui.WindowData{consoleWin, chatWin, inventoryWin, playersWin}
	for i, w := range panes {
		w.ClearZone()
		w.SetSize(eui.Point{X: 175 * float32(scale), Y: 245 * float32(scale)})
		w.SetPos(eui.Point{X: float32((i % 2) * 775 * scale), Y: float32((i / 2) * 270 * scale)})
		w.MarkOpen()
	}
	prepareTiledWorkspaceWindowChrome()
	finishTiledWorkspaceWindowChrome()
	g := &renderStallStudy{scale: scale, panes: panes, output: output, minimumDuration: time.Duration(seconds * float64(time.Second)), minimumSamples: minimumSamples}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
	if len(g.results) != len(renderStallCases) {
		t.Fatalf("render study interrupted: completed %d of %d cases", len(g.results), len(renderStallCases))
	}
	report := map[string]any{"worldBounds": gameImage.Bounds().String(), "worldScale": worldScale, "fixture": performanceTourFixture, "pictures": len(fixture.busyScene.pictures), "mobiles": len(fixture.busyScene.mobiles), "scale": scale, "nightPercent": 50, "animatedDefault": gsdef.AnimatedChatBubbles, "artworkUpscale": artworkScale, "minimumSamplesPerCase": minimumSamples, "minimumSecondsPerCase": seconds, "warmupFrames": 90, "fullscreen": fullscreen, "goVersion": runtime.Version(), "goos": runtime.GOOS, "goarch": runtime.GOARCH, "cases": g.results}
	report["vsync"] = vsync
	report["docked"] = docked
	report["gameWindowNoBackground"] = gameWin.NoBGColor
	monitor := ebiten.Monitor()
	mw, mh := monitor.Size()
	report["monitor"] = map[string]any{"name": monitor.Name(), "widthDP": mw, "heightDP": mh, "deviceScaleFactor": monitor.DeviceScaleFactor()}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, data, 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s", output)
}

type renderStallResult struct {
	Samples                     int
	GraphicsLibrary             string
	UIRepaints                  []eui.RepaintStats
	Name                        string
	Submission, FrameInterval   map[string]float64
	Lights, Darks, LightShadows int
	GPUBytes                    int64
}
type renderStallStudy struct {
	minimumSamples          int
	minimumDuration         time.Duration
	measurementStart        time.Time
	uiStart                 map[string]eui.RepaintStats
	output                  string
	err                     error
	game                    Game
	scale, index, frame     int
	panes                   []*eui.WindowData
	previous, lastUIRefresh time.Time
	cpu, intervals          []float64
	results                 []renderStallResult
}

var renderStallCases = []string{"baseline", "animated_bubbles", "all_shaders_off", "lighting_off", "fast_shadows", "shadows_off", "bubble_animation_off", "bubbles_off", "ui_refresh_5hz", "ui_refresh_5hz_unbatched", "ui_hidden", "baseline_repeat"}

func (g *renderStallStudy) Layout(_, _ int) (int, int) { return 960 * g.scale, 540 * g.scale }
func (g *renderStallStudy) Update() error {
	if g.index == len(renderStallCases) {
		return ebiten.Termination
	}
	return nil
}
func (g *renderStallStudy) Draw(screen *ebiten.Image) {
	if g.index >= len(renderStallCases) {
		return
	}
	name := renderStallCases[g.index]
	if g.frame == 0 {
		gs.ShadersEnabled = name != "all_shaders_off"
		gs.ShaderLighting = name != "lighting_off"
		gs.FasterCharacterShadows = name == "fast_shadows"
		gs.CharacterShadows = name != "shadows_off"
		gs.SpeechBubbles = name != "bubbles_off"
		gs.AnimatedChatBubbles = gsdef.AnimatedChatBubbles
		if name == "animated_bubbles" {
			gs.AnimatedChatBubbles = true
		}
		if name == "bubble_animation_off" {
			gs.AnimatedChatBubbles = false
		}
		for _, w := range g.panes {
			w.DeferRepaint = name == "ui_refresh_5hz"
			if name == "ui_hidden" {
				w.Close()
			} else {
				w.MarkOpen()
			}
		}
		g.cpu, g.intervals = nil, nil
		g.previous = time.Time{}
		g.lastUIRefresh = time.Time{}
	}
	now := time.Now()
	if g.frame == 90 {
		g.measurementStart = now
		g.uiStart = make(map[string]eui.RepaintStats)
		for _, stats := range eui.WindowRepaintStats() {
			g.uiStart[stats.Title] = stats
		}
	}
	if (name == "ui_refresh_5hz" || name == "ui_refresh_5hz_unbatched") && now.Sub(g.lastUIRefresh) >= 200*time.Millisecond {
		for _, w := range g.panes {
			w.RefreshWithReason("study 5 Hz refresh")
		}
		g.lastUIRefresh = now
	}
	if g.frame >= 90 {
		g.intervals = append(g.intervals, float64(now.Sub(g.previous))/float64(time.Millisecond))
	}
	g.previous = now
	start := time.Now()
	g.game.Draw(screen)
	if g.frame >= 90 {
		g.cpu = append(g.cpu, float64(time.Since(start))/float64(time.Millisecond))
	}
	g.frame++
	if g.frame >= 90+g.minimumSamples && time.Since(g.measurementStart) >= g.minimumDuration {
		var info ebiten.DebugInfo
		ebiten.ReadDebugInfo(&info)
		stats := eui.WindowRepaintStats()
		for i := range stats {
			before := g.uiStart[stats[i].Title]
			stats[i].Count -= before.Count
			stats[i].Pixels -= before.Pixels
			stats[i].DeferredFrames -= before.DeferredFrames
		}
		g.results = append(g.results, renderStallResult{Samples: len(g.cpu), GraphicsLibrary: info.GraphicsLibrary.String(), UIRepaints: stats, Name: name, Submission: renderStallQuantiles(g.cpu), FrameInterval: renderStallQuantiles(g.intervals), Lights: len(frameLights), Darks: len(frameDarks), LightShadows: len(frameLightShadows), GPUBytes: info.TotalGPUImageMemoryUsageInBytes})
		if g.index == 0 {
			pixels := image.NewRGBA(screen.Bounds())
			screen.ReadPixels(pixels.Pix)
			file, err := os.Create(g.output + ".png")
			if err != nil {
				g.err = err
			} else {
				g.err = png.Encode(file, pixels)
				file.Close()
			}
		}
		g.frame = 0
		g.index++
	}
}
func renderStallQuantiles(values []float64) map[string]float64 {
	sort.Float64s(values)
	q := func(p float64) float64 { return values[int(float64(len(values)-1)*p)] }
	over60, over30 := 0, 0
	for _, v := range values {
		if v > 1000.0/60 {
			over60++
		}
		if v > 1000.0/30 {
			over30++
		}
	}
	return map[string]float64{"over_16_67ms_pct": 100 * float64(over60) / float64(len(values)), "over_33_33ms_pct": 100 * float64(over30) / float64(len(values)), "p50_ms": q(.5), "p95_ms": q(.95), "p99_ms": q(.99), "max_ms": q(1)}
}
