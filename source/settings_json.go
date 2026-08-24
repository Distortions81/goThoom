package main

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// settingsDocument is the human-facing v4 settings format. Keep categories
// broad and stable so settings remain easy to browse and edit by hand.
type settingsDocument struct {
	Version       int                        `json:"version"`
	General       map[string]json.RawMessage `json:"general"`
	Controls      map[string]json.RawMessage `json:"controls"`
	Interface     map[string]json.RawMessage `json:"interface"`
	SpeechBubbles map[string]json.RawMessage `json:"speech_bubbles"`
	Rendering     map[string]json.RawMessage `json:"rendering"`
	Audio         map[string]json.RawMessage `json:"audio"`
	Chat          map[string]json.RawMessage `json:"chat"`
	Notifications map[string]json.RawMessage `json:"notifications"`
	Performance   map[string]json.RawMessage `json:"performance"`
	Recording     map[string]json.RawMessage `json:"recording"`
	Controller    map[string]json.RawMessage `json:"controller"`
	Windows       map[string]json.RawMessage `json:"windows"`
	Scripts       map[string]json.RawMessage `json:"scripts"`
	Updates       map[string]json.RawMessage `json:"updates"`
}

const (
	settingsGeneral       = "general"
	settingsControls      = "controls"
	settingsInterface     = "interface"
	settingsSpeechBubbles = "speech_bubbles"
	settingsRendering     = "rendering"
	settingsAudio         = "audio"
	settingsChat          = "chat"
	settingsNotifications = "notifications"
	settingsPerformance   = "performance"
	settingsRecording     = "recording"
	settingsController    = "controller"
	settingsWindows       = "windows"
	settingsScripts       = "scripts"
	settingsUpdates       = "updates"
)

type settingsSchemaEntry struct {
	field    string
	category string
	name     string
}

