package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"time"

	"gothoom/climg"
	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
)

const SETTINGS_VERSION = 9

type BarPlacement int

const (
	BarPlacementBottom BarPlacement = iota
	BarPlacementLowerLeft
	BarPlacementLowerRight
	BarPlacementUpperRight
)

var gs settings = gsdef

// settingsLoaded reports whether settings were successfully loaded from disk.
var settingsLoaded bool

// windowsRestored tracks whether window positions have been restored for the
// current UI scale. Initialization defers restoration until the first layout
// provides a final screen size.
var windowsRestored bool

var gsdef settings = settings{
    Version: SETTINGS_VERSION,

	KBWalkSpeed:             0.25,
	MainFontSize:            8,
	BubbleFontSize:          6,
	ConsoleFontSize:         12,
	ChatFontSize:            14,
	InventoryFontSize:       18,
	PlayersFontSize:         18,
	BubbleOpacity:           0.7,
	BubbleBaseLife:          2.0,
	BubbleLifePerWord:       1.0,
	NameBgOpacity:           0.7,
    NameTagLabelColors:      true,
	BarOpacity:              0.5,
	ObscuringPictureOpacity: 0.5,
	FadeObscuringPictures:   true,
	SpeechBubbles:           true,
	BubbleNormal:            true,
	BubbleWhisper:           true,
	BubbleYell:              true,
	BubbleThought:           true,
	BubbleRealAction:        true,
	BubbleMonster:           true,
	BubblePlayerAction:      true,
	BubblePonder:            true,
	BubbleNarrate:           true,
	BubbleSelf:              true,
	BubbleOtherPlayers:      true,
	BubbleMonsters:          true,
	BubbleNarration:         true,

    MotionSmoothing:      true,
    DenoiseImages:        true,  // High preset default
    BlendMobiles:         false, // High preset default
    BlendPicts:           true,  // High preset default
    NoCaching:            false, // High preset default
    ObjectPinning:        true,
	BlendAmount:          1.0,
	MobileBlendAmount:    0.33,
	MobileBlendFrames:    10,
	PictBlendFrames:      10,
	DenoiseSharpness:     4.0,
	DenoiseAmount:        0.33,
	ShowFPS:              true,
	UIScale:              1.0,
	MasterVolume:         1.0,
	GameVolume:           0.6,
	MusicVolume:          0.8,
	Music:                true,
	GameSound:            true,
	GameScale:            2,
	BarPlacement:         BarPlacementBottom,
	ShaderLightStrength:  1.0,
	ShaderGlowStrength:   1.0,
	MaxNightLevel:        100,
	ForceNightLevel:      -1,
	ChatTTS:              true,
	ChatTTSVolume:        1.0,
	ChatTTSSpeed:         1.5,
	ChatTTSVoice:         "en_US-hfc_female-medium",
	ChatTTSBlocklist:     []string{"koppi", "crius"},
	Notifications:        true,
	NotifyFallen:         true,
	NotifyNotFallen:      true,
	NotifyShares:         true,
	NotifyFriendOnline:   true,
    NotifyCopyText:       true,
    NotificationDuration: 6,
    PluginSpamKill:       true,
    PromptOnSaveRecording: true,
	TimestampFormat:      "3:04PM",
	LastUpdateCheck:      time.Time{},
	NotifiedVersion:      0,

	GameWindow:      WindowState{Open: true},
	InventoryWindow: WindowState{Open: true},
	PlayersWindow:   WindowState{Open: true},
	MessagesWindow:  WindowState{Open: true},
	ChatWindow:      WindowState{Open: true},
	EnabledPlugins:  map[string]any{},
	vsync:           true,
	nightEffect:     true,
	shaderLighting:  true,
	throttleSounds:  true,
}

