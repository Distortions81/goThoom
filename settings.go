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
	BlendPicts:        true,
	BlendAmount:       1.0,
	MobileBlendAmount: 0.33,
	MobileBlendFrames: 10,
	PictBlendFrames:   10,
	DenoiseImages:     true,
	DenoiseSharpness:  4.0,
	DenoisePercent:    0.2,
	ShowFPS:           true,
	UIScale:           1.0,
	Fullscreen:        false,

	imgPlanesDebug:    false,
	smoothingDebug:    false,
	hideMoving:        false,
	hideMobiles:       false,
	vsync:             true,
	fastSound:         true,
	nightEffect:       true,
	precacheSounds:    false,
	precacheImages:    false,
	textureFiltering:  false,
	lateInputUpdates:  true,
	cacheWholeSheet:   true,
	smoothMoving:      true,
	fastBars:          true,
	bubbleMessages:    false,
	Volume:            0.5,
	Mute:              false,
	recordAssetStats:  true,
	scale:             2,
	AnyGameWindowSize: false,

	GameWindow:      WindowState{Open: true},
	InventoryWindow: WindowState{Open: true},
	PlayersWindow:   WindowState{Open: true},
	MessagesWindow:  WindowState{Open: true},
	ChatWindow:      WindowState{Open: true},
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
	bubbleMessages    bool
	Volume            float64
	Mute              bool
	recordAssetStats  bool
	scale             float64
	AnyGameWindowSize bool

	GameWindow      WindowState
	InventoryWindow WindowState
	PlayersWindow   WindowState
	MessagesWindow  WindowState
	ChatWindow      WindowState
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