// settingsSchema is the single mapping between internal Go names and the
// readable v4 JSON names. Derived compatibility fields and runtime-only fields
// are intentionally not included.
var settingsSchema = []settingsSchemaEntry{
	{field: "SetupWizardVersion", category: settingsGeneral, name: "setup_wizard_version"},
	{field: "LastCharacter", category: settingsGeneral, name: "last_character"},
	{field: "ServerAddress", category: settingsGeneral, name: "server_address"},
	{field: "AltNetMode", category: settingsGeneral, name: "alternate_network_enabled"},
	{field: "AltNetDelay", category: settingsGeneral, name: "alternate_network_delay_ms"},

	{field: "ClickToToggle", category: settingsControls, name: "click_to_toggle"},
	{field: "MiddleClickMoveWindow", category: settingsControls, name: "middle_click_moves_window"},
	{field: "InputBarAlwaysOpen", category: settingsControls, name: "keep_input_bar_open"},
	{field: "KBWalkSpeed", category: settingsControls, name: "keyboard_walk_speed"},

	{field: "MainFontSize", category: settingsInterface, name: "main_font_size"},
	{field: "BubbleFontSize", category: settingsInterface, name: "speech_bubble_font_size"},
	{field: "ConsoleFontSize", category: settingsInterface, name: "messages_font_size"},
	{field: "ChatFontSize", category: settingsInterface, name: "chat_font_size"},
	{field: "InventoryFontSize", category: settingsInterface, name: "inventory_font_size"},
	{field: "PlayersFontSize", category: settingsInterface, name: "players_font_size"},
	{field: "ShowRecentPlayers", category: settingsInterface, name: "show_recent_players"},
	{field: "PlayerGroups", category: settingsInterface, name: "player_groups"},
	{field: "InventoryGroups", category: settingsInterface, name: "inventory_groups"},
	{field: "AlternateRowBackgrounds", category: settingsInterface, name: "alternate_row_backgrounds"},
	{field: "NameBgOpacity", category: settingsInterface, name: "name_background_opacity"},
	{field: "DarkBubblesAndNames", category: settingsInterface, name: "dark_mode_names_and_bubbles"},
	{field: "NameHealthBarModern", category: settingsInterface, name: "name_health_bar_modern"},
	{field: "NameHealthBarAbove", category: settingsInterface, name: "name_health_bar_above"},
	{field: "NameHealthBarThickness", category: settingsInterface, name: "name_health_bar_thickness"},
	{field: "NameTagLabelColors", category: settingsInterface, name: "colored_name_labels"},
	{field: "HideSelfNameTag", category: settingsInterface, name: "hide_own_name"},
	{field: "NameTagsOnHoverOnly", category: settingsInterface, name: "names_only_on_hover"},
	{field: "BarOpacity", category: settingsInterface, name: "status_bar_opacity"},
	{field: "BarColorByValue", category: settingsInterface, name: "status_bar_color_by_value"},
	{field: "ShowFPS", category: settingsInterface, name: "show_fps"},
	{field: "UIScale", category: settingsInterface, name: "ui_scale"},
	{field: "Fullscreen", category: settingsInterface, name: "fullscreen"},
	{field: "AlwaysOnTop", category: settingsInterface, name: "always_on_top"},
	{field: "Theme", category: settingsInterface, name: "color_theme"},
	{field: "Style", category: settingsInterface, name: "style_theme"},
	{field: "ShowClanLordSplashImage", category: settingsInterface, name: "show_clan_lord_splash"},
	{field: "WindowShadows", category: settingsInterface, name: "window_shadows"},

	{field: "SpeechBubbles", category: settingsSpeechBubbles, name: "enabled"},
	{field: "AnimatedChatBubbles", category: settingsSpeechBubbles, name: "animated"},
	{field: "BubbleOpacity", category: settingsSpeechBubbles, name: "opacity"},
	{field: "BubbleBaseLife", category: settingsSpeechBubbles, name: "base_lifetime_seconds"},
	{field: "BubbleLifePerWord", category: settingsSpeechBubbles, name: "seconds_per_word"},
	{field: "BubbleScale", category: settingsSpeechBubbles, name: "visual_scale"},
	{field: "BubbleNormal", category: settingsSpeechBubbles, name: "show_speech"},
	{field: "BubbleWhisper", category: settingsSpeechBubbles, name: "show_whispers"},
	{field: "BubbleYell", category: settingsSpeechBubbles, name: "show_yells"},
	{field: "BubbleThought", category: settingsSpeechBubbles, name: "show_thoughts"},
	{field: "BubbleRealAction", category: settingsSpeechBubbles, name: "show_real_actions"},
	{field: "BubbleMonster", category: settingsSpeechBubbles, name: "show_monster_speech"},
	{field: "BubblePlayerAction", category: settingsSpeechBubbles, name: "show_player_actions"},
	{field: "BubblePonder", category: settingsSpeechBubbles, name: "show_ponders"},
	{field: "BubbleNarrate", category: settingsSpeechBubbles, name: "show_narration"},
	{field: "BubbleSelf", category: settingsSpeechBubbles, name: "show_own_bubbles"},
	{field: "BubbleOtherPlayers", category: settingsSpeechBubbles, name: "show_other_players"},
	{field: "BubbleMonsters", category: settingsSpeechBubbles, name: "show_monsters"},
	{field: "BubbleNarration", category: settingsSpeechBubbles, name: "show_narrator"},

	{field: "MotionSmoothing", category: settingsRendering, name: "smooth_movement"},
	{field: "PixelArtScaling", category: settingsRendering, name: "pixel_perfect_scaling"},
	{field: "ObjectPinning", category: settingsRendering, name: "pin_world_objects"},
	{field: "BlendMobiles", category: settingsRendering, name: "blend_character_animation"},
	{field: "BlendPicts", category: settingsRendering, name: "blend_world_animation"},
	{field: "BlendAmount", category: settingsRendering, name: "world_animation_blend_amount"},
	{field: "MobileBlendAmount", category: settingsRendering, name: "character_animation_blend_amount"},
	{field: "MobileBlendFrames", category: settingsRendering, name: "character_animation_blend_frames"},
	{field: "PictBlendFrames", category: settingsRendering, name: "world_animation_blend_frames"},
	{field: "DenoiseImages", category: settingsRendering, name: "denoise_dithered_artwork"},
	{field: "DenoiseSharpness", category: settingsRendering, name: "denoise_sharpness"},
	{field: "DenoiseAmount", category: settingsRendering, name: "denoise_amount"},
	{field: "VSync", category: settingsRendering, name: "vertical_sync"},
	{field: "GameScale", category: settingsRendering, name: "artwork_scale"},
	{field: "SpriteGammaCorrection", category: settingsRendering, name: "artwork_gamma_correction"},
	{field: "SpriteGamma", category: settingsRendering, name: "artwork_source_gamma"},
	{field: "MonitorGamma", category: settingsRendering, name: "display_gamma"},
	{field: "ObscuringPictureOpacity", category: settingsRendering, name: "obscuring_artwork_opacity"},
	{field: "FadeObscuringPictures", category: settingsRendering, name: "fade_obscuring_artwork"},
	{field: "MaxNightLevel", category: settingsRendering, name: "maximum_night_darkness"},
	{field: "NightEffect", category: settingsRendering, name: "night_effect"},
	{field: "ShaderLighting", category: settingsRendering, name: "shader_lighting"},
	{field: "ShaderLightStrength", category: settingsRendering, name: "light_strength"},
	{field: "ShaderGlowStrength", category: settingsRendering, name: "glow_strength"},
	{field: "FlameLightFlicker", category: settingsRendering, name: "flame_light_flicker"},
	{field: "FlameFlickerStrength", category: settingsRendering, name: "flame_flicker_strength"},
	{field: "CharacterShadows", category: settingsRendering, name: "character_shadows"},
	{field: "CharacterShadowDarkness", category: settingsRendering, name: "character_shadow_darkness"},
	{field: "DetailedCharacterShadows", category: settingsRendering, name: "realistic_character_shadows"},
	{field: "MobilesReceiveSunShadows", category: settingsRendering, name: "mobiles_receive_sun_shadows"},

	{field: "MasterVolume", category: settingsAudio, name: "master_volume"},
	{field: "GameVolume", category: settingsAudio, name: "sound_volume"},
	{field: "MusicVolume", category: settingsAudio, name: "music_volume"},
	{field: "Music", category: settingsAudio, name: "music_enabled"},
	{field: "GameSound", category: settingsAudio, name: "sound_enabled"},
	{field: "Mute", category: settingsAudio, name: "muted"},
	{field: "MuteWhenUnfocused", category: settingsAudio, name: "mute_when_unfocused"},
	{field: "ThrottleSounds", category: settingsAudio, name: "limit_repeated_sounds"},
	{field: "SoundEnhancement", category: settingsAudio, name: "sound_enhancement"},
	{field: "SoundEnhancementAmount", category: settingsAudio, name: "sound_enhancement_amount"},
	{field: "MusicEnhancement", category: settingsAudio, name: "music_enhancement"},
	{field: "HighQualityResampling", category: settingsAudio, name: "high_quality_resampling"},

	{field: "MessagesToConsole", category: settingsChat, name: "copy_bubble_messages_to_messages_window"},
	{field: "MessageTextColors", category: settingsChat, name: "message_text_colors"},
	{field: "ChatTTS", category: settingsChat, name: "text_to_speech"},
	{field: "ChatTTSVolume", category: settingsChat, name: "text_to_speech_volume"},
	{field: "ChatTTSSpeed", category: settingsChat, name: "text_to_speech_speed"},
	{field: "ChatTTSVoice", category: settingsChat, name: "text_to_speech_voice"},
	{field: "ChatTTSBlocklist", category: settingsChat, name: "text_to_speech_blocklist"},
	{field: "ChatTimestamps", category: settingsChat, name: "chat_timestamps"},
	{field: "ConsoleTimestamps", category: settingsChat, name: "message_timestamps"},
	{field: "TimestampFormat", category: settingsChat, name: "timestamp_format"},

	{field: "Notifications", category: settingsNotifications, name: "enabled"},
	{field: "NotifyWhenBackground", category: settingsNotifications, name: "only_when_unfocused"},
	{field: "NotifyFallen", category: settingsNotifications, name: "fallen"},
	{field: "NotifyNotFallen", category: settingsNotifications, name: "recovered"},
	{field: "NotifyShares", category: settingsNotifications, name: "shares"},
	{field: "NotifyFriendOnline", category: settingsNotifications, name: "friend_online"},
	{field: "NotifyCopyText", category: settingsNotifications, name: "copy_text"},
	{field: "NotificationVolume", category: settingsNotifications, name: "volume"},
	{field: "NotificationBeep", category: settingsNotifications, name: "beep"},
	{field: "NotificationDuration", category: settingsNotifications, name: "duration_seconds"},

	{field: "PowerSaveBackground", category: settingsPerformance, name: "power_save_when_unfocused"},
	{field: "PowerSaveAlways", category: settingsPerformance, name: "always_power_save"},
	{field: "PowerSaveFPS", category: settingsPerformance, name: "power_save_fps"},
	{field: "PotatoGPU", category: settingsPerformance, name: "integrated_or_low_memory_gpu_mode"},
	{field: "PrecacheSounds", category: settingsPerformance, name: "precache_sounds"},
	{field: "PromptDisableShaders", category: settingsPerformance, name: "prompt_to_disable_slow_shaders"},

	{field: "PromptOnSaveRecording", category: settingsRecording, name: "prompt_when_saving"},
	{field: "AutoRecord", category: settingsRecording, name: "record_automatically"},

	{field: "JoystickEnabled", category: settingsController, name: "enabled"},
	{field: "JoystickBindings", category: settingsController, name: "button_bindings"},
	{field: "JoystickWalkStick", category: settingsController, name: "walk_stick"},
	{field: "JoystickCursorStick", category: settingsController, name: "cursor_stick"},
	{field: "JoystickWalkDeadzone", category: settingsController, name: "walk_deadzone"},
	{field: "JoystickCursorDeadzone", category: settingsController, name: "cursor_deadzone"},

	{field: "WindowWidth", category: settingsWindows, name: "application_width"},
	{field: "WindowHeight", category: settingsWindows, name: "application_height"},
	{field: "GameWindow", category: settingsWindows, name: "game"},
	{field: "InventoryWindow", category: settingsWindows, name: "inventory"},
	{field: "PlayersWindow", category: settingsWindows, name: "players"},
	{field: "MessagesWindow", category: settingsWindows, name: "messages"},
	{field: "ChatWindow", category: settingsWindows, name: "chat"},
	{field: "MovieWindow", category: settingsWindows, name: "movie"},
	{field: "ToolbarWindow", category: settingsWindows, name: "toolbar"},
	{field: "ToolbarPlacement", category: settingsWindows, name: "toolbar_placement"},
	{field: "ToolbarInfoBar", category: settingsWindows, name: "show_toolbar_info_bar"},
	{field: "AutoResizeWindows", category: settingsWindows, name: "auto_resize"},
	{field: "WindowSnapping", category: settingsWindows, name: "snapping"},

	{field: "Enabledscripts", category: settingsScripts, name: "enabled"},
	{field: "ScriptSpamKill", category: settingsScripts, name: "stop_spamming_scripts"},

	{field: "LastUpdateCheck", category: settingsUpdates, name: "last_check"},
	{field: "NotifiedVersion", category: settingsUpdates, name: "last_notified_version"},
}