type settings struct {
	Version int

	LastCharacter           string
	ClickToToggle           bool
	MiddleClickMoveWindow   bool
	InputBarAlwaysOpen      bool
	KBWalkSpeed             float64
	MainFontSize            float64
	BubbleFontSize          float64
	ConsoleFontSize         float64
	ChatFontSize            float64
	InventoryFontSize       float64
	PlayersFontSize         float64
	BubbleOpacity           float64
	BubbleBaseLife          float64 `json:"BubbleLife"`
	BubbleLifePerWord       float64 `json:"BubblePerWord"`
	NameBgOpacity           float64
	NameTagLabelColors      bool
	BarOpacity              float64
	ObscuringPictureOpacity float64
	FadeObscuringPictures   bool
	SpeechBubbles           bool
	BubbleNormal            bool
	BubbleWhisper           bool
	BubbleYell              bool
	BubbleThought           bool
	BubbleRealAction        bool
	BubbleMonster           bool
	BubblePlayerAction      bool
	BubblePonder            bool
	BubbleNarrate           bool
	BubbleSelf              bool
	BubbleOtherPlayers      bool
	BubbleMonsters          bool
	BubbleNarration         bool

	MotionSmoothing      bool
	ObjectPinning        bool
	BlendMobiles         bool
	BlendPicts           bool
	BlendAmount          float64
	MobileBlendAmount    float64
	MobileBlendFrames    int
	PictBlendFrames      int
	DenoiseImages        bool
	DenoiseSharpness     float64
	DenoiseAmount        float64
	ShowFPS              bool
	UIScale              float64
	Fullscreen           bool
	AlwaysOnTop          bool
	MasterVolume         float64
	GameVolume           float64
	MusicVolume          float64
	Music                bool
	GameSound            bool
	Mute                 bool
	GameScale            float64
	BarPlacement         BarPlacement
	MaxNightLevel        int
	ForceNightLevel      int
	Theme                string
	MessagesToConsole    bool
	ChatTTS              bool
	ChatTTSVolume        float64
	ChatTTSSpeed         float64
	ChatTTSVoice         string
	ChatTTSBlocklist     []string
	Notifications        bool
	NotifyFallen         bool
	NotifyNotFallen      bool
	NotifyShares         bool
	NotifyFriendOnline   bool
    NotifyCopyText       bool
    NotificationDuration float64
    PluginSpamKill       bool
    PromptOnSaveRecording bool
	ChatTimestamps       bool
	ConsoleTimestamps    bool
	TimestampFormat      string
	LastUpdateCheck      time.Time
	NotifiedVersion      int
	WindowTiling         bool
	WindowSnapping       bool
	ShowPinToLocations   bool

	WindowWidth  int
	WindowHeight int

	GameWindow      WindowState
	InventoryWindow WindowState
	PlayersWindow   WindowState
	MessagesWindow  WindowState
	ChatWindow      WindowState
	WindowZones     map[string]eui.WindowZoneState

	ShaderLightStrength float64
	ShaderGlowStrength  float64

	imgPlanesDebug      bool
	smoothingDebug      bool
	pictAgainDebug      bool
	pictIDDebug         bool
	pluginOutputDebug   bool
	hideMoving          bool
	hideMobiles         bool
	EnabledPlugins      map[string]any
	vsync               bool
	nightEffect         bool
	shaderLighting      bool
	precacheSounds      bool
	precacheImages      bool
	throttleSounds      bool
	smoothMoving        bool
	dontShiftNewSprites bool
	nameTagsNative      bool
	BarColorByValue     bool
	recordAssetStats    bool
	NoCaching           bool
	PotatoComputer      bool
}

var (
	settingsDirty    bool
	lastSettingsSave = time.Now()
	bubbleTypeMask   uint32
	bubbleSourceMask uint32
)

