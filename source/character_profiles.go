package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"gothoom/eui"
)

const (
	characterProfilesFile    = "profiles.json"
	characterProfilesVersion = 1
)

type characterProfile struct {
	Name          string                     `json:"name"`
	Interface     map[string]json.RawMessage `json:"interface,omitempty"`
	SpeechBubbles map[string]json.RawMessage `json:"speech_bubbles,omitempty"`
	Rendering     map[string]json.RawMessage `json:"rendering,omitempty"`
	Audio         map[string]json.RawMessage `json:"audio,omitempty"`
	Chat          map[string]json.RawMessage `json:"chat,omitempty"`
	Notifications map[string]json.RawMessage `json:"notifications,omitempty"`
	Windows       map[string]json.RawMessage `json:"windows,omitempty"`
	Scripts       map[string]json.RawMessage `json:"scripts,omitempty"`
}

type characterProfilesDocument struct {
	Version  int                         `json:"version"`
	Enabled  map[string]bool             `json:"enabled,omitempty"`
	Profiles map[string]characterProfile `json:"profiles,omitempty"`
}

var (
	globalSettingsBase      settings
	globalSettingsBaseReady bool
	activeCharacterProfile  string
	characterProfiles       = characterProfilesDocument{Version: characterProfilesVersion}
)

func characterProfileKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func characterProfilesPath() string {
	return filepath.Join(dataDirPath, characterProfilesFile)
}

func characterProfileEnabled(name string) bool {
	return characterProfiles.Enabled[characterProfileKey(name)]
}

func (profile *characterProfile) category(name string) map[string]json.RawMessage {
	switch name {
	case settingsInterface:
		return profile.Interface
	case settingsSpeechBubbles:
		return profile.SpeechBubbles
	case settingsRendering:
		return profile.Rendering
	case settingsAudio:
		return profile.Audio
	case settingsChat:
		return profile.Chat
	case settingsNotifications:
		return profile.Notifications
	case settingsWindows:
		return profile.Windows
	case settingsScripts:
		return profile.Scripts
	default:
		return nil
	}
}

func (profile *characterProfile) ensureCategory(name string) map[string]json.RawMessage {
	if category := profile.category(name); category != nil {
		return category
	}
	category := make(map[string]json.RawMessage)
	switch name {
	case settingsInterface:
		profile.Interface = category
	case settingsSpeechBubbles:
		profile.SpeechBubbles = category
	case settingsRendering:
		profile.Rendering = category
	case settingsAudio:
		profile.Audio = category
	case settingsChat:
		profile.Chat = category
	case settingsNotifications:
		profile.Notifications = category
	case settingsWindows:
		profile.Windows = category
	case settingsScripts:
		profile.Scripts = category
	default:
		return nil
	}
	return category
}

func characterProfileSetting(entry settingsSchemaEntry) bool {
	switch entry.category {
	case settingsInterface, settingsSpeechBubbles, settingsRendering, settingsAudio,
		settingsNotifications, settingsWindows, settingsScripts:
		return true
	}
	switch entry.field {
	case "MessagesToConsole", "MessageTextColors", "ChatTTS", "ChatTTSVolume", "ChatTTSSpeed", "ChatTTSVoice":
		return true
	default:
		return false
	}
}

func cloneSettings(value settings) settings {
	data, err := marshalSettingsDocument(value)
	if err != nil {
		return value
	}
	cloned, err := unmarshalSettingsDocument(data, value)
	if err != nil {
		return value
	}
	return cloned
}

func captureCharacterProfile(name string, value settings) (characterProfile, error) {
	profile := characterProfile{Name: strings.TrimSpace(name)}
	settingsValue := reflect.ValueOf(value)
	for _, entry := range settingsSchema {
		if !characterProfileSetting(entry) {
			continue
		}
		field := settingsValue.FieldByName(entry.field)
		if !field.IsValid() {
			return characterProfile{}, fmt.Errorf("unknown profile setting field %q", entry.field)
		}
		raw, err := json.Marshal(field.Interface())
		if err != nil {
			return characterProfile{}, fmt.Errorf("encode profile setting %s.%s: %w", entry.category, entry.name, err)
		}
		profile.ensureCategory(entry.category)[entry.name] = raw
	}
	if raw, err := json.Marshal(artworkUpscaleModeName(value.SpriteUpscaleMode)); err == nil {
		profile.ensureCategory(settingsRendering)["artwork_upscale_style"] = raw
	}
	if raw, err := json.Marshal(barPlacementName(value.BarPlacement)); err == nil {
		profile.ensureCategory(settingsInterface)["status_bar_placement"] = raw
	}
	return profile, nil
}

