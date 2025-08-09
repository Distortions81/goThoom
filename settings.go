package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/Distortions81/EUI/eui"
	"github.com/hajimehoshi/ebiten/v2"
)

var gs settings = settings{
	Version: 1,

	KBWalkSpeed:    0.25,
	MainFontSize:   8,
	BubbleFontSize: 6,
	BubbleOpacity:  0.7,
	NameBgOpacity:  0.7,

	MotionSmoothing:   true,
	BlendMobiles:      false,
	BlendPicts:        true,
	BlendAmount:       1.0,
	MobileBlendAmount: 0.33,
	DenoiseImages:     true,
	DenoiseSharpness:  4.0,
	DenoisePercent:    0.2,
	ShowFPS:           true,
	UIScale:           1.0,

	vsync:            true,
	nightEffect:      true,
	lateInputUpdates: true,

	fastSound:       false,
	precacheSounds:  true,
	precacheImages:  false,
	cacheWholeSheet: false,
	smoothMoving:    true,
	fastBars:        true,
	speechBubbles:   true,
	bubbleMessages:  false,

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

	MotionSmoothing   bool
	BlendMobiles      bool
	BlendPicts        bool
	BlendAmount       float64
	MobileBlendAmount float64
	DenoiseImages     bool
	DenoiseSharpness  float64
	DenoisePercent    float64
	ShowFPS           bool

	Scale   float64
	UIScale float64

	imgPlanesDebug   bool
	smoothingDebug   bool
	hideMoving       bool
	hideMobiles      bool
	vsync            bool
	fastSound        bool
	nightEffect      bool
	precacheSounds   bool
	precacheImages   bool
	textureFiltering bool
	lateInputUpdates bool
	cacheWholeSheet  bool
	smoothMoving     bool
	fastBars         bool
	speechBubbles    bool
	bubbleMessages   bool
	recordAssetStats bool

	GameWindow      WindowState
	InventoryWindow WindowState
	PlayersWindow   WindowState
	MessagesWindow  WindowState
	ChatWindow      WindowState
}

var settingsDirty bool

type WindowPoint struct {
	X float64
	Y float64
}

type WindowState struct {
	Open     bool
	Position WindowPoint
	Size     WindowPoint
}

func loadSettings() bool {
	path := filepath.Join(baseDir, "data", "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	if err := json.Unmarshal(data, &gs); err != nil {
		return false
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
	ebiten.SetVsyncEnabled(gs.vsync)
	initFont()
}

func saveSettings() {
	syncWindowSettings()
	data, err := json.MarshalIndent(gs, "", "  ")
	if err != nil {
		log.Printf("save settings: %v", err)
		return
	}
	path := filepath.Join(baseDir, "data", "settings.json")
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
	if state.Open != win.Open {
		state.Open = win.Open
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
