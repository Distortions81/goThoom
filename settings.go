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
	BubbleOpacity:  160.0 / 255.0,
	NameBgOpacity:  1.0,

	NightEffect:      true,
	SpeechBubbles:    true,
	FastBars:         true,
	MotionSmoothing:  true,
	SmoothMoving:     true,
	BlendMobiles:     false,
	BlendPicts:       true,
	BlendAmount:      1.0,
	DenoiseImages:    false,
	DenoiseThreshold: 3000,
	DenoisePercent:   0.5,
	PrecacheAssets:   false,
	CacheWholeSheet:  true,
	ShowFPS:          true,
	LateInputUpdates: true,
	Scale:            2,

	vsync: true,
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

	NightEffect      bool
	SpeechBubbles    bool
	FastBars         bool
	MotionSmoothing  bool
	SmoothMoving     bool
	BlendMobiles     bool
	BlendPicts       bool
	BlendAmount      float64
	DenoiseImages    bool
	DenoiseThreshold int
	DenoisePercent   float64
	PrecacheAssets   bool
	CacheWholeSheet  bool
	ShowFPS          bool
	LateInputUpdates bool
	TextureFiltering bool
	FastSound        bool
	Scale            int

	imgPlanesDebug bool
	smoothingDebug bool
	hideMoving     bool
	hideMobiles    bool
	vsync          bool
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
		clImages.DenoiseThreshold = gs.DenoiseThreshold
		clImages.DenoisePercent = gs.DenoisePercent
	}
	if gs.TextureFiltering {
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