func applyCharacterProfile(base settings, profile characterProfile) (settings, error) {
	result := cloneSettings(base)
	settingsValue := reflect.ValueOf(&result).Elem()
	for _, entry := range settingsSchema {
		if !characterProfileSetting(entry) {
			continue
		}
		raw, ok := profile.category(entry.category)[entry.name]
		if !ok {
			continue
		}
		field := settingsValue.FieldByName(entry.field)
		if !field.IsValid() || !field.CanAddr() {
			return base, fmt.Errorf("unknown profile setting field %q", entry.field)
		}
		if err := json.Unmarshal(raw, field.Addr().Interface()); err != nil {
			return base, fmt.Errorf("read profile setting %s.%s: %w", entry.category, entry.name, err)
		}
	}
	if raw, ok := profile.Rendering["artwork_upscale_style"]; ok {
		var name string
		if err := json.Unmarshal(raw, &name); err != nil {
			return base, fmt.Errorf("read profile rendering.artwork_upscale_style: %w", err)
		}
		result.SpriteUpscaleMode = parseArtworkUpscaleMode(name)
		result.SpriteUpscaleFilter = result.SpriteUpscaleMode != artworkUpscaleOff
	}
	if raw, ok := profile.Interface["status_bar_placement"]; ok {
		var name string
		if err := json.Unmarshal(raw, &name); err != nil {
			return base, fmt.Errorf("read profile interface.status_bar_placement: %w", err)
		}
		result.BarPlacement = parseBarPlacement(name)
	}
	result.SpriteUpscale = spriteUpscaleFactorFromScale(result.GameScale)
	clampWindowSettingsValue(&result)
	return result, nil
}

func clampWindowSettingsValue(value *settings) {
	if value == nil {
		return
	}
	states := []*WindowState{
		&value.GameWindow, &value.InventoryWindow, &value.PlayersWindow, &value.MessagesWindow,
		&value.ChatWindow, &value.MovieWindow, &value.ToolbarWindow,
	}
	for _, state := range states {
		clampWindowState(state)
	}
}

func loadCharacterProfilesDocument() {
	characterProfiles = characterProfilesDocument{Version: characterProfilesVersion}
	if isWASM {
		return
	}
	data, err := os.ReadFile(characterProfilesPath())
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		log.Printf("read character profiles: %v", err)
		return
	}
	var document characterProfilesDocument
	if err := json.Unmarshal(data, &document); err != nil {
		log.Printf("parse character profiles: %v", err)
		return
	}
	if document.Version != characterProfilesVersion {
		log.Printf("character profiles: unsupported version %d", document.Version)
		return
	}
	if document.Profiles == nil {
		document.Profiles = make(map[string]characterProfile)
	}
	normalizedEnabled := make(map[string]bool, len(document.Enabled))
	for key, enabled := range document.Enabled {
		if normalizedKey := characterProfileKey(key); enabled && normalizedKey != "" {
			normalizedEnabled[normalizedKey] = true
		}
	}
	document.Enabled = normalizedEnabled
	normalized := make(map[string]characterProfile, len(document.Profiles))
	for key, profile := range document.Profiles {
		if strings.TrimSpace(profile.Name) == "" {
			profile.Name = strings.TrimSpace(key)
		}
		if normalizedKey := characterProfileKey(profile.Name); normalizedKey != "" {
			normalized[normalizedKey] = profile
		}
	}
	document.Profiles = normalized
	characterProfiles = document
}

func saveCharacterProfilesDocument() error {
	if isWASM {
		return nil
	}
	characterProfiles.Version = characterProfilesVersion
	if characterProfiles.Profiles == nil {
		characterProfiles.Profiles = make(map[string]characterProfile)
	}
	data, err := json.MarshalIndent(characterProfiles, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dataDirPath, 0o755); err != nil {
		return err
	}
	return legacyMacroAtomicWriteFile(characterProfilesPath(), append(data, '\n'), 0o644)
}

func initializeCharacterProfiles(value settings) settings {
	globalSettingsBase = cloneSettings(value)
	globalSettingsBaseReady = true
	loadCharacterProfilesDocument()
	activeCharacterProfile = ""
	if !characterProfileEnabled(value.LastCharacter) {
		return value
	}
	activeCharacterProfile = characterProfileKey(value.LastCharacter)
	if activeCharacterProfile == "" {
		return value
	}
	profile, ok := characterProfiles.Profiles[activeCharacterProfile]
	if !ok {
		return value
	}
	profiled, err := applyCharacterProfile(globalSettingsBase, profile)
	if err != nil {
		log.Printf("apply character profile %q: %v", profile.Name, err)
		return value
	}
	profiled.LastCharacter = value.LastCharacter
	return profiled
}

func copyGlobalSettingsFromActive() {
	if !globalSettingsBaseReady {
		globalSettingsBase = cloneSettings(gs)
		globalSettingsBaseReady = true
		return
	}
	destination := reflect.ValueOf(&globalSettingsBase).Elem()
	source := reflect.ValueOf(gs)
	for _, entry := range settingsSchema {
		if characterProfileSetting(entry) {
			continue
		}
		destination.FieldByName(entry.field).Set(source.FieldByName(entry.field))
	}
}

