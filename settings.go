package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"go_client/eui"

	"github.com/hajimehoshi/ebiten/v2"
)

var gs settings = gsdef

var gsdef settings = settings{
	Version: 2,

	LastCharacter:   "",
	ClickToToggle:   false,
	KBWalkSpeed:     0.25,
	MainFontSize:    8,
	BubbleFontSize:  6,
	ConsoleFontSize: 10,
	ChatFontSize:    10,
	BubbleOpacity:   0.7,
	NameBgOpacity:   0.7,
	SpeechBubbles:   true,

	MotionSmoothing:   true,
	BlendMobiles:      false,
	BlendPicts:        false,
	BlendAmount:       1.0,
	MobileBlendAmount: 0.33,
	MobileBlendFrames: 10,
	PictBlendFrames:   10,
	DenoiseImages:     false,
	DenoiseSharpness:  4.0,
	DenoisePercent:    0.2,
	ShowFPS:           true,
	UIScale:           1.0,
	Fullscreen:        false,
	Volume:            0.5,
	Mute:              false,
	GameScale:         2,
	Theme:             "",
	MessagesToConsole: false,
	WindowTiling:      true,
	AnyGameWindowSize: false,

	GameWindow:      WindowState{Open: true},
	InventoryWindow: WindowState{Open: true},
	PlayersWindow:   WindowState{Open: true},
	MessagesWindow:  WindowState{Open: true},
	ChatWindow:      WindowState{Open: true},

	imgPlanesDebug:   false,
	smoothingDebug:   false,
	hideMoving:       false,
	hideMobiles:      false,
	vsync:            true,
	nightEffect:      true,
	precacheSounds:   false,
	precacheImages:   false,
	textureFiltering: false,
	lateInputUpdates: false,
	cacheWholeSheet:  true,
	smoothMoving:     true,
	fastBars:         true,
	recordAssetStats: false,
}

type settings struct {
	Version int

	LastCharacter   string
	ClickToToggle   bool
	KBWalkSpeed     float64
	MainFontSize    float64
	BubbleFontSize  float64
	ConsoleFontSize float64
	ChatFontSize    float64
	BubbleOpacity   float64
	NameBgOpacity   float64
	SpeechBubbles   bool

	MotionSmoothing   bool
	BlendMobiles      bool
	BlendPicts        bool
	BlendAmount       float64
	MobileBlendAmount float64
	MobileBlendFrames int
	PictBlendFrames   int
	DenoiseImages     bool
	DenoiseSharpness  float64
	DenoisePercent    float64
	ShowFPS           bool
	UIScale           float64
	Fullscreen        bool
	Volume            float64
	Mute              bool
	AnyGameWindowSize bool
	GameScale         float64
	Theme             string
	MessagesToConsole bool
	WindowTiling      bool

	GameWindow      WindowState
	InventoryWindow WindowState
	PlayersWindow   WindowState
	MessagesWindow  WindowState
	ChatWindow      WindowState

	imgPlanesDebug   bool
	smoothingDebug   bool
	hideMoving       bool
	hideMobiles      bool
	vsync            bool
	nightEffect      bool
	precacheSounds   bool
	precacheImages   bool
	textureFiltering bool
	lateInputUpdates bool
	cacheWholeSheet  bool
	smoothMoving     bool
	fastBars         bool
	recordAssetStats bool
}

var (
	settingsDirty    bool
	lastSettingsSave = time.Now()
)

type WindowPoint struct {
	X float64
	Y float64
}

type WindowState struct {
	Open     bool
	Position WindowPoint
	Size     WindowPoint
}

const settingsFile = "settings.json"

