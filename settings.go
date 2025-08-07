package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
)

type Settings struct {
	Scale          int     `json:"scale"`
	ClickToToggle  bool    `json:"clickToToggle"`
	Linear         bool    `json:"linear"`
	Vsync          bool    `json:"vsync"`
	Interp         bool    `json:"interp"`
	SmoothMoving   bool    `json:"smoothMoving"`
	Onion          bool    `json:"onion"`
	BlendPicts     bool    `json:"blendPicts"`
	BlendRate      float64 `json:"blendRate"`
	NightMode      bool    `json:"nightMode"`
	ShowBubbles    bool    `json:"showBubbles"`
	MainFontSize   float64 `json:"mainFontSize"`
	BubbleFontSize float64 `json:"bubbleFontSize"`
	ShowPlanes     bool    `json:"showPlanes"`
	HideMoving     bool    `json:"hideMoving"`
	HideMobiles    bool    `json:"hideMobiles"`
	KeyWalkSpeed   float64 `json:"keyWalkSpeed"`
	LastCharacter  string  `json:"lastCharacter"`
}

var settingsDirty bool

func loadSettings() bool {
	path := filepath.Join(baseDir, "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return false
	}
	scale = s.Scale
	clickToToggle = s.ClickToToggle
	linear = s.Linear
	vsync = s.Vsync
	interp = s.Interp
	smoothMoving = s.SmoothMoving
	mobileBlending = s.Onion
	blendPicts = s.BlendPicts
	blendRate = s.BlendRate
	nightMode = s.NightMode
	showBubbles = s.ShowBubbles
	mainFontSize = s.MainFontSize
	bubbleFontSize = s.BubbleFontSize
	showPlanes = s.ShowPlanes
	hideMoving = s.HideMoving
	hideMobiles = s.HideMobiles
	keyWalkSpeed = s.KeyWalkSpeed
	if keyWalkSpeed == 0 {
		keyWalkSpeed = 0.5
	}
	lastCharacter = s.LastCharacter
	return true
}

func applySettings() {
	if linear {
		drawFilter = ebiten.FilterLinear
	} else {
		drawFilter = ebiten.FilterNearest
	}
	ebiten.SetVsyncEnabled(vsync)
	ebiten.SetWindowSize(gameAreaSizeX*scale, gameAreaSizeY*scale)
	initFont()
	inputBg = nil
}

func saveSettings() {
	s := Settings{
		Scale:          scale,
		ClickToToggle:  clickToToggle,
		Linear:         linear,
		Vsync:          vsync,
		Interp:         interp,
		SmoothMoving:   smoothMoving,
		Onion:          mobileBlending,
		BlendPicts:     blendPicts,
		BlendRate:      blendRate,
		NightMode:      nightMode,
		ShowBubbles:    showBubbles,
		MainFontSize:   mainFontSize,
		BubbleFontSize: bubbleFontSize,
		ShowPlanes:     showPlanes,
		HideMoving:     hideMoving,
		HideMobiles:    hideMobiles,
		KeyWalkSpeed:   keyWalkSpeed,
		LastCharacter:  lastCharacter,
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		log.Printf("save settings: %v", err)
		return
	}
	path := filepath.Join(baseDir, "/settings.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("save settings: %v", err)
	}
}