func settingsForSave() settings {
	currentProfile := ""
	if characterProfileEnabled(gs.LastCharacter) {
		currentProfile = characterProfileKey(gs.LastCharacter)
	}
	if activeCharacterProfile != currentProfile {
		// Direct settings replacement is common in tests and recovery paths.
		// Treat that state as a fresh baseline instead of writing it into a
		// previously active character's profile.
		activeCharacterProfile = currentProfile
		globalSettingsBase = cloneSettings(gs)
		globalSettingsBaseReady = true
		return gs
	}
	if activeCharacterProfile == "" {
		globalSettingsBase = cloneSettings(gs)
		globalSettingsBaseReady = true
		return gs
	}
	copyGlobalSettingsFromActive()
	profile, err := captureCharacterProfile(gs.LastCharacter, gs)
	if err != nil {
		log.Printf("capture character profile: %v", err)
	} else {
		if characterProfiles.Profiles == nil {
			characterProfiles.Profiles = make(map[string]characterProfile)
		}
		characterProfiles.Profiles[activeCharacterProfile] = profile
		if err := saveCharacterProfilesDocument(); err != nil {
			log.Printf("save character profiles: %v", err)
		}
	}
	globalSettingsBase.LastCharacter = gs.LastCharacter
	return globalSettingsBase
}

func switchCharacterProfile(character string) {
	character = strings.TrimSpace(character)
	nextKey := ""
	if characterProfileEnabled(character) {
		nextKey = characterProfileKey(character)
	}
	if nextKey == activeCharacterProfile {
		gs.LastCharacter = character
		if globalSettingsBaseReady {
			globalSettingsBase.LastCharacter = character
		}
		saveSettings()
		return
	}
	if uiReady {
		syncWindowSettings()
	}
	saveSettings()
	if !globalSettingsBaseReady {
		globalSettingsBase = cloneSettings(gs)
		globalSettingsBaseReady = true
	}
	globalSettingsBase.LastCharacter = character
	applySelectedCharacterSettings(character)
}

func applySelectedCharacterSettings(character string) {
	character = strings.TrimSpace(character)
	globalSettingsBase.LastCharacter = character
	next := cloneSettings(globalSettingsBase)
	nextKey := ""
	if characterProfileEnabled(character) {
		nextKey = characterProfileKey(character)
	}
	if nextKey != "" {
		if profile, ok := characterProfiles.Profiles[nextKey]; ok {
			profiled, err := applyCharacterProfile(next, profile)
			if err != nil {
				log.Printf("apply character profile %q: %v", character, err)
			} else {
				next = profiled
			}
		}
	}
	next.LastCharacter = character
	gs = next
	activeCharacterProfile = nextKey
	if uiReady {
		applyCharacterProfileRuntime()
	}
	settingsDirty = true
	saveSettings()
}

func setCharacterProfileEnabled(character string, enabled bool) {
	character = strings.TrimSpace(character)
	key := characterProfileKey(character)
	if key == "" || characterProfileEnabled(character) == enabled {
		return
	}

	selected := strings.EqualFold(strings.TrimSpace(gs.LastCharacter), character)
	if selected {
		if uiReady {
			syncWindowSettings()
		}
		// Preserve the currently active global or character settings before
		// changing which storage owns subsequent edits.
		saveSettings()
	}

	if characterProfiles.Enabled == nil {
		characterProfiles.Enabled = make(map[string]bool)
	}
	if enabled {
		characterProfiles.Enabled[key] = true
		if _, exists := characterProfiles.Profiles[key]; !exists && selected {
			profile, err := captureCharacterProfile(character, gs)
			if err != nil {
				log.Printf("create character profile %q: %v", character, err)
			} else {
				if characterProfiles.Profiles == nil {
					characterProfiles.Profiles = make(map[string]characterProfile)
				}
				characterProfiles.Profiles[key] = profile
			}
		}
	} else {
		delete(characterProfiles.Enabled, key)
	}
	if err := saveCharacterProfilesDocument(); err != nil {
		log.Printf("save character profile selection: %v", err)
	}

	if selected {
		applySelectedCharacterSettings(character)
	}
}

func applyCharacterProfileRuntime() {
	if gs.Theme != "" {
		if err := eui.LoadTheme(gs.Theme); err != nil {
			log.Printf("load profile theme %q: %v", gs.Theme, err)
		}
	}
	if gs.Style != "" {
		if err := eui.LoadStyle(gs.Style); err != nil {
			log.Printf("load profile style %q: %v", gs.Style, err)
		}
	}
	eui.SetUserUIScale(float32(gs.UIScale))
	setArtworkUpscaleMode(gs.SpriteUpscaleMode)
	setHighQualityResamplingEnabled(gs.HighQualityResampling)
	applySettings()
	updateGameWindowSize()
	placeToolbar(gs.ToolbarPlacement, false)
	applyManagedWindowLayout()
	updateInventoryWindow()
	updatePlayersWindow()
	updateConsoleWindow()
	updateChatWindow()
	updateDimmedScreenBG()
	applyEnabledScripts()
	refreshscriptsWindow()
	rebuildConfigurationWindows()
}