func newSettingsDocument() settingsDocument {
	return settingsDocument{
		Version:       SETTINGS_VERSION,
		General:       make(map[string]json.RawMessage),
		Controls:      make(map[string]json.RawMessage),
		Interface:     make(map[string]json.RawMessage),
		SpeechBubbles: make(map[string]json.RawMessage),
		Rendering:     make(map[string]json.RawMessage),
		Audio:         make(map[string]json.RawMessage),
		Chat:          make(map[string]json.RawMessage),
		Notifications: make(map[string]json.RawMessage),
		Performance:   make(map[string]json.RawMessage),
		Recording:     make(map[string]json.RawMessage),
		Controller:    make(map[string]json.RawMessage),
		Windows:       make(map[string]json.RawMessage),
		Scripts:       make(map[string]json.RawMessage),
		Updates:       make(map[string]json.RawMessage),
	}
}

func (d *settingsDocument) category(name string) map[string]json.RawMessage {
	switch name {
	case settingsGeneral:
		return d.General
	case settingsControls:
		return d.Controls
	case settingsInterface:
		return d.Interface
	case settingsSpeechBubbles:
		return d.SpeechBubbles
	case settingsRendering:
		return d.Rendering
	case settingsAudio:
		return d.Audio
	case settingsChat:
		return d.Chat
	case settingsNotifications:
		return d.Notifications
	case settingsPerformance:
		return d.Performance
	case settingsRecording:
		return d.Recording
	case settingsController:
		return d.Controller
	case settingsWindows:
		return d.Windows
	case settingsScripts:
		return d.Scripts
	case settingsUpdates:
		return d.Updates
	default:
		return nil
	}
}

