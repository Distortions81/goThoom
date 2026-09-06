package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gothoom/climg"
	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
)

const SETTINGS_VERSION = 4

type BarPlacement int

const (
	BarPlacementBottom BarPlacement = iota
	BarPlacementLowerLeft
	BarPlacementLowerRight
	BarPlacementUpperRight
)

type ToolbarPlacement int

const (
	ToolbarInInventory ToolbarPlacement = iota
	ToolbarInPlayers
	ToolbarFloating
)

// TiledLayout selects one of the managed workspace arrangements.
type TiledLayout int

const (
	// TiledLayoutCenter keeps the game view between two side columns.
	TiledLayoutCenter TiledLayout = iota
	// TiledLayoutSide places the game view on one side and the other panels on
	// the opposite side.
	TiledLayoutSide
)

var gs settings = gsdef

var gammaOptions = []float64{1.8, 2.0, 2.2, 2.4}

var defaultPrecacheSounds = precacheSoundsDefault(systemMemoryBytes(), isWASM)

const gibibyte = uint64(1024 * 1024 * 1024)

func precacheSoundsDefault(totalMemory uint64, wasm bool) bool {
	if wasm || totalMemory == 0 {
		return false
	}
	return totalMemory >= 4*gibibyte
}

// settingsLoaded reports whether settings were successfully loaded from disk.
var settingsLoaded bool

// windowsRestored tracks whether window geometry has been restored.
// Initialization defers restoration until the first layout provides a final
// screen size.
var windowsRestored bool

