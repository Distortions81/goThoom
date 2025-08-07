package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
)

const settingFilePath = "data/settings.json"

var gs settings = settings{
	Version: 1,

	KBWalkSpeed:    0.25,
	MainFontSize:   9,
	BubbleFontSize: 9,

	NightEffect:     true,
	SpeechBubbles:   true,
	FastBars:        true,
	MotionSmoothing: true,
	SmoothMoving:    true,
	BlendMobiles:    false,
	BlendPicts:      true,
	BlendAmount:     1.0,
	Scale:           2,

	vsync: true,
}

type settings struct {
	Version int

	LastCharacter  string
	ClickToToggle  bool
	KBWalkSpeed    float64
	MainFontSize   float64
	BubbleFontSize float64

	NightEffect      bool
	SpeechBubbles    bool
	FastBars         bool
	MotionSmoothing  bool
	SmoothMoving     bool
	BlendMobiles     bool
	BlendPicts       bool
	BlendAmount      float64
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
	//Remove older settings
	os.Remove("settings.json")

	path := filepath.Join(baseDir, settingFilePath)
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var s settings
	if err := json.Unmarshal(data, &s); err != nil {
		return false
	}
	return true
}

func applySettings() {
	if gs.TextureFiltering {
		drawFilter = ebiten.FilterLinear
	} else {
		drawFilter = ebiten.FilterNearest
	}
	ebiten.SetVsyncEnabled(gs.vsync)
	initFont()
}

func saveSettings() {
	data, err := json.MarshalIndent(gs, "", "  ")
	if err != nil {
		log.Printf("save settings: %v", err)
		return
	}
	path := filepath.Join(baseDir, "data/settings.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("save settings: %v", err)
	}
}