func settingsDocumentVersion(data []byte) (version int, modern bool, err error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return 0, false, err
	}
	if raw, ok := root["version"]; ok {
		if err := json.Unmarshal(raw, &version); err != nil {
			return 0, true, err
		}
		return version, true, nil
	}
	if raw, ok := root["Version"]; ok {
		if err := json.Unmarshal(raw, &version); err != nil {
			return 0, false, err
		}
		return version, false, nil
	}
	return 0, false, nil
}

func marshalSettingsDocument(s settings) ([]byte, error) {
	doc := newSettingsDocument()
	value := reflect.ValueOf(s)
	for _, entry := range settingsSchema {
		field := value.FieldByName(entry.field)
		if !field.IsValid() {
			return nil, fmt.Errorf("unknown settings field %q", entry.field)
		}
		raw, err := json.Marshal(field.Interface())
		if err != nil {
			return nil, fmt.Errorf("marshal setting %s.%s: %w", entry.category, entry.name, err)
		}
		doc.category(entry.category)[entry.name] = raw
	}
	if err := putSettingValue(doc.Rendering, "artwork_upscale_style", artworkUpscaleModeName(s.SpriteUpscaleMode)); err != nil {
		return nil, err
	}
	if err := putSettingValue(doc.Interface, "status_bar_placement", barPlacementName(s.BarPlacement)); err != nil {
		return nil, err
	}
	return json.MarshalIndent(doc, "", "  ")
}

