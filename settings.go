package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"gothoom/climg"
	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
)

const SETTINGS_VERSION = 3

type BarPlacement int

const (
	BarPlacementBottom BarPlacement = iota
	BarPlacementLowerLeft
	BarPlacementLowerRight
	BarPlacementUpperRight
)

var gs settings = gsdef

var gammaOptions = []float64{1.8, 2.0, 2.2, 2.4}

// settingsLoaded reports whether settings were successfully loaded from disk.
var settingsLoaded bool

// windowsRestored tracks whether window positions have been restored for the
// current UI scale. Initialization defers restoration until the first layout
// provides a final screen size.
var windowsRestored bool

func spriteUpscaleFactorFromScale(scale float64) int {
	factor := int(math.Round(scale))
	if factor < 1 {
		factor = 1
	}
	if factor > 4 {
		factor = 4
	}
	return factor
}

func spriteUpscaleFactor() int {
	return spriteUpscaleFactorFromScale(gs.GameScale)
}

func clampSoundEnhancementAmount(v float64) float64 {
	if v < 0.1 {
		return 0.1
	}
	if v > 10 {
		return 10
	}
	return v
}

func normalizeGamma(v, fallback float64) float64 {
	if math.IsNaN(v) || v <= 0 {
		return fallback
	}
	best := fallback
	bestDist := math.Abs(v - best)
	for _, opt := range gammaOptions {
		d := math.Abs(v - opt)
		if d < bestDist {
			best = opt
			bestDist = d
		}
	}
	return best
}