const (
	bubbleSourceSelf = 1 << iota
	bubbleSourceOtherPlayers
	bubbleSourceMonsters
	bubbleSourceNarration
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
	defer syncTTSBlocklist()
	path := filepath.Join(dataDirPath, settingsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		gs = gsdef
		applyQualityPreset("High")
		settingsLoaded = false
		return false
	}

	type settingsFile struct {
		settings
		EnabledPlugins map[string]any `json:"EnabledPlugins"`
	}

	tmp := settingsFile{settings: gsdef}
	if err := json.Unmarshal(data, &tmp); err != nil {
		gs = gsdef
		settingsLoaded = false
		return false
	}

	if tmp.settings.Version == SETTINGS_VERSION {
		gs = tmp.settings
		// Normalize and retain whatever was in the file; migrate into runtime scope map.
		gs.EnabledPlugins = make(map[string]any)
		for k, v := range tmp.EnabledPlugins {
			gs.EnabledPlugins[k] = v
			s := scopeFromSettingValue(v)
			if !s.empty() {
				pluginMu.Lock()
				pluginEnabledFor[k] = s
				pluginMu.Unlock()
			}
		}
		settingsLoaded = true
	} else {
		gs = gsdef
		applyQualityPreset("High")
		settingsLoaded = false
		return false
	}

	if gs.EnabledPlugins == nil {
		gs.EnabledPlugins = make(map[string]any)
	}

	if gs.ChatTTSBlocklist == nil {
		gs.ChatTTSBlocklist = append([]string(nil), gsdef.ChatTTSBlocklist...)
	}

	if gs.DenoiseAmount < 0 || gs.DenoiseAmount > 1 {
		gs.DenoiseAmount = gsdef.DenoiseAmount
	}
	if gs.DenoiseSharpness < 0 || gs.DenoiseSharpness > 20 {
		gs.DenoiseSharpness = gsdef.DenoiseSharpness
	}

	if gs.ChatTTSSpeed <= 0 {
		gs.ChatTTSSpeed = gsdef.ChatTTSSpeed
	}
	if gs.ChatTTSVoice == "" {
		gs.ChatTTSVoice = gsdef.ChatTTSVoice
	}

	if gs.ShaderLightStrength < 0 || gs.ShaderLightStrength > 2 {
		gs.ShaderLightStrength = gsdef.ShaderLightStrength
	}
	if gs.ShaderGlowStrength < 0 || gs.ShaderGlowStrength > 2 {
		gs.ShaderGlowStrength = gsdef.ShaderGlowStrength
	}

	if gs.WindowWidth > 0 && gs.WindowHeight > 0 {
		eui.SetScreenSize(gs.WindowWidth, gs.WindowHeight)
	}

	clampWindowSettings()
	return settingsLoaded
}

func applySettings() {
	updateBubbleVisibility()
	eui.SetWindowTiling(gs.WindowTiling)
	eui.SetWindowSnapping(gs.WindowSnapping)
	eui.SetShowPinLocations(gs.ShowPinToLocations)
	eui.SetMiddleClickMove(gs.MiddleClickMoveWindow)
	eui.SetPotatoMode(gs.PotatoComputer)
	climg.SetPotatoMode(gs.PotatoComputer)
	if clImages != nil {
		clImages.Denoise = gs.DenoiseImages
		clImages.DenoiseSharpness = gs.DenoiseSharpness
		clImages.DenoiseAmount = gs.DenoiseAmount
	}
	ebiten.SetVsyncEnabled(gs.vsync)
	ebiten.SetFullscreen(gs.Fullscreen)
	ebiten.SetWindowFloating(gs.Fullscreen || gs.AlwaysOnTop)
	initFont()
	updateSoundVolume()
	if gs.InputBarAlwaysOpen {
		inputActive = true
	}
}

func updateBubbleVisibility() {
	bubbleTypeMask = 0
	if gs.BubbleNormal {
		bubbleTypeMask |= 1 << kBubbleNormal
	}
	if gs.BubbleWhisper {
		bubbleTypeMask |= 1 << kBubbleWhisper
	}
	if gs.BubbleYell {
		bubbleTypeMask |= 1 << kBubbleYell
	}
	if gs.BubbleThought {
		bubbleTypeMask |= 1 << kBubbleThought
	}
	if gs.BubbleRealAction {
		bubbleTypeMask |= 1 << kBubbleRealAction
	}
	if gs.BubbleMonster {
		bubbleTypeMask |= 1 << kBubbleMonster
	}
	if gs.BubblePlayerAction {
		bubbleTypeMask |= 1 << kBubblePlayerAction
	}
	if gs.BubblePonder {
		bubbleTypeMask |= 1 << kBubblePonder
	}
	if gs.BubbleNarrate {
		bubbleTypeMask |= 1 << kBubbleNarrate
	}

	bubbleSourceMask = 0
	if gs.BubbleSelf {
		bubbleSourceMask |= bubbleSourceSelf
	}
	if gs.BubbleOtherPlayers {
		bubbleSourceMask |= bubbleSourceOtherPlayers
	}
	if gs.BubbleMonsters {
		bubbleSourceMask |= bubbleSourceMonsters
	}
	if gs.BubbleNarration {
		bubbleSourceMask |= bubbleSourceNarration
	}
}