func unmarshalSettingsDocument(data []byte, defaults settings) (settings, error) {
	doc := newSettingsDocument()
	if err := json.Unmarshal(data, &doc); err != nil {
		return settings{}, err
	}
	if doc.Version != SETTINGS_VERSION {
		return settings{}, fmt.Errorf("unsupported settings version %d", doc.Version)
	}
	result := defaults
	value := reflect.ValueOf(&result).Elem()
	for _, entry := range settingsSchema {
		raw, ok := doc.category(entry.category)[entry.name]
		if !ok && entry.field == "DarkBubblesAndNames" {
			raw, ok = doc.Interface["dark_bubbles_and_names"]
		}
		if !ok {
			continue
		}
		field := value.FieldByName(entry.field)
		if !field.IsValid() || !field.CanAddr() {
			return settings{}, fmt.Errorf("unknown settings field %q", entry.field)
		}
		if err := json.Unmarshal(raw, field.Addr().Interface()); err != nil {
			return settings{}, fmt.Errorf("read setting %s.%s: %w", entry.category, entry.name, err)
		}
	}
	if raw, ok := doc.Rendering["artwork_upscale_style"]; ok {
		var name string
		if err := json.Unmarshal(raw, &name); err != nil {
			return settings{}, fmt.Errorf("read setting rendering.artwork_upscale_style: %w", err)
		}
		result.SpriteUpscaleMode = parseArtworkUpscaleMode(name)
	}
	result.SpriteUpscaleFilter = result.SpriteUpscaleMode != artworkUpscaleOff
	result.SpriteUpscale = spriteUpscaleFactorFromScale(result.GameScale)
	if raw, ok := doc.Interface["status_bar_placement"]; ok {
		var name string
		if err := json.Unmarshal(raw, &name); err != nil {
			return settings{}, fmt.Errorf("read setting interface.status_bar_placement: %w", err)
		}
		result.BarPlacement = parseBarPlacement(name)
	}
	result.Version = SETTINGS_VERSION
	return result, nil
}

func putSettingValue(category map[string]json.RawMessage, name string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal setting %s: %w", name, err)
	}
	category[name] = raw
	return nil
}

func artworkUpscaleModeName(mode int) string {
	switch mode {
	case artworkUpscaleOff:
		return "off"
	case artworkUpscaleCrisp:
		return "crisp"
	case artworkUpscaleBalanced:
		return "balanced"
	case artworkUpscaleSmooth:
		return "smooth"
	case artworkUpscaleUltraSmooth:
		return "ultra_smooth"
	default:
		return "ultra_smooth"
	}
}

func parseArtworkUpscaleMode(name string) int {
	switch name {
	case "off":
		return artworkUpscaleOff
	case "crisp":
		return artworkUpscaleCrisp
	case "balanced":
		return artworkUpscaleBalanced
	case "smooth":
		return artworkUpscaleSmooth
	case "ultra_smooth":
		return artworkUpscaleUltraSmooth
	default:
		return artworkUpscaleUltraSmooth
	}
}

func barPlacementName(placement BarPlacement) string {
	switch placement {
	case BarPlacementLowerLeft:
		return "lower_left"
	case BarPlacementLowerRight:
		return "lower_right"
	case BarPlacementUpperRight:
		return "upper_right"
	default:
		return "bottom"
	}
}

func parseBarPlacement(name string) BarPlacement {
	switch name {
	case "lower_left":
		return BarPlacementLowerLeft
	case "lower_right":
		return BarPlacementLowerRight
	case "upper_right":
		return BarPlacementUpperRight
	default:
		return BarPlacementBottom
	}
}