var gsdef settings = settings{
	Version:            SETTINGS_VERSION,
	SetupWizardVersion: 0,

	LastCharacter:           "",
	ClickToToggle:           false,
	MiddleClickMoveWindow:   false,
	InputBarAlwaysOpen:      false,
	KBWalkSpeed:             0.25,
	MainFontSize:            8,
	BubbleFontSize:          15,
	ConsoleFontSize:         12,
	ChatFontSize:            14,
	InventoryFontSize:       18,
	PlayersFontSize:         18,
	AlternateRowBackgrounds: true,
	BubbleOpacity:           0.8,
	BubbleBaseLife:          2,
	BubbleLifePerWord:       1,
	BubbleScale:             2.0,
	NameBgOpacity:           0.8,
	DarkBubblesAndNames:     true,
	NameTagLabelColors:      true,
	HideSelfNameTag:         true,
	NameTagsOnHoverOnly:     false,
	BarOpacity:              0.66,
	ObscuringPictureOpacity: 0.66,
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

	MotionSmoothing:       true,
	ObjectPinning:         true,
	BlendMobiles:          true,
	BlendPicts:            true,
	BlendAmount:           1.0,
	MobileBlendAmount:     0.25,
	MobileBlendFrames:     10,
	PictBlendFrames:       10,
	DenoiseImages:         true,
	DenoiseSharpness:      4,
	DenoiseAmount:         0.33,
	ShowFPS:               true,
	UIScale:               1.0,
	Fullscreen:            false,
	AlwaysOnTop:           false,
	MasterVolume:          1.0,
	GameVolume:            0.6,
	MusicVolume:           1.0,
	Music:                 true,
	GameSound:             true,
	Mute:                  false,
	GameScale:             2.0,
	SpriteUpscale:         2,
	SpriteUpscaleFilter:   false,
	SpriteGammaCorrection: true,
	SpriteGamma:           1.8,
	MonitorGamma:          2.2,
	BarPlacement:          BarPlacementBottom,
	MaxNightLevel:         100,
	MessagesToConsole:     false,
	ChatTTS:               false,
	ChatTTSVolume:         0.33,
	ChatTTSSpeed:          1.25,
	ChatTTSVoice:          "en_US-hfc_female-medium",
	Notifications:         false,
	NotifyWhenBackground:  false,
	// Power saving defaults: limit FPS in background
	PowerSaveBackground:   false,
	PowerSaveAlways:       false,
	PowerSaveFPS:          15,
	MuteWhenUnfocused:     false,
	NotifyFallen:          true,
	NotifyNotFallen:       true,
	NotifyShares:          true,
	NotifyFriendOnline:    true,
	NotifyCopyText:        true,
	NotificationVolume:    0.6,
	NotificationBeep:      false,
	NotificationDuration:  6,
	ScriptSpamKill:        true,
	PromptOnSaveRecording: true,
	AutoRecord:            false,
	PromptDisableShaders:  true,
	ChatTimestamps:        false,
	ConsoleTimestamps:     false,
	TimestampFormat:       "3:04PM",
	LastUpdateCheck:       time.Time{},
	NotifiedVersion:       0,
	WindowTiling:          false,
	WindowSnapping:        false,
	WindowPinning:         false,
	ShowPinToLocations:    false,

	JoystickEnabled:        false,
	JoystickWalkStick:      0,
	JoystickCursorStick:    1,
	JoystickWalkDeadzone:   0.1,
	JoystickCursorDeadzone: 0.1,

	GameWindow:      WindowState{Open: true},
	InventoryWindow: WindowState{Open: true},
	PlayersWindow:   WindowState{Open: true},
	MessagesWindow:  WindowState{Open: true},
	ChatWindow:      WindowState{Open: true},
	WindowZones:     *new(map[string]eui.WindowZoneState),

	ShaderLightStrength: 1.0,
	ShaderGlowStrength:  1.0,

	PotatoGPU:              false,
	BarColorByValue:        false,
	ThrottleSounds:         true,
	SoundEnhancement:       false,
	SoundEnhancementAmount: 1.5,
	MusicEnhancement:       true,
	HighQualityResampling:  false,
	ServerAddress:          defaultServerHostName + ":5010",

	NightEffect:    true,
	ShaderLighting: true,

	// Window behavior
	ShowClanLordSplashImage: true,

	// Advanced and runtime defaults.
	VSync:             true,
	PrecacheSounds:    false,
	PrecacheImages:    false,
	smoothMoving:      false,
	recordAssetStats:  false,
	AltNetMode:        true,
	AltNetDelay:       100,
	hideMobiles:       false,
	imgPlanesDebug:    false,
	smoothingDebug:    false,
	pictAgainDebug:    false,
	pictIDDebug:       false,
	scriptOutputDebug: false,
	scriptEventDebug:  false,
	forceNightLevel:   -1,
}