func loadSettings() bool {
	path := filepath.Join(dataDirPath, settingsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	newGS := gsdef
	if err := json.Unmarshal(data, &newGS); err != nil {
		return false
	}
	if newGS.Theme == "" {
		newGS.Theme = gsdef.Theme
	}

	if gs.Version == 2 {
		gs = newGS
	}

	clampWindowSettings()
	return true
}

func applySettings() {
	eui.SetWindowTiling(gs.WindowTiling)
	if clImages != nil {
		clImages.Denoise = gs.DenoiseImages
		clImages.DenoiseSharpness = gs.DenoiseSharpness
		clImages.DenoisePercent = gs.DenoisePercent
	}
	if gs.textureFiltering {
		drawFilter = ebiten.FilterLinear
	} else {
		drawFilter = ebiten.FilterNearest
	}
	ebiten.SetVsyncEnabled(gs.vsync)
	ebiten.SetFullscreen(gs.Fullscreen)
	initFont()
	updateSoundVolume()
}

func saveSettings() {
	data, err := json.MarshalIndent(gs, "", "  ")
	if err != nil {
		logError("save settings: %v", err)
		return
	}
	path := filepath.Join(dataDirPath, settingsFile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		logError("save settings: %v", err)
	}
}

func syncWindowSettings() bool {
	changed := false
	if syncWindow(gameWin, &gs.GameWindow) {
		changed = true
	}
	if syncWindow(inventoryWin, &gs.InventoryWindow) {
		changed = true
	}
	if syncWindow(playersWin, &gs.PlayersWindow) {
		changed = true
	}
	if syncWindow(consoleWin, &gs.MessagesWindow) {
		changed = true
	}
	if chatWin != nil {
		if syncWindow(chatWin, &gs.ChatWindow) {
			changed = true
		}
	} else if gs.ChatWindow.Open {
		gs.ChatWindow.Open = false
		changed = true
	}
	return changed
}

func syncWindow(win *eui.WindowData, state *WindowState) bool {
	if win == nil {
		if state.Open {
			state.Open = false
			return true
		}
		return false
	}
	changed := false
	if state.Open != win.IsOpen() {
		state.Open = win.IsOpen()
		changed = true
	}
	pos := WindowPoint{X: float64(win.Position.X), Y: float64(win.Position.Y)}
	if state.Position != pos {
		state.Position = pos
		changed = true
	}
	size := WindowPoint{X: float64(win.Size.X), Y: float64(win.Size.Y)}
	if state.Size != size {
		state.Size = size
		changed = true
	}
	return changed
}

func clampWindowSettings() {
	sx, sy := eui.ScreenSize()
	states := []*WindowState{&gs.GameWindow, &gs.InventoryWindow, &gs.PlayersWindow, &gs.MessagesWindow, &gs.ChatWindow}
	for _, st := range states {
		clampWindowState(st, float64(sx), float64(sy))
	}
}

func clampWindowState(st *WindowState, sx, sy float64) {
	if st.Size.X < 100 || st.Size.Y < 100 {
		st.Position = WindowPoint{}
		st.Size = WindowPoint{}
		return
	}
	if st.Size.X > sx {
		st.Size.X = sx
	}
	if st.Size.Y > sy {
		st.Size.Y = sy
	}
	maxX := sx - st.Size.X
	maxY := sy - st.Size.Y
	if st.Position.X < 0 {
		st.Position.X = 0
	} else if st.Position.X > maxX {
		st.Position.X = maxX
	}
	if st.Position.Y < 0 {
		st.Position.Y = 0
	} else if st.Position.Y > maxY {
		st.Position.Y = maxY
	}
}

type qualityPreset struct {
	DenoiseImages   bool
	MotionSmoothing bool
	BlendMobiles    bool
	BlendPicts      bool
}

var (
	lowPreset = qualityPreset{
		DenoiseImages:   false,
		MotionSmoothing: false,
		BlendMobiles:    false,
		BlendPicts:      false,
	}
	standardPreset = qualityPreset{
		DenoiseImages:   true,
		MotionSmoothing: true,
		BlendMobiles:    false,
		BlendPicts:      false,
	}
	highPreset = qualityPreset{
		DenoiseImages:   true,
		MotionSmoothing: true,
		BlendMobiles:    false,
		BlendPicts:      true,
	}
	ultimatePreset = qualityPreset{
		DenoiseImages:   true,
		MotionSmoothing: true,
		BlendMobiles:    true,
		BlendPicts:      true,
	}
)

func applyQualityPreset(name string) {
	var p qualityPreset
	switch name {
	case "Low":
		p = lowPreset
	case "Standard":
		p = standardPreset
	case "High":
		p = highPreset
	case "Ultimate":
		p = ultimatePreset
	default:
		return
	}

	gs.DenoiseImages = p.DenoiseImages
	gs.MotionSmoothing = p.MotionSmoothing
	gs.BlendMobiles = p.BlendMobiles
	gs.BlendPicts = p.BlendPicts

	if denoiseCB != nil {
		denoiseCB.Checked = gs.DenoiseImages
	}
	if motionCB != nil {
		motionCB.Checked = gs.MotionSmoothing
	}
	if animCB != nil {
		animCB.Checked = gs.BlendMobiles
	}
	if pictBlendCB != nil {
		pictBlendCB.Checked = gs.BlendPicts
	}

	applySettings()
	clearCaches()
	settingsDirty = true
	if qualityWin != nil {
		qualityWin.Refresh()
	}
	if graphicsWin != nil {
		graphicsWin.Refresh()
	}
	if debugWin != nil {
		debugWin.Refresh()
	}
}

func matchesPreset(p qualityPreset) bool {
	return gs.DenoiseImages == p.DenoiseImages &&
		gs.MotionSmoothing == p.MotionSmoothing &&
		gs.BlendMobiles == p.BlendMobiles &&
		gs.BlendPicts == p.BlendPicts
}

func detectQualityPreset() int {
	switch {
	case matchesPreset(lowPreset):
		return 0
	case matchesPreset(standardPreset):
		return 1
	case matchesPreset(highPreset):
		return 2
	case matchesPreset(ultimatePreset):
		return 3
	default:
		return 4
	}
}