func saveSettings() {
	pluginMu.RLock()
	// Rebuild the persisted map from the current scope set.
	gs.EnabledPlugins = make(map[string]any, len(pluginEnabledFor))
	for k, s := range pluginEnabledFor {
		if s.All {
			gs.EnabledPlugins[k] = "all"
			continue
		}
		if len(s.Chars) > 0 {
			// Collect and sort for stable output
			names := make([]string, 0, len(s.Chars))
			for n := range s.Chars {
				names = append(names, n)
			}
			sort.Strings(names)
			gs.EnabledPlugins[k] = names
		}
	}
	pluginMu.RUnlock()

	data, err := json.MarshalIndent(gs, "", "  ")
	if err != nil {
		logError("save settings: %v", err)
		return
	}
	path := filepath.Join(dataDirPath, settingsFile)
	if err := os.WriteFile(path+".tmp", data, 0644); err != nil {
		logError("save settings: %v", err)
	}

	os.Rename(path+".tmp", path)
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
	zones := eui.SaveWindowZones()
	if !reflect.DeepEqual(zones, gs.WindowZones) {
		gs.WindowZones = zones
		changed = true
	}
	w, h := ebiten.WindowSize()
	if w > 0 && h > 0 {
		if gs.WindowWidth != w || gs.WindowHeight != h {
			gs.WindowWidth = w
			gs.WindowHeight = h
			changed = true
		}
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

func applyWindowState(win *eui.WindowData, st *WindowState) {
	if win == nil || st == nil {
		return
	}
	if st.Size.X >= eui.MinWindowSize && st.Size.Y >= eui.MinWindowSize {
		_ = win.SetSize(eui.Point{X: float32(st.Size.X), Y: float32(st.Size.Y)})
	}
	if st.Position.X != 0 || st.Position.Y != 0 {
		_ = win.SetPos(eui.Point{X: float32(st.Position.X), Y: float32(st.Position.Y)})
	}
	if st.Open {
		win.MarkOpen()
	}
}

func restoreWindowSettings() {
	eui.LoadWindowZones(gs.WindowZones)
	applyWindowState(gameWin, &gs.GameWindow)
	if gameWin != nil {
		gameWin.MarkOpen()
	}
	applyWindowState(inventoryWin, &gs.InventoryWindow)
	applyWindowState(playersWin, &gs.PlayersWindow)
	applyWindowState(consoleWin, &gs.MessagesWindow)
	applyWindowState(chatWin, &gs.ChatWindow)
	if hudWin != nil {
		hudWin.MarkOpen()
	}
	windowsRestored = true
}

// restoreWindowsAfterScale ensures window geometry is applied only after the UI
// scale and HiDPI settings have been established. It restores saved window
// positions a single time per scale change.
func restoreWindowsAfterScale() {
	if windowsRestored {
		return
	}
	restoreWindowSettings()
}

type qualityPreset struct {
	DenoiseImages   bool
	MotionSmoothing bool
	BlendMobiles    bool
	BlendPicts      bool
	NoCaching       bool
	ShaderLighting  bool
}

var (
	ultraLowPreset = qualityPreset{
		DenoiseImages:   false,
		MotionSmoothing: false,
		BlendMobiles:    false,
		BlendPicts:      false,
		NoCaching:       true,
		ShaderLighting:  false,
	}
	lowPreset = qualityPreset{
		DenoiseImages:   false,
		MotionSmoothing: false,
		BlendMobiles:    false,
		BlendPicts:      false,
		NoCaching:       false,
		ShaderLighting:  false,
	}
	standardPreset = qualityPreset{
		DenoiseImages:   true,
		MotionSmoothing: true,
		BlendMobiles:    false,
		BlendPicts:      false,
		NoCaching:       false,
		ShaderLighting:  true,
	}
	highPreset = qualityPreset{
		DenoiseImages:   true,
		MotionSmoothing: true,
		BlendMobiles:    false,
		BlendPicts:      true,
		NoCaching:       false,
		ShaderLighting:  true,
	}
	ultimatePreset = qualityPreset{
		DenoiseImages:   true,
		MotionSmoothing: true,
		BlendMobiles:    true,
		BlendPicts:      true,
		NoCaching:       false,
		ShaderLighting:  true,
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
	gs.shaderLighting = p.ShaderLighting
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
	if shaderLightSlider != nil {
		shaderLightSlider.Disabled = !gs.shaderLighting
	}
	if shaderGlowSlider != nil {
		shaderGlowSlider.Disabled = !gs.shaderLighting
	}
}

func matchesPreset(p qualityPreset) bool {
	return gs.DenoiseImages == p.DenoiseImages &&
		gs.MotionSmoothing == p.MotionSmoothing &&
		gs.BlendMobiles == p.BlendMobiles &&
		gs.BlendPicts == p.BlendPicts &&
		gs.NoCaching == p.NoCaching &&
		gs.shaderLighting == p.ShaderLighting
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