type settings struct {
	Version            int
	SetupWizardVersion int

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
	AlternateRowBackgrounds bool
	BubbleOpacity           float64
	BubbleBaseLife          float64
	BubbleLifePerWord       float64
	// BubbleScale scales bubble visuals (not font). Range 1.0–8.0.
	BubbleScale         float64
	NameBgOpacity       float64
	DarkBubblesAndNames bool
	NameTagLabelColors  bool
	HideSelfNameTag     bool
	// NameTagsOnHoverOnly hides name tags unless the cursor is over a mobile.
	NameTagsOnHoverOnly     bool
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

	MotionSmoothing       bool
	PixelArtScaling       bool
	ObjectPinning         bool
	BlendMobiles          bool
	BlendPicts            bool
	BlendAmount           float64
	MobileBlendAmount     float64
	MobileBlendFrames     int
	PictBlendFrames       int
	DenoiseImages         bool
	DenoiseSharpness      float64
	DenoiseAmount         float64
	ShowFPS               bool
	UIScale               float64
	Fullscreen            bool
	AlwaysOnTop           bool
	VSync                 bool
	MasterVolume          float64
	GameVolume            float64
	MusicVolume           float64
	Music                 bool
	GameSound             bool
	Mute                  bool
	GameScale             float64
	SpriteUpscale         int
	SpriteUpscaleFilter   bool
	SpriteGammaCorrection bool
	SpriteGamma           float64
	MonitorGamma          float64
	BarPlacement          BarPlacement
	MaxNightLevel         int
	forceNightLevel       int
	Theme                 string
	Style                 string
	MessagesToConsole     bool
	MessageTextColors     map[string]eui.Color
	ChatTTS               bool
	ChatTTSVolume         float64
	ChatTTSSpeed          float64
	ChatTTSVoice          string
	ChatTTSBlocklist      []string
	Notifications         bool
	NotifyWhenBackground  bool
	// PowerSaveBackground reduces FPS when window is unfocused.
	PowerSaveBackground bool
	// PowerSaveAlways reduces FPS even when focused (e.g., laptops).
	PowerSaveAlways bool
	// PowerSaveFPS is the target FPS when power saving is active (1-45).
	PowerSaveFPS          int
	MuteWhenUnfocused     bool
	NotifyFallen          bool
	NotifyNotFallen       bool
	NotifyShares          bool
	NotifyFriendOnline    bool
	NotifyCopyText        bool
	NotificationVolume    float64
	NotificationBeep      bool
	NotificationDuration  float64
	ScriptSpamKill        bool
	PromptOnSaveRecording bool
	AutoRecord            bool
	PromptDisableShaders  bool
	ChatTimestamps        bool
	ConsoleTimestamps     bool
	TimestampFormat       string
	LastUpdateCheck       time.Time
	NotifiedVersion       int
	WindowTiling          bool
	WindowSnapping        bool
	WindowPinning         bool
	ShowPinToLocations    bool

	JoystickEnabled        bool
	JoystickBindings       map[string]ebiten.GamepadButton
	JoystickWalkStick      int
	JoystickCursorStick    int
	JoystickWalkDeadzone   float64
	JoystickCursorDeadzone float64

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

	PotatoGPU              bool
	PrecacheSounds         bool
	PrecacheImages         bool
	Enabledscripts         map[string]any
	BarColorByValue        bool
	ThrottleSounds         bool
	SoundEnhancement       bool
	SoundEnhancementAmount float64
	MusicEnhancement       bool
	HighQualityResampling  bool

	imgPlanesDebug    bool
	smoothingDebug    bool
	pictAgainDebug    bool
	pictIDDebug       bool
	scriptOutputDebug bool
	scriptEventDebug  bool
	AltNetMode        bool
	AltNetDelay       int
	ServerAddress     string
	hideMoving        bool
	hideMobiles       bool
	NightEffect       bool
	ShaderLighting    bool

	// Window behavior
	ShowClanLordSplashImage bool
	smoothMoving            bool
	recordAssetStats        bool
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
	defer syncTTSBlocklist()
	path := filepath.Join(dataDirPath, settingsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		gs = gsdef
		preset := "Classic"
		applyQualityPreset(preset)
		setHighQualityResamplingEnabled(gs.HighQualityResampling)
		settingsLoaded = false
		applyServerAddressSetting()
		return false
	}

	type settingsFile struct {
		settings
		Enabledscripts    map[string]any `json:"Enabledscripts"`
		LegacySoundReverb *bool          `json:"SoundReverb"`
		LegacyMusicReverb *bool          `json:"MusicReverb"`
	}

	tmp := settingsFile{settings: gsdef}
	if err := json.Unmarshal(data, &tmp); err != nil {
		gs = gsdef
		setHighQualityResamplingEnabled(gs.HighQualityResampling)
		settingsLoaded = false
		return false
	}

	if tmp.settings.Version == SETTINGS_VERSION {
		if tmp.LegacySoundReverb != nil {
			tmp.settings.SoundEnhancement = *tmp.LegacySoundReverb
		}
		if tmp.LegacyMusicReverb != nil {
			tmp.settings.MusicEnhancement = *tmp.LegacyMusicReverb
		}
		gs = tmp.settings
		setHighQualityResamplingEnabled(gs.HighQualityResampling)
		// Normalize and retain whatever was in the file; migrate into runtime scope map.
		gs.Enabledscripts = make(map[string]any)
		for k, v := range tmp.Enabledscripts {
			gs.Enabledscripts[k] = v
			s := scopeFromSettingValue(v)
			if !s.empty() {
				scriptMu.Lock()
				scriptEnabledFor[k] = s
				scriptMu.Unlock()
			}
		}
		settingsLoaded = true
	} else {
		gs = gsdef
		preset := "High"
		if isWASM {
			preset = "Standard"
		}
		applyQualityPreset(preset)
		setHighQualityResamplingEnabled(gs.HighQualityResampling)
		settingsLoaded = false
		applyServerAddressSetting()
		return false
	}

	if gs.Enabledscripts == nil {
		gs.Enabledscripts = make(map[string]any)
	}

	if gs.ChatTTSBlocklist == nil {
		gs.ChatTTSBlocklist = append([]string(nil), gsdef.ChatTTSBlocklist...)
	}
	ensureMessageTextColors()

	if gs.JoystickBindings == nil {
		gs.JoystickBindings = make(map[string]ebiten.GamepadButton)
	}

	if gs.JoystickWalkDeadzone < 0.01 || gs.JoystickWalkDeadzone > 0.2 {
		gs.JoystickWalkDeadzone = gsdef.JoystickWalkDeadzone
	}
	if gs.JoystickCursorDeadzone < 0.01 || gs.JoystickCursorDeadzone > 0.2 {
		gs.JoystickCursorDeadzone = gsdef.JoystickCursorDeadzone
	}

	if gs.DenoiseAmount < 0 || gs.DenoiseAmount > 1 {
		gs.DenoiseAmount = gsdef.DenoiseAmount
	}
	if gs.DenoiseSharpness < 0 || gs.DenoiseSharpness > 20 {
		gs.DenoiseSharpness = gsdef.DenoiseSharpness
	}
	if gs.AltNetDelay < 0 || gs.AltNetDelay > 190 {
		gs.AltNetDelay = gsdef.AltNetDelay
	}

	gs.SpriteGamma = normalizeGamma(gs.SpriteGamma, gsdef.SpriteGamma)
	gs.MonitorGamma = normalizeGamma(gs.MonitorGamma, gsdef.MonitorGamma)

	if gs.ChatTTSSpeed <= 0 {
		gs.ChatTTSSpeed = gsdef.ChatTTSSpeed
	}
	if gs.ChatTTSVoice == "" {
		gs.ChatTTSVoice = gsdef.ChatTTSVoice
	}

	applyServerAddressSetting()

	if gs.ShaderLightStrength < 0 || gs.ShaderLightStrength > 2 {
		gs.ShaderLightStrength = gsdef.ShaderLightStrength
	}
	if gs.ShaderGlowStrength < 0 || gs.ShaderGlowStrength > 2 {
		gs.ShaderGlowStrength = gsdef.ShaderGlowStrength
	}

	gs.SoundEnhancementAmount = clampSoundEnhancementAmount(gs.SoundEnhancementAmount)

	// Clamp BubbleScale to 1.0–8.0
	if gs.BubbleScale < 1.0 || gs.BubbleScale > 8.0 {
		gs.BubbleScale = gsdef.BubbleScale
	}

	gs.SpriteUpscale = spriteUpscaleFactor()

	if gs.WindowWidth > 0 && gs.WindowHeight > 0 {
		eui.SetScreenSize(gs.WindowWidth, gs.WindowHeight)
	}

	clampWindowSettings()
	// Clamp power-save FPS and set sane defaults when out-of-range or zero
	if gs.PowerSaveFPS < 1 {
		gs.PowerSaveFPS = 1
	}
	if gs.PowerSaveFPS > 45 {
		gs.PowerSaveFPS = 45
	}
	return settingsLoaded
}

