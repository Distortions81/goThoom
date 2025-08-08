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
	BlendMobiles:      true,
	BlendPicts:        true,
	BlendAmount:       1.0,
	MobileBlendAmount: 0.33,
	DenoiseImages:     true,
	DenoiseSharpness:  4.0,
	DenoisePercent:    0.2,
	ShowFPS:           true,
	Scale:             2,

	vsync:            true,
	nightEffect:      true,
	lateInputUpdates: true,
	cacheWholeSheet:  true,
	smoothMoving:     true,
	fastBars:         true,
	speechBubbles:    true,
	bubbleMessages:   false,
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

	Scale int

	imgPlanesDebug   bool
	smoothingDebug   bool
	hideMoving       bool
	hideMobiles      bool
	vsync            bool
	fastSound        bool
	nightEffect      bool
	precacheAssets   bool
	textureFiltering bool
	lateInputUpdates bool
	cacheWholeSheet  bool
	smoothMoving     bool
	fastBars         bool
	speechBubbles    bool
	bubbleMessages   bool
}

var settingsDirty bool

func loadSettings() bool {
	path := filepath.Join(baseDir, "data", "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	if err := json.Unmarshal(data, &gs); err != nil {
		return false
	}

	initFont()
	resizeUI()
	ebiten.SetWindowSize(gameAreaSizeX*gs.Scale, gameAreaSizeY*gs.Scale)
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
	resizeUI()
	ebiten.SetWindowSize(gameAreaSizeX*gs.Scale, gameAreaSizeY*gs.Scale)
}

func saveSettings() {
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

func resizeUI() {
	var val float32 = 1.0
	if gs.Scale == 1 {
		val = 0.5
	}

	eui.SetUIScale(val)
}