func spriteUpscaleFactorFromScale(scale float64) int {
	factor := int(math.Round(scale))
	if factor < 2 {
		factor = 2
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

// clampMusicEnhancementAmount keeps the bard-music ambience control within a
// useful range. A value of 1 preserves the original enhanced mix; lower values
// are subtler and 2 is deliberately pronounced without overwhelming the tune.
func clampMusicEnhancementAmount(v float64) float64 {
	if v < 0.1 {
		return 0.1
	}
	if v > 2 {
		return 2
	}
	return v
}

func clampUIScalePreference(v float64) float64 {
	if v < 0.75 {
		return 0.75
	}
	if v > 4 {
		return 4
	}
	return v
}

func clampTiledPaneFraction(v float64) float64 {
	if v < 0.15 {
		return 0.15
	}
	if v > 0.60 {
		return 0.60
	}
	return v
}

func clampTiledLayoutSettings() {
	if gs.TiledLayout < TiledLayoutCenter || gs.TiledLayout > TiledLayoutSide {
		gs.TiledLayout = gsdef.TiledLayout
	}
	gs.TiledLeftBottom = clampTiledPaneFraction(gs.TiledLeftBottom)
	gs.TiledRightBottom = clampTiledPaneFraction(gs.TiledRightBottom)
	gs.TiledGamePosition = math.Min(math.Max(gs.TiledGamePosition, -1), 1)
	gs.TiledLeftWidth = math.Min(math.Max(gs.TiledLeftWidth, 0.15), 0.55)
	gs.TiledRightWidth = math.Min(math.Max(gs.TiledRightWidth, 0.15), 0.55)
	gs.TiledSideGameWidth = math.Min(math.Max(gs.TiledSideGameWidth, 0.35), 0.80)
	gs.TiledSideTopSplit = math.Min(math.Max(gs.TiledSideTopSplit, 0.15), 0.85)
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

	LastCharacter:                 "",
	ClickToToggle:                 false,
	MiddleClickMoveWindow:         false,
	InputBarAlwaysOpen:            true,
	KBWalkSpeed:                   0.25,
	MainFontSize:                  8,
	BubbleFontSize:                20,
	ConsoleFontSize:               12,
	ChatFontSize:                  12,
	InventoryFontSize:             14,
	PlayersFontSize:               14,
	ShowRecentPlayers:             true,
	GroupClanMembers:              false,
	PlayerShareIcons:              false,
	InventoryAlternatingRowColors: true,
	ChatAlternatingRowColors:      false,
	ConsoleAlternatingRowColors:   false,
	PlayersAlternatingRowColors:   false,
	BubbleOpacity:                 0.8,
	BubbleLifetimeMode:            BubbleLifetimeModern,
	BubbleBaseLife:                2,
	BubbleLifePerWord:             1,
	BubbleScale:                   2.0,
	NameBgOpacity:                 0.8,
	DarkBubblesAndNames:           true,
	NameHealthBarModern:           true,
	NameHealthBarAbove:            true,
	NameHealthBarThickness:        3,
	NameTagLabelColors:            true,
	HideSelfNameTag:               false,
	NameTagsOnHoverOnly:           false,
	BarOpacity:                    0.66,
	ObscuringPictureOpacity:       0.66,
	FadeObscuringPictures:         true,
	SpeechBubbles:                 true,
	AnimatedChatBubbles:           false,
	AvoidBubbleOverlap:            true,
	BubbleNormal:                  true,
	BubbleWhisper:                 true,
	BubbleYell:                    true,
	BubbleThought:                 true,
	BubbleRealAction:              true,
	BubbleMonster:                 true,
	BubblePlayerAction:            true,
	BubblePonder:                  true,
	BubbleNarrate:                 true,
	BubbleSelf:                    true,
	BubbleOtherPlayers:            true,
	BubbleMonsters:                true,
	BubbleNarration:               true,

	MotionSmoothing:                true,
	InterpolateSmallMovingPictures: true,
	FloatingPointSpriteCoords:      true,
	ObjectPinning:                  true,
	BlendMobiles:                   false,
	BlendPicts:                     true,
	BlendAmount:                    1.0,
	MobileBlendAmount:              0.25,
	DenoiseImages:                  false,
	DenoiseSharpness:               10,
	DenoiseAmount:                  0.35,
	ShowFPS:                        true,
	UIScale:                        1.0,
	Fullscreen:                     false,
	AlwaysOnTop:                    false,
	MasterVolume:                   1.0,
	GameVolume:                     0.28260868787765503,
	MusicVolume:                    1.0,
	Music:                          true,
	GameSound:                      true,
	Mute:                           false,
	GameScale:                      2.0,
	SpriteUpscale:                  2,
	SpriteUpscaleFilter:            true,
	SpriteUpscaleMode:              artworkUpscaleBalanced,
	ReplacementEffects:             false,
	SpriteGammaCorrection:          true,
	SpriteGamma:                    1.8,
	MonitorGamma:                   2.2,
	BarPlacement:                   BarPlacementBottom,
	MaxNightLevel:                  100,
	MessagesToConsole:              true,
	ChatTTS:                        false,
	ChatTTSVolume:                  0.20652173459529877,
	ChatTTSSpeed:                   1.25,
	ChatTTSVoice:                   "en_US-hfc_female-medium",
	Notifications:                  true,
	NotifyWhenBackground:           false,
	// Power saving defaults: limit FPS in background
	PowerSaveBackground:   false,
	PowerSaveAlways:       false,
	PowerSaveFPS:          15,
	MuteWhenUnfocused:     false,
	NotifyFallen:          true,
	NotifyNotFallen:       true,
	NotifyShares:          true,
	NotifyFriendOnline:    true,
	NotifyCopyText:        false,
	NotificationVolume:    0.20652173459529877,
	NotificationBeep:      true,
	NotificationDuration:  6,
	ScriptSpamKill:        true,
	LegacyMacroContinuous: false,
	PromptOnSaveRecording: true,
	AutoRecord:            false,
	PromptDisableShaders:  true,
	ChatTimestamps:        true,
	ConsoleTimestamps:     true,
	TimestampFormat:       "3:04PM",
	LastUpdateCheck:       time.Time{},
	NotifiedVersion:       0,
	WindowSnapping:        false,
	WindowShadows:         true,
	AutoResizeWindows:     true,
	TiledWindows:          true,
	TiledLayout:           TiledLayoutCenter,
	TiledKeepGameLarge:    true,
	TiledGamePosition:     0,
	TiledLeftBottom:       0.30,
	TiledRightBottom:      0.30,
	TiledLeftWidth:        0.235,
	TiledRightWidth:       0.260,
	TiledSideGameWidth:    0.60,
	TiledSideTopSplit:     0.50,
	TiledInventoryLeft:    true,
	TiledConsoleLeft:      true,
	TiledGameLeft:         true,
	ToolbarPlacement:      ToolbarInInventory,
	ToolbarInfoBar:        false,

	JoystickEnabled:        false,
	JoystickWalkStick:      0,
	JoystickCursorStick:    1,
	JoystickWalkDeadzone:   0.1,
	JoystickCursorDeadzone: 0.1,

	WindowWidth:  1920,
	WindowHeight: 1080,
	GameWindow: WindowState{
		Open:     true,
		Position: WindowPoint{X: 438.0 / 1858.0},
		Size:     WindowPoint{X: 936.0 / 1858.0, Y: 1},
	},
	InventoryWindow: WindowState{
		Open:     true,
		Position: WindowPoint{},
		Size:     WindowPoint{X: 438.0 / 1858.0, Y: 444.0 / 861.0},
	},
	PlayersWindow: WindowState{
		Open:     true,
		Position: WindowPoint{X: 1374.0 / 1858.0},
		Size:     WindowPoint{X: 484.0 / 1858.0, Y: 526.0 / 946.0},
	},
	MessagesWindow: WindowState{
		Open:     true,
		Position: WindowPoint{Y: 444.0 / 861.0},
		Size:     WindowPoint{X: 438.0 / 1858.0, Y: 417.0 / 861.0},
	},
	ChatWindow: WindowState{
		Open:     true,
		Position: WindowPoint{X: 1374.0 / 1858.0, Y: 526.0 / 946.0},
		Size:     WindowPoint{X: 484.0 / 1858.0, Y: 420.0 / 946.0},
	},
	MovieWindow: WindowState{
		Open:     false,
		Position: WindowPoint{X: 350.0 / 1858.0, Y: 117.0 / 948.0},
	},
	ToolbarWindow: WindowState{
		Open: true,
	},
	ShaderLightStrength:  1.0,
	ShaderGlowStrength:   1.0,
	FlameLightFlicker:    true,
	FlameFlickerStrength: 1.0,

	PotatoGPU:               false,
	BatchArtworkLoading:     true,
	SpriteCacheMiB:          defaultSpriteCacheMiB,
	AssetActivityIndicators: false,
	BarColorByValue:         false,
	ThrottleSounds:          true,
	SoundEnhancement:        true,
	SoundEnhancementAmount:  1.5,
	MusicEnhancement:        true,
	MusicEnhancementAmount:  1.0,
	HighQualityResampling:   true,
	ServerAddress:           defaultServerHostName + ":5010",
	ServerAddresses:         nil,
	AssetsPath:              "",
	LogsPath:                "",
	MacrosPath:              "",
	ScriptsPath:             "",

	NightEffect:              true,
	ShadersEnabled:           true,
	ShaderLighting:           true,
	MobileLightConeShadows:   false,
	CharacterShadows:         true,
	FasterCharacterShadows:   false,
	CharacterShadowDarkness:  1.8,
	MobilesReceiveSunShadows: true,

	// Window behavior
	ShowClanLordSplashImage: true,

	// Advanced and runtime defaults.
	VSync:            true,
	PrecacheSounds:   defaultPrecacheSounds,
	recordAssetStats: false,
	AltNetMode:       true,
	hideMobiles:      false,
	imgPlanesDebug:   false,
	smoothingDebug:   false,
	pictAgainDebug:   false,
	pictIDDebug:      false,
	scriptEventDebug: false,
	forceNightLevel:  -1,
}

type settings struct {
	Version            int
	SetupWizardVersion int

	LastCharacter                 string
	ClickToToggle                 bool
	MiddleClickMoveWindow         bool
	InputBarAlwaysOpen            bool
	KBWalkSpeed                   float64
	MainFontSize                  float64
	BubbleFontSize                float64
	ConsoleFontSize               float64
	ChatFontSize                  float64
	InventoryFontSize             float64
	PlayersFontSize               float64
	ShowRecentPlayers             bool
	GroupClanMembers              bool
	PlayerShareIcons              bool
	PlayerGroups                  customGroups
	InventoryGroups               customGroups
	InventoryAlternatingRowColors bool
	ChatAlternatingRowColors      bool
	ConsoleAlternatingRowColors   bool
	PlayersAlternatingRowColors   bool
	BubbleOpacity                 float64
	BubbleLifetimeMode            string
	BubbleBaseLife                float64
	BubbleLifePerWord             float64
	// BubbleScale scales bubble visuals (not font). Range 1.0–8.0.
	BubbleScale            float64
	NameBgOpacity          float64
	DarkBubblesAndNames    bool
	NameHealthBarModern    bool
	NameHealthBarAbove     bool
	NameHealthBarThickness int
	NameTagLabelColors     bool
	HideSelfNameTag        bool
	// NameTagsOnHoverOnly hides name tags unless the cursor is over a mobile.
	NameTagsOnHoverOnly     bool
	BarOpacity              float64
	ObscuringPictureOpacity float64
	FadeObscuringPictures   bool
	SpeechBubbles           bool
	AnimatedChatBubbles     bool
	AvoidBubbleOverlap      bool
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

	MotionSmoothing                bool
	InterpolateSmallMovingPictures bool
	FloatingPointSpriteCoords      bool
	PixelArtScaling                bool
	ObjectPinning                  bool
	BlendMobiles                   bool
	BlendPicts                     bool
	BlendAmount                    float64
	MobileBlendAmount              float64
	DenoiseImages                  bool
	DenoiseSharpness               float64
	DenoiseAmount                  float64
	ShowFPS                        bool
	UIScale                        float64
	Fullscreen                     bool
	AlwaysOnTop                    bool
	VSync                          bool
	MasterVolume                   float64
	GameVolume                     float64
	MusicVolume                    float64
	Music                          bool
	GameSound                      bool
	Mute                           bool
	GameScale                      float64
	SpriteUpscale                  int
	SpriteUpscaleFilter            bool
	SpriteUpscaleMode              int
	ReplacementEffects             bool
	SpriteGammaCorrection          bool
	SpriteGamma                    float64
	MonitorGamma                   float64
	BarPlacement                   BarPlacement
	MaxNightLevel                  int
	forceNightLevel                int
	Theme                          string
	Style                          string
	MessagesToConsole              bool
	MessageTextColors              map[string]eui.Color
	MessageTextColorsLight         map[string]eui.Color
	OverrideThemeTextColor         bool
	ClassicMessageColors           bool
	ChatTTS                        bool
	ChatTTSVolume                  float64
	ChatTTSSpeed                   float64
	ChatTTSVoice                   string
	ChatTTSBlocklist               []string
	Notifications                  bool
	NotifyWhenBackground           bool
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
	LegacyMacroContinuous bool
	PromptOnSaveRecording bool
	AutoRecord            bool
	PromptDisableShaders  bool
	ChatTimestamps        bool
	ConsoleTimestamps     bool
	TimestampFormat       string
	LastUpdateCheck       time.Time
	NotifiedVersion       int
	WindowSnapping        bool
	WindowShadows         bool
	AutoResizeWindows     bool
	TiledWindows          bool
	TiledLayout           TiledLayout
	TiledKeepGameLarge    bool
	// TiledGamePosition places the fixed-width centered game pane between the
	// side-column limits: -1 is far left, 0 centered, and 1 far right.
	TiledGamePosition float64
	// TiledLeftBottom and TiledRightBottom are the bottom-pane fractions for
	// the left and right columns in the centered layout.
	TiledLeftBottom    float64
	TiledRightBottom   float64
	TiledLeftWidth     float64
	TiledRightWidth    float64
	TiledSideGameWidth float64
	TiledSideTopSplit  float64
	TiledInventoryLeft bool
	TiledConsoleLeft   bool
	TiledGameLeft      bool
	ToolbarPlacement   ToolbarPlacement
	ToolbarInfoBar     bool

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
	MovieWindow     WindowState
	ToolbarWindow   WindowState

	ShaderLightStrength  float64
	ShaderGlowStrength   float64
	FlameLightFlicker    bool
	FlameFlickerStrength float64

	PotatoGPU               bool
	BatchArtworkLoading     bool
	SpriteCacheMiB          int // Reference reserve at 2x; scales with texture area.
	AssetActivityIndicators bool
	PrecacheSounds          bool
	BarColorByValue         bool
	ThrottleSounds          bool
	SoundEnhancement        bool
	SoundEnhancementAmount  float64
	MusicEnhancement        bool
	MusicEnhancementAmount  float64
	HighQualityResampling   bool

	imgPlanesDebug           bool
	smoothingDebug           bool
	pictAgainDebug           bool
	pictIDDebug              bool
	scriptEventDebug         bool
	AltNetMode               bool
	ServerAddress            string
	ServerAddresses          []string
	AssetsPath               string
	LogsPath                 string
	MacrosPath               string
	ScriptsPath              string
	hideMoving               bool
	hideMobiles              bool
	NightEffect              bool
	ShadersEnabled           bool
	ShaderLighting           bool
	MobileLightConeShadows   bool
	CharacterShadows         bool
	FasterCharacterShadows   bool
	CharacterShadowDarkness  float64
	MobilesReceiveSunShadows bool

	// Window behavior
	ShowClanLordSplashImage bool
	recordAssetStats        bool
}

var (
	settingsDirty    bool
	lastSettingsSave = time.Now()
)

type WindowPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type WindowState struct {
	Open bool `json:"open"`
	// Position and resizable window Size are fractions of the UI screen.
	// Fixed-size windows ignore Size and retain their content-defined size.
	Position WindowPoint `json:"position"`
	Size     WindowPoint `json:"size"`

	runtimePosition WindowPoint
	runtimeSize     WindowPoint
	runtimeApplied  bool
}

const settingsFile = "settings.json"

func loadSettings() bool {
	defer syncTTSBlocklist()
	defer func() { gs = initializeCharacterProfiles(gs) }()
	path := filepath.Join(dataDirPath, settingsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		gs = gsdef
		setHighQualityResamplingEnabled(gs.HighQualityResampling)
		settingsLoaded = false
		applyServerAddressSetting()
		return false
	}

	type legacySettingsFile struct {
		settings
		LegacyAlternateRows *bool `json:"AlternateRowBackgrounds"`
		LegacySoundReverb   *bool `json:"SoundReverb"`
		LegacyMusicReverb   *bool `json:"MusicReverb"`
	}

	version, modern, err := settingsDocumentVersion(data)
	if err != nil {
		gs = gsdef
		setHighQualityResamplingEnabled(gs.HighQualityResampling)
		settingsLoaded = false
		return false
	}

	migrated := false
	switch {
	case modern && version == SETTINGS_VERSION:
		loaded, err := unmarshalSettingsDocument(data, gsdef)
		if err != nil {
			gs = gsdef
			setHighQualityResamplingEnabled(gs.HighQualityResampling)
			settingsLoaded = false
			return false
		}
		gs = loaded
	case !modern && version > 0 && version < SETTINGS_VERSION:
		tmp := legacySettingsFile{settings: gsdef}
		if err := json.Unmarshal(data, &tmp); err != nil {
			gs = gsdef
			setHighQualityResamplingEnabled(gs.HighQualityResampling)
			settingsLoaded = false
			return false
		}
		if tmp.LegacySoundReverb != nil {
			tmp.settings.SoundEnhancement = *tmp.LegacySoundReverb
		}
		if tmp.LegacyMusicReverb != nil {
			tmp.settings.MusicEnhancement = *tmp.LegacyMusicReverb
		}
		if tmp.LegacyAlternateRows != nil {
			tmp.settings.InventoryAlternatingRowColors = *tmp.LegacyAlternateRows
		}
		gs = tmp.settings
		gs.Version = SETTINGS_VERSION
		migrated = true
	default:
		gs = gsdef
		applyQualityPreset("High")
		setHighQualityResamplingEnabled(gs.HighQualityResampling)
		settingsLoaded = false
		applyServerAddressSetting()
		return false
	}

	setHighQualityResamplingEnabled(gs.HighQualityResampling)
	settingsLoaded = true
	if migrated {
		settingsDirty = true
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
	setArtworkUpscaleMode(artworkUpscaleMode())
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
	if gs.FlameFlickerStrength < 0 || gs.FlameFlickerStrength > 2 {
		gs.FlameFlickerStrength = gsdef.FlameFlickerStrength
	}
	if gs.CharacterShadowDarkness < 0.01 || gs.CharacterShadowDarkness > 2 {
		gs.CharacterShadowDarkness = gsdef.CharacterShadowDarkness
	}
	if gs.NameHealthBarThickness < 1 || gs.NameHealthBarThickness > 8 {
		gs.NameHealthBarThickness = gsdef.NameHealthBarThickness
	}

	gs.SoundEnhancementAmount = clampSoundEnhancementAmount(gs.SoundEnhancementAmount)
	gs.MusicEnhancementAmount = clampMusicEnhancementAmount(gs.MusicEnhancementAmount)
	gs.UIScale = clampUIScalePreference(gs.UIScale)
	clampTiledLayoutSettings()

	// Clamp BubbleScale to 1.0–8.0
	if gs.BubbleScale < 1.0 || gs.BubbleScale > 8.0 {
		gs.BubbleScale = gsdef.BubbleScale
	}
	gs.BubbleLifetimeMode = normalizeBubbleLifetimeMode(gs.BubbleLifetimeMode)

	gs.SpriteUpscale = spriteUpscaleFactor()
	if gs.ToolbarPlacement < ToolbarInInventory || gs.ToolbarPlacement > ToolbarFloating {
		gs.ToolbarPlacement = gsdef.ToolbarPlacement
		settingsDirty = true
	}
	if gs.TiledWindows && gs.ToolbarPlacement == ToolbarFloating {
		gs.ToolbarPlacement = ToolbarInInventory
		settingsDirty = true
	}
	if !normalizedWindowSettingsValid() {
		resetSavedWindowSettings()
		settingsDirty = true
	}

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
	normalizeServerListSettings()
	addr := strings.TrimSpace(gs.ServerAddress)
	if addr == "" {
		addr = gsdef.ServerAddress
	}
	gs.ServerAddress = addr
	host = addr
	if serverAddressOverride != "" {
		host = serverAddressOverride
	}
}

func effectiveVSyncEnabled() bool {
	return gs.VSync && !setupWizardVSyncBypass
}

func applyVSyncSetting() {
	ebiten.SetVsyncEnabled(effectiveVSyncEnabled())
}

func applySettings() {
	setClientActivityIndicatorsEnabled(gs.AssetActivityIndicators)
	eui.SetWindowSnapping(gs.WindowSnapping)
	eui.SetMiddleClickMove(gs.MiddleClickMoveWindow)
	eui.SetPotatoMode(gs.PotatoGPU)
	applyWindowShadowsSetting()
	climg.SetPotatoMode(gs.PotatoGPU)
	if clImages != nil {
		clImages.SetDenoise(gs.DenoiseImages, gs.DenoiseSharpness, gs.DenoiseAmount)
		clImages.SetGammaCorrection(gs.SpriteGammaCorrection, gs.SpriteGamma, gs.MonitorGamma)
	}
	applyVSyncSetting()
	if runtime := legacyMacroRuntimeSnapshot(); runtime != nil {
		runtime.setAllowContinuous(gs.LegacyMacroContinuous)
	}
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
	if err := writeSettingsFile(settingsForSave()); err != nil {
		logError("save settings: %v", err)
	}
}

func writeSettingsFile(value settings) error {
	data, err := marshalSettingsDocument(value)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	path := filepath.Join(dataDirPath, settingsFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".settings-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	written, err := os.ReadFile(temporaryPath)
	if err != nil {
		return err
	}
	if !bytes.Equal(written, data) {
		return fmt.Errorf("settings verification failed")
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	written, err = os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(written, data) {
		return fmt.Errorf("saved settings verification failed")
	}
	return nil
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
	if syncWindow(movieWin, &gs.MovieWindow) {
		changed = true
	}
	if gs.ToolbarPlacement == ToolbarFloating {
		if syncWindow(hudWin, &gs.ToolbarWindow) {
			changed = true
		}
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
	gs.MovieWindow = gsdef.MovieWindow
	gs.ToolbarWindow = gsdef.ToolbarWindow
	gs.TiledWindows = gsdef.TiledWindows
	gs.TiledLayout = gsdef.TiledLayout
	gs.TiledKeepGameLarge = gsdef.TiledKeepGameLarge
	gs.TiledGamePosition = gsdef.TiledGamePosition
	gs.TiledLeftBottom = gsdef.TiledLeftBottom
	gs.TiledRightBottom = gsdef.TiledRightBottom
	gs.TiledLeftWidth = gsdef.TiledLeftWidth
	gs.TiledRightWidth = gsdef.TiledRightWidth
	gs.TiledSideGameWidth = gsdef.TiledSideGameWidth
	gs.TiledSideTopSplit = gsdef.TiledSideTopSplit
	gs.TiledInventoryLeft = gsdef.TiledInventoryLeft
	gs.TiledConsoleLeft = gsdef.TiledConsoleLeft
	gs.TiledGameLeft = gsdef.TiledGameLeft
	gs.MessagesToConsole = gsdef.MessagesToConsole
	gs.ToolbarPlacement = gsdef.ToolbarPlacement
	gs.ToolbarInfoBar = gsdef.ToolbarInfoBar
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
	pos := windowPoint(win.GetPos())
	size := windowPoint(win.GetSize())
	if !state.runtimeApplied || !windowPointsNear(pos, state.runtimePosition) || !windowPointsNear(size, state.runtimeSize) {
		sx, sy := eui.ScreenSize()
		if sx > 0 && sy > 0 {
			normalizedPos := WindowPoint{X: pos.X / float64(sx), Y: pos.Y / float64(sy)}
			if state.Position != normalizedPos {
				state.Position = normalizedPos
				changed = true
			}
			if win.Resizable {
				normalizedSize := WindowPoint{X: size.X / float64(sx), Y: size.Y / float64(sy)}
				if state.Size != normalizedSize {
					state.Size = normalizedSize
					changed = true
				}
			}
			clampWindowState(state)
		}
	}
	state.runtimePosition = pos
	state.runtimeSize = size
	state.runtimeApplied = true
	return changed
}

func clampWindowSettings() {
	states := []*WindowState{&gs.GameWindow, &gs.InventoryWindow, &gs.PlayersWindow, &gs.MessagesWindow, &gs.ChatWindow, &gs.MovieWindow, &gs.ToolbarWindow}
	for _, st := range states {
		clampWindowState(st)
	}
}

func clampWindowState(st *WindowState) {
	st.Position.X = math.Min(math.Max(st.Position.X, 0), 1)
	st.Position.Y = math.Min(math.Max(st.Position.Y, 0), 1)
	st.Size.X = math.Min(math.Max(st.Size.X, 0), 1)
	st.Size.Y = math.Min(math.Max(st.Size.Y, 0), 1)
}

func applyWindowState(win *eui.WindowData, st *WindowState) {
	if win == nil || st == nil {
		return
	}
	sx, sy := eui.ScreenSize()
	if sx <= 0 || sy <= 0 {
		return
	}
	win.ClearZone()
	if win.Resizable && st.Size.X > 0 && st.Size.Y > 0 {
		_ = win.SetSize(eui.Point{X: float32(st.Size.X * float64(sx)), Y: float32(st.Size.Y * float64(sy))})
	}
	_ = win.SetPos(eui.Point{X: float32(st.Position.X * float64(sx)), Y: float32(st.Position.Y * float64(sy))})
	captureWindowRuntime(win, st)
	if st.Open {
		win.MarkOpen()
	} else if win.IsOpen() {
		win.Close()
	}
}

func windowPoint(p eui.Point) WindowPoint {
	return WindowPoint{X: float64(p.X), Y: float64(p.Y)}
}

func windowPointsNear(a, b WindowPoint) bool {
	return math.Abs(a.X-b.X) <= 0.5 && math.Abs(a.Y-b.Y) <= 0.5
}

func captureWindowRuntime(win *eui.WindowData, st *WindowState) {
	st.runtimePosition = windowPoint(win.GetPos())
	st.runtimeSize = windowPoint(win.GetSize())
	st.runtimeApplied = true
}

func normalizedWindowStateValid(st WindowState, resizable bool) bool {
	if st.Position.X < 0 || st.Position.X > 1 || st.Position.Y < 0 || st.Position.Y > 1 {
		return false
	}
	if !resizable {
		return true
	}
	return st.Size.X > 0 && st.Size.X <= 1 && st.Size.Y > 0 && st.Size.Y <= 1
}

func normalizedWindowSettingsValid() bool {
	return normalizedWindowStateValid(gs.GameWindow, true) &&
		normalizedWindowStateValid(gs.InventoryWindow, true) &&
		normalizedWindowStateValid(gs.PlayersWindow, true) &&
		normalizedWindowStateValid(gs.MessagesWindow, true) &&
		normalizedWindowStateValid(gs.ChatWindow, true) &&
		normalizedWindowStateValid(gs.MovieWindow, false) &&
		normalizedWindowStateValid(gs.ToolbarWindow, false)
}

var (
	windowLayoutScreenWidth  int
	windowLayoutScreenHeight int
	windowLayoutUIScale      float32
)

func applyManagedWindowLayout() {
	applyTiledWindowStates()
	if gs.TiledWindows && gs.MessagesToConsole && chatWin != nil && chatWin.IsOpen() {
		chatWin.Close()
	}
	prepareTiledWorkspaceWindowChrome()
	applyWindowState(gameWin, &gs.GameWindow)
	applyWindowState(inventoryWin, &gs.InventoryWindow)
	applyWindowState(playersWin, &gs.PlayersWindow)
	applyWindowState(consoleWin, &gs.MessagesWindow)
	applyWindowState(chatWin, &gs.ChatWindow)
	applyWindowState(movieWin, &gs.MovieWindow)
	applyWindowState(hudWin, &gs.ToolbarWindow)
	managed := []struct {
		win   *eui.WindowData
		state *WindowState
	}{
		{gameWin, &gs.GameWindow},
		{inventoryWin, &gs.InventoryWindow},
		{playersWin, &gs.PlayersWindow},
		{consoleWin, &gs.MessagesWindow},
		{chatWin, &gs.ChatWindow},
		{movieWin, &gs.MovieWindow},
		{hudWin, &gs.ToolbarWindow},
	}
	for _, item := range managed {
		if item.win != nil {
			captureWindowRuntime(item.win, item.state)
		}
	}
	finishTiledWorkspaceWindowChrome()
	configureTiledWorkspaceDividers()

	windowLayoutScreenWidth, windowLayoutScreenHeight = eui.ScreenSize()
	windowLayoutUIScale = eui.UIScale()
}

func tiledWorkspaceWindows() []struct {
	win  *eui.WindowData
	game bool
} {
	return []struct {
		win  *eui.WindowData
		game bool
	}{
		{gameWin, true},
		{inventoryWin, false},
		{playersWin, false},
		{consoleWin, false},
		{chatWin, false},
	}
}

// prepareTiledWorkspaceWindowChrome temporarily enables resizing while a
// managed layout is being applied. The resize affordance is removed again by
// finishTiledWorkspaceWindowChrome once every tile is in place.
func prepareTiledWorkspaceWindowChrome() {
	for _, item := range tiledWorkspaceWindows() {
		if item.win == nil {
			continue
		}
		if gs.TiledWindows {
			if item.game {
				if item.win.GetRawTitleSize() > 0 {
					gameWindowFreeformTitleHeight = item.win.GetRawTitleSize()
				}
				if item.win.Padding > 0 {
					gameWindowFreeformPadding = item.win.Padding
				}
				if item.win.Margin > 0 {
					gameWindowFreeformMargin = item.win.Margin
				}
				item.win.TitleHeight = 0
				item.win.Padding = 0
				item.win.Margin = 0
			}
			item.win.SetDocked(true)
			item.win.Closable = false
			item.win.Maximizable = false
			item.win.Movable = false
			item.win.Resizable = true
			continue
		}
		item.win.SetDocked(false)
		if item.game && gameWindowFreeformTitleHeight > 0 {
			item.win.TitleHeight = gameWindowFreeformTitleHeight
			item.win.Padding = gameWindowFreeformPadding
			item.win.Margin = gameWindowFreeformMargin
		}
		item.win.Resizable = true
		item.win.Closable = !item.game
		item.win.Movable = true
		if item.game {
			item.win.Maximizable = true
		}
	}
}

func finishTiledWorkspaceWindowChrome() {
	if !gs.TiledWindows {
		return
	}
	for _, item := range tiledWorkspaceWindows() {
		if item.win == nil {
			continue
		}
		item.win.Resizable = false
		item.win.SetDocked(true)
		item.win.Closable = false
		item.win.Maximizable = false
		item.win.Movable = false
	}
}

func tiledWindowState(state *WindowState, x, y, width, height float64) {
	state.Position = WindowPoint{X: x, Y: y}
	state.Size = WindowPoint{X: width, Y: height}
	clampWindowState(state)
}

// applyTiledWindowStates derives the persisted geometry from the selected
// workspace. The normal window-state path continues to handle freeform mode.
func applyTiledWindowStates() {
	if !gs.TiledWindows {
		return
	}
	clampTiledLayoutSettings()
	if width, height := eui.ScreenSize(); width > 0 && height > 0 {
		toolbarMinimum := 0.0
		if gs.ToolbarPlacement != ToolbarFloating {
			toolbarMinimum = dockedToolbarMinimumWidth * float64(eui.UIScale()) / float64(width)
		}
		if gs.TiledLayout == TiledLayoutCenter && gs.TiledKeepGameLarge {
			maximizeCenteredGameForWorkspace(width, height, toolbarMinimum)
		}
		clampTiledLayoutForToolbar(toolbarMinimum)
	}
	gs.GameWindow.Open = true
	gs.InventoryWindow.Open = true
	gs.PlayersWindow.Open = true
	gs.MessagesWindow.Open = true
	gs.ChatWindow.Open = !gs.MessagesToConsole
	if gs.TiledLayout == TiledLayoutSide {
		// The side-game arrangement reserves one bottom pane for combined
		// messages, matching its two-panel upper workspace.
		gs.MessagesToConsole = true
		gs.ChatWindow.Open = false
		applySideTiledWindowStates()
		return
	}
	applyCenteredTiledWindowStates()
}

func applyCenteredTiledWindowStates() {
	leftWidth := gs.TiledLeftWidth
	rightWidth := gs.TiledRightWidth
	gameWidth := 1 - leftWidth - rightWidth
	leftBottom := gs.TiledLeftBottom
	rightBottom := gs.TiledRightBottom

	if gs.MessagesToConsole {
		// Chat output is folded into Console. The Chat tile disappears, the
		// game remains full-height, and only the side containing Console keeps
		// a bottom pane.
		leftTop := 1.0
		rightTop := 1.0
		if gs.TiledConsoleLeft {
			leftTop = 1 - leftBottom
		} else {
			rightTop = 1 - rightBottom
		}
		tiledWindowState(&gs.GameWindow, leftWidth, 0, gameWidth, 1)
		if gs.TiledInventoryLeft {
			tiledWindowState(&gs.InventoryWindow, 0, 0, leftWidth, leftTop)
			tiledWindowState(&gs.PlayersWindow, leftWidth+gameWidth, 0, rightWidth, rightTop)
		} else {
			tiledWindowState(&gs.PlayersWindow, 0, 0, leftWidth, leftTop)
			tiledWindowState(&gs.InventoryWindow, leftWidth+gameWidth, 0, rightWidth, rightTop)
		}
		if gs.TiledConsoleLeft {
			tiledWindowState(&gs.MessagesWindow, 0, leftTop, leftWidth, leftBottom)
		} else {
			tiledWindowState(&gs.MessagesWindow, leftWidth+gameWidth, rightTop, rightWidth, rightBottom)
		}
		gs.ChatWindow.Open = false
		return
	}

	tiledWindowState(&gs.GameWindow, leftWidth, 0, gameWidth, 1)
	if gs.TiledInventoryLeft {
		tiledWindowState(&gs.InventoryWindow, 0, 0, leftWidth, 1-leftBottom)
		tiledWindowState(&gs.PlayersWindow, leftWidth+gameWidth, 0, rightWidth, 1-rightBottom)
	} else {
		tiledWindowState(&gs.PlayersWindow, 0, 0, leftWidth, 1-leftBottom)
		tiledWindowState(&gs.InventoryWindow, leftWidth+gameWidth, 0, rightWidth, 1-rightBottom)
	}
	if gs.TiledConsoleLeft {
		tiledWindowState(&gs.MessagesWindow, 0, 1-leftBottom, leftWidth, leftBottom)
		tiledWindowState(&gs.ChatWindow, leftWidth+gameWidth, 1-rightBottom, rightWidth, rightBottom)
	} else {
		tiledWindowState(&gs.ChatWindow, 0, 1-leftBottom, leftWidth, leftBottom)
		tiledWindowState(&gs.MessagesWindow, leftWidth+gameWidth, 1-rightBottom, rightWidth, rightBottom)
	}
}

func applySideTiledWindowStates() {
	bottom := gs.TiledRightBottom
	top := 1 - bottom
	gameWidth := gs.TiledSideGameWidth
	panelWidth := 1 - gameWidth
	panelX := gameWidth
	gameX := 0.0
	if !gs.TiledGameLeft {
		gameX = panelWidth
		panelX = 0
	}
	tiledWindowState(&gs.GameWindow, gameX, 0, gameWidth, 1)
	firstWidth := panelWidth * gs.TiledSideTopSplit
	secondWidth := panelWidth - firstWidth
	if gs.TiledInventoryLeft {
		tiledWindowState(&gs.InventoryWindow, panelX, 0, firstWidth, top)
		tiledWindowState(&gs.PlayersWindow, panelX+firstWidth, 0, secondWidth, top)
	} else {
		tiledWindowState(&gs.PlayersWindow, panelX, 0, firstWidth, top)
		tiledWindowState(&gs.InventoryWindow, panelX+firstWidth, 0, secondWidth, top)
	}
	if gs.MessagesToConsole {
		tiledWindowState(&gs.MessagesWindow, panelX, top, panelWidth, bottom)
		gs.ChatWindow.Open = false
		return
	}
	// Keep both message windows available when the user has not chosen the
	// combined-message mode; each gets half of the bottom panel.
	tiledWindowState(&gs.MessagesWindow, panelX, top, panelWidth/2, bottom)
	tiledWindowState(&gs.ChatWindow, panelX+panelWidth/2, top, panelWidth/2, bottom)
}

func managedWindowLayoutChanged() bool {
	sx, sy := eui.ScreenSize()
	return sx != windowLayoutScreenWidth || sy != windowLayoutScreenHeight || eui.UIScale() != windowLayoutUIScale
}

func restoreWindowSettings() {
	// Login has no persisted geometry and should start in the true screen
	// center even when an older saved zone table says it was unpinned.
	centerLoginWindow()
	applyManagedWindowLayout()
	if gameWin != nil {
		gameWin.MarkOpen()
	}
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
	artworkUpscaleMode       int
	fadeObscuringPictures    bool
	precacheSounds           bool
	windowShadows            bool
	characterShadows         bool
	shadersEnabled           bool
	shaderLighting           bool
	blendPicts               bool
	mobilesReceiveSunShadows bool
	musicEnhancement         bool
	soundEnhancement         bool
	highQualityResampling    bool
}

var (
	lowestPreset = qualityPreset{
		artworkUpscaleMode: artworkUpscaleOff,
	}
	lowPreset = qualityPreset{
		artworkUpscaleMode: artworkUpscaleBalanced,
		characterShadows:   true,
	}
	mediumPreset = qualityPreset{
		artworkUpscaleMode: artworkUpscaleBalanced,
		precacheSounds:     true,
		windowShadows:      true,
		characterShadows:   true,
		shadersEnabled:     true,
		shaderLighting:     true,
	}
	highPreset = qualityPreset{
		artworkUpscaleMode:       artworkUpscaleBalanced,
		fadeObscuringPictures:    true,
		precacheSounds:           true,
		windowShadows:            true,
		characterShadows:         true,
		shadersEnabled:           true,
		shaderLighting:           true,
		blendPicts:               true,
		mobilesReceiveSunShadows: true,
		musicEnhancement:         true,
	}
	ultraPreset = qualityPreset{
		artworkUpscaleMode:       artworkUpscaleBalanced,
		fadeObscuringPictures:    true,
		precacheSounds:           true,
		windowShadows:            true,
		characterShadows:         true,
		shadersEnabled:           true,
		shaderLighting:           true,
		blendPicts:               true,
		mobilesReceiveSunShadows: true,
		musicEnhancement:         true,
		soundEnhancement:         true,
		highQualityResampling:    true,
	}
)

func applyQualityPreset(name string) {
	p := lowestPreset
	switch name {
	case "Lowest":
	case "Low":
		p = lowPreset
	case "Medium":
		p = mediumPreset
	case "High":
		p = highPreset
	case "Ultra":
		p = ultraPreset
	default:
		return
	}

	setArtworkUpscaleMode(p.artworkUpscaleMode)
	gs.FadeObscuringPictures = p.fadeObscuringPictures
	gs.PrecacheSounds = p.precacheSounds
	gs.WindowShadows = p.windowShadows
	gs.CharacterShadows = p.characterShadows
	gs.ShadersEnabled = p.shadersEnabled
	gs.ShaderLighting = p.shaderLighting
	gs.BlendPicts = p.blendPicts
	gs.MobilesReceiveSunShadows = p.mobilesReceiveSunShadows
	gs.MusicEnhancement = p.musicEnhancement
	gs.SoundEnhancement = p.soundEnhancement
	gs.HighQualityResampling = p.highQualityResampling
	setHighQualityResamplingEnabled(gs.HighQualityResampling)
	gs.SpriteUpscale = spriteUpscaleFactor()

	if pictBlendCB != nil {
		pictBlendCB.Checked = gs.BlendPicts
	}
	if qualityRenderScaleSlider != nil {
		qualityRenderScaleSlider.Value = float32(gs.GameScale)
	}
	if fadeObscuringCB != nil {
		fadeObscuringCB.Checked = gs.FadeObscuringPictures
	}
	if precacheSoundCB != nil {
		precacheSoundCB.Checked = gs.PrecacheSounds
	}
	if characterShadowsCB != nil {
		characterShadowsCB.Checked = gs.CharacterShadows
	}
	if characterShadowSlider != nil {
		characterShadowSlider.Disabled = !gs.CharacterShadows
	}
	if mobileSunShadowsCB != nil {
		mobileSunShadowsCB.Checked = gs.MobilesReceiveSunShadows
		mobileSunShadowsCB.Disabled = !gs.CharacterShadows
	}
	if potatoCB != nil {
		potatoCB.Checked = gs.PotatoGPU
	}
	if shaderLightingCB != nil {
		shaderLightingCB.Checked = gs.ShaderLighting
	}
	refreshShaderEffectControls()
	if upscaleModeDD != nil {
		upscaleModeDD.Selected = artworkUpscaleMode()
	}
	if resampleAudioCB != nil {
		resampleAudioCB.Checked = gs.HighQualityResampling
	}
	refreshMixerEnhancementControls()
	applySettings()
	clearCaches()
	settingsDirty = true
	if settingsWin != nil {
		settingsWin.Refresh()
	}
	if graphicsWin != nil {
		graphicsWin.Refresh()
	}
	if debugWin != nil {
		debugWin.Refresh()
	}
}

func matchesPreset(p qualityPreset) bool {
	return artworkUpscaleMode() == p.artworkUpscaleMode &&
		gs.FadeObscuringPictures == p.fadeObscuringPictures &&
		gs.PrecacheSounds == p.precacheSounds &&
		gs.WindowShadows == p.windowShadows &&
		gs.CharacterShadows == p.characterShadows &&
		gs.ShadersEnabled == p.shadersEnabled &&
		gs.ShaderLighting == p.shaderLighting &&
		gs.BlendPicts == p.blendPicts &&
		gs.MobilesReceiveSunShadows == p.mobilesReceiveSunShadows &&
		gs.MusicEnhancement == p.musicEnhancement &&
		gs.SoundEnhancement == p.soundEnhancement &&
		gs.HighQualityResampling == p.highQualityResampling
}

func detectQualityPreset() int {
	switch {
	case matchesPreset(lowestPreset):
		return 0
	case matchesPreset(lowPreset):
		return 1
	case matchesPreset(mediumPreset):
		return 2
	case matchesPreset(highPreset):
		return 3
	case matchesPreset(ultraPreset):
		return 4
	default:
		return 5
	}
}