func applyServerAddressSetting() {
	addr := strings.TrimSpace(gs.ServerAddress)
	if addr == "" {
		addr = gsdef.ServerAddress
	}
	gs.ServerAddress = addr
	host = addr
}

func applySettings() {
	eui.SetWindowTiling(gs.WindowTiling)
	eui.SetWindowSnapping(gs.WindowSnapping)
	eui.SetWindowPinning(gs.WindowPinning)
	eui.SetShowPinLocations(gs.ShowPinToLocations)
	eui.SetMiddleClickMove(gs.MiddleClickMoveWindow)
	eui.SetPotatoMode(gs.PotatoGPU)
	climg.SetPotatoMode(gs.PotatoGPU)
	if clImages != nil {
		clImages.Denoise = gs.DenoiseImages
		clImages.DenoiseSharpness = gs.DenoiseSharpness
		clImages.DenoiseAmount = gs.DenoiseAmount
		clImages.SetGammaCorrection(gs.SpriteGammaCorrection, gs.SpriteGamma, gs.MonitorGamma)
	}
	ebiten.SetVsyncEnabled(gs.VSync)
	ebiten.SetFullscreen(gs.Fullscreen)
	ebiten.SetWindowFloating(gs.Fullscreen || gs.AlwaysOnTop)
	initFont()
	updateSoundVolume()
	if gs.InputBarAlwaysOpen {
		inputActive = true
	}
}

