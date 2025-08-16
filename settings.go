package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"gothoom/climg"
	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
)

var gs settings = gsdef

var gsdef settings = settings{
	Version: 3,

	LastCharacter:     "",
	ClickToToggle:     false,
	KBWalkSpeed:       0.25,
	MainFontSize:      8,
	BubbleFontSize:    6,
	ConsoleFontSize:   12,
	ChatFontSize:      14,
	InventoryFontSize: 18,
	PlayersFontSize:   18,
	BubbleOpacity:     0.7,
	NameBgOpacity:     0.7,
	SpeechBubbles:     true,

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
	Volume:            0.125,
	Mute:              false,
	GameScale:         2,
	Theme:             "",
	MessagesToConsole: false,
	WindowTiling:      false,
	WindowSnapping:    false,
	AnyGameWindowSize: true,
	TitlebarMaximize:  true,
	IntegerScaling:    false,
	NoCaching:         false,
	PotatoComputer:    false,

	GameWindow:      WindowState{Open: true},
	InventoryWindow: WindowState{Open: true},
	PlayersWindow:   WindowState{Open: true},
	MessagesWindow:  WindowState{Open: true},
	ChatWindow:      WindowState{Open: true},

	imgPlanesDebug:      false,
	smoothingDebug:      false,
	pictAgainDebug:      false,
	pictIDDebug:         false,
	hideMoving:          false,
	hideMobiles:         false,
	vsync:               true,
	nightEffect:         true,
	precacheSounds:      false,
	precacheImages:      false,
	lateInputUpdates:    false,
	cacheWholeSheet:     true,
	smoothMoving:        false,
	dontShiftNewSprites: false,
	fastBars:            true,
	recordAssetStats:    false,
}

type settings struct {
	Version int

	LastCharacter     string
	ClickToToggle     bool
	KBWalkSpeed       float64
	MainFontSize      float64
	BubbleFontSize    float64
	ConsoleFontSize   float64
	ChatFontSize      float64
	InventoryFontSize float64
	PlayersFontSize   float64
	BubbleOpacity     float64
	NameBgOpacity     float64
	SpeechBubbles     bool

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
	AnyGameWindowSize bool // allow arbitrary game window sizes
	GameScale         float64
	Theme             string
	MessagesToConsole bool
	WindowTiling      bool
	WindowSnapping    bool
	TitlebarMaximize  bool
	IntegerScaling    bool

	GameWindow      WindowState
	InventoryWindow WindowState
	PlayersWindow   WindowState
	MessagesWindow  WindowState
	ChatWindow      WindowState

	imgPlanesDebug      bool
	smoothingDebug      bool
	pictAgainDebug      bool
	pictIDDebug         bool
	hideMoving          bool
	hideMobiles         bool
	vsync               bool
	nightEffect         bool
	precacheSounds      bool
	precacheImages      bool
	lateInputUpdates    bool
	cacheWholeSheet     bool
	smoothMoving        bool
	dontShiftNewSprites bool
	fastBars            bool
	recordAssetStats    bool
	NoCaching           bool
	PotatoComputer      bool
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

	if newGS.Version == 2 || newGS.Version == 3 {
		gs = newGS
	}

	clampWindowSettings()
	return true
}

func applySettings() {
	// Fixed-size mode is deprecated; force any-size mode on.
	gs.AnyGameWindowSize = true
	eui.SetWindowTiling(gs.WindowTiling)
	eui.SetWindowSnapping(gs.WindowSnapping)
	eui.SetPotatoMode(gs.PotatoComputer)
	climg.SetPotatoMode(gs.PotatoComputer)
	if clImages != nil {
		clImages.Denoise = gs.DenoiseImages
		clImages.DenoiseSharpness = gs.DenoiseSharpness
		clImages.DenoisePercent = gs.DenoisePercent
	}
	ebiten.SetVsyncEnabled(gs.vsync)
	ebiten.SetFullscreen(gs.Fullscreen)
	ebiten.SetWindowFloating(gs.Fullscreen)
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
	if st.Size.X < eui.MinWindowSize || st.Size.Y < eui.MinWindowSize {
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
	NoCaching       bool
}

var (
	ultraLowPreset = qualityPreset{
		DenoiseImages:   false,
		MotionSmoothing: false,
		BlendMobiles:    false,
		BlendPicts:      false,
		NoCaching:       true,
	}
	lowPreset = qualityPreset{
		DenoiseImages:   false,
		MotionSmoothing: false,
		BlendMobiles:    false,
		BlendPicts:      false,
		NoCaching:       false,
	}
	standardPreset = qualityPreset{
		DenoiseImages:   true,
		MotionSmoothing: true,
		BlendMobiles:    false,
		BlendPicts:      false,
		NoCaching:       false,
	}
	highPreset = qualityPreset{
		DenoiseImages:   true,
		MotionSmoothing: true,
		BlendMobiles:    false,
		BlendPicts:      true,
		NoCaching:       false,
	}
	ultimatePreset = qualityPreset{
		DenoiseImages:   true,
		MotionSmoothing: true,
		BlendMobiles:    true,
		BlendPicts:      true,
		NoCaching:       false,
	}
)

func applyQualityPreset(name string) {
	var p qualityPreset
	switch name {
	case "Ultra Low":
		p = ultraLowPreset
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
	gs.NoCaching = p.NoCaching
	if gs.NoCaching {
		gs.precacheSounds = false
		gs.precacheImages = false
	}

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
	if precacheSoundCB != nil {
		precacheSoundCB.Disabled = gs.NoCaching
		if gs.NoCaching {
			precacheSoundCB.Checked = false
		}
	}
	if precacheImageCB != nil {
		precacheImageCB.Disabled = gs.NoCaching
		if gs.NoCaching {
			precacheImageCB.Checked = false
		}
	}
	if noCacheCB != nil {
		noCacheCB.Checked = gs.NoCaching
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
		gs.BlendPicts == p.BlendPicts &&
		gs.NoCaching == p.NoCaching
}

func detectQualityPreset() int {
	switch {
	case matchesPreset(ultraLowPreset):
		return 0
	case matchesPreset(lowPreset):
		return 1
	case matchesPreset(standardPreset):
		return 2
	case matchesPreset(highPreset):
		return 3
	case matchesPreset(ultimatePreset):
		return 4
	default:
		return 5
	}
}
