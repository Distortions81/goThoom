package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/Distortions81/EUI/eui"
	"github.com/hajimehoshi/ebiten/v2"
)

var gs settings = gsdef

var gsdef settings = settings{
	Version: 2,

	LastCharacter:  "",
	ClickToToggle:  false,
	KBWalkSpeed:    0.25,
	MainFontSize:   8,
	BubbleFontSize: 6,
	BubbleOpacity:  0.7,
	NameBgOpacity:  0.7,
	SpeechBubbles:  true,

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
	MessagesToConsole: false,
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
	fastSound:        true,
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

	LastCharacter  string
	ClickToToggle  bool
	KBWalkSpeed    float64
	MainFontSize   float64
	BubbleFontSize float64
	BubbleOpacity  float64
	NameBgOpacity  float64
	SpeechBubbles  bool

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

	GameWindow      WindowState
	InventoryWindow WindowState
	PlayersWindow   WindowState
	MessagesWindow  WindowState
	ChatWindow      WindowState

	imgPlanesDebug    bool
	smoothingDebug    bool
	hideMoving        bool
	hideMobiles       bool
	vsync             bool
	fastSound         bool
	nightEffect       bool
	precacheSounds    bool
	precacheImages    bool
	textureFiltering  bool
	lateInputUpdates  bool
	cacheWholeSheet   bool
	smoothMoving      bool
	fastBars          bool
	MessagesToConsole bool
	recordAssetStats  bool
}

var (
	settingsDirty    bool
	lastSettingsSave = time.Now()
)

// WindowPoint represents a normalized point on the screen where 0 and 1
// correspond to the minimum and maximum screen extents respectively.
type WindowPoint struct {
	X float64
	Y float64
}

// WindowState stores window visibility and geometry using normalized values in
// the range [0,1].
type WindowState struct {
	Open bool
	// Position holds the normalized top-left corner of the window.
	Position WindowPoint
	// Size represents the normalized width and height of the window.
	Size WindowPoint
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

	if gs.Version == 2 {
		gs = newGS
	}

	clampWindowSettings()

	if !gs.fastSound {
		initSinc()
	}

	return true
}

func applySettings() {
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
	if !gs.fastSound {
		initSinc()
	}
	ebiten.SetVsyncEnabled(gs.vsync)
	ebiten.SetFullscreen(gs.Fullscreen)
	initFont()
	updateSoundVolume()
}

func saveSettings() {
	syncWindowSettings()
	data, err := json.MarshalIndent(gs, "", "  ")
	if err != nil {
		log.Printf("save settings: %v", err)
		return
	}
	path := filepath.Join(dataDirPath, settingsFile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("save settings: %v", err)
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
	if syncWindow(messagesWin, &gs.MessagesWindow) {
		changed = true
	}
	if syncWindow(chatWin, &gs.ChatWindow) {
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
	sx, sy := eui.ScreenSize()
	pos := WindowPoint{X: float64(win.Position.X) / float64(sx), Y: float64(win.Position.Y) / float64(sy)}
	if state.Position != pos {
		state.Position = pos
		changed = true
	}
	size := WindowPoint{X: float64(win.Size.X) / float64(sx), Y: float64(win.Size.Y) / float64(sy)}
	if state.Size != size {
		state.Size = size
		changed = true
	}
	return changed
}

func clampWindowSettings() {
	states := []*WindowState{&gs.GameWindow, &gs.InventoryWindow, &gs.PlayersWindow, &gs.MessagesWindow, &gs.ChatWindow}
	for _, st := range states {
		clampWindowState(st)
	}
}

func clampWindowState(st *WindowState) {
	if st.Size.X <= 0 || st.Size.X > 1 || st.Size.Y <= 0 || st.Size.Y > 1 {
		st.Position = WindowPoint{}
		st.Size = WindowPoint{}
		return
	}
	if st.Position.X < 0 {
		st.Position.X = 0
	} else if st.Position.X > 1 {
		st.Position.X = 1
	}
	if st.Position.Y < 0 {
		st.Position.Y = 0
	} else if st.Position.Y > 1 {
		st.Position.Y = 1
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
	if gs.fastSound {
		resample = resampleLinear
	} else {
		initSinc()
		resample = resampleSincHQ
	}
	clearCaches()
	settingsDirty = true
	if qualityWin != nil {
		qualityWin.Refresh()
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