func saveSettings() {
	if isWASM {
		// Skip disk writes in WASM; silently ignore.
		return
	}
	scriptMu.RLock()
	// Rebuild the persisted map from the current scope set.
	gs.Enabledscripts = make(map[string]any, len(scriptEnabledFor))
	for k, s := range scriptEnabledFor {
		if s.All {
			gs.Enabledscripts[k] = "all"
			continue
		}
		if len(s.Chars) > 0 {
			// Collect and sort for stable output
			names := make([]string, 0, len(s.Chars))
			for n := range s.Chars {
				names = append(names, n)
			}
			sort.Strings(names)
			gs.Enabledscripts[k] = names
		}
	}
	scriptMu.RUnlock()

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

// resetSavedWindowSettings restores only persisted window layout state.
// Other client preferences intentionally remain unchanged.
func resetSavedWindowSettings() {
	gs.GameWindow = gsdef.GameWindow
	gs.InventoryWindow = gsdef.InventoryWindow
	gs.PlayersWindow = gsdef.PlayersWindow
	gs.MessagesWindow = gsdef.MessagesWindow
	gs.ChatWindow = gsdef.ChatWindow
	gs.WindowZones = make(map[string]eui.WindowZoneState)
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
	DenoiseImages          bool
	MotionSmoothing        bool
	BlendMobiles           bool
	BlendPicts             bool
	NoCaching              bool
	ShaderLighting         bool
	SpriteUpscaleFilter    bool
	HighQualityResampling  bool
	SoundEnhancement       bool
	SoundEnhancementAmount float64
	MusicEnhancement       bool
}

var (
	classicPreset = qualityPreset{
		DenoiseImages:          false,
		MotionSmoothing:        false,
		BlendMobiles:           false,
		BlendPicts:             false,
		ShaderLighting:         false,
		SpriteUpscaleFilter:    false,
		HighQualityResampling:  false,
		SoundEnhancement:       false,
		SoundEnhancementAmount: 1.0,
		MusicEnhancement:       false,
	}
	lowPreset = qualityPreset{
		DenoiseImages:          false,
		MotionSmoothing:        true,
		BlendMobiles:           false,
		BlendPicts:             false,
		ShaderLighting:         false,
		SpriteUpscaleFilter:    false,
		HighQualityResampling:  false,
		SoundEnhancement:       false,
		SoundEnhancementAmount: 1.0,
		MusicEnhancement:       false,
	}
	mediumPreset = qualityPreset{
		DenoiseImages:          true,
		MotionSmoothing:        true,
		BlendMobiles:           false,
		BlendPicts:             true,
		ShaderLighting:         false,
		SpriteUpscaleFilter:    false,
		HighQualityResampling:  false,
		SoundEnhancement:       false,
		SoundEnhancementAmount: 1.0,
		MusicEnhancement:       true,
	}
	highPreset = qualityPreset{
		DenoiseImages:          true,
		MotionSmoothing:        true,
		BlendMobiles:           true,
		BlendPicts:             true,
		ShaderLighting:         true,
		SpriteUpscaleFilter:    true,
		HighQualityResampling:  true,
		SoundEnhancement:       true,
		SoundEnhancementAmount: 1.25,
		MusicEnhancement:       true,
	}
)

func applyQualityPreset(name string) {
	var p qualityPreset
	switch name {
	case "Classic":
		p = classicPreset
	case "Low":
		p = lowPreset
	case "Medium":
		p = mediumPreset
	case "High":
		p = highPreset
	default:
		return
	}

	gs.DenoiseImages = p.DenoiseImages
	gs.MotionSmoothing = p.MotionSmoothing
	gs.BlendMobiles = p.BlendMobiles
	gs.BlendPicts = p.BlendPicts
	gs.ShaderLighting = p.ShaderLighting
	gs.SpriteUpscaleFilter = p.SpriteUpscaleFilter
	gs.HighQualityResampling = p.HighQualityResampling
	setHighQualityResamplingEnabled(gs.HighQualityResampling)
	gs.SoundEnhancement = p.SoundEnhancement
	gs.SoundEnhancementAmount = clampSoundEnhancementAmount(p.SoundEnhancementAmount)
	gs.MusicEnhancement = p.MusicEnhancement
	gs.SpriteUpscale = spriteUpscaleFactor()

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
	if shaderLightingCB != nil {
		shaderLightingCB.Checked = gs.ShaderLighting
	}
	if upscaleFilterCB != nil {
		upscaleFilterCB.Checked = gs.SpriteUpscaleFilter
	}
	if soundEnhanceCB != nil {
		soundEnhanceCB.Checked = gs.SoundEnhancement
	}
	if soundEnhanceSlider != nil {
		soundEnhanceSlider.Value = float32(gs.SoundEnhancementAmount)
		soundEnhanceSlider.Disabled = !gs.SoundEnhancement
	}
	if resampleAudioCB != nil {
		resampleAudioCB.Checked = gs.HighQualityResampling
	}
	if musicEnhanceCB != nil {
		musicEnhanceCB.Checked = gs.MusicEnhancement
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
		shaderLightSlider.Disabled = !gs.ShaderLighting
	}
	if shaderGlowSlider != nil {
		shaderGlowSlider.Disabled = !gs.ShaderLighting
	}
}

func matchesPreset(p qualityPreset) bool {
	if gs.DenoiseImages != p.DenoiseImages ||
		gs.MotionSmoothing != p.MotionSmoothing ||
		gs.BlendMobiles != p.BlendMobiles ||
		gs.BlendPicts != p.BlendPicts ||
		gs.ShaderLighting != p.ShaderLighting ||
		gs.SpriteUpscaleFilter != p.SpriteUpscaleFilter ||
		gs.HighQualityResampling != p.HighQualityResampling ||
		gs.SoundEnhancement != p.SoundEnhancement ||
		gs.MusicEnhancement != p.MusicEnhancement {
		return false
	}
	if p.SoundEnhancement {
		if math.Abs(gs.SoundEnhancementAmount-p.SoundEnhancementAmount) > 0.05 {
			return false
		}
	}
	return true
}

func detectQualityPreset() int {
	switch {
	case matchesPreset(classicPreset):
		return 0
	case matchesPreset(lowPreset):
		return 1
	case matchesPreset(mediumPreset):
		return 2
	case matchesPreset(highPreset):
		return 3
	default:
		return 2
	}
}
