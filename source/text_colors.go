package main

import (
	"strings"

	"gothoom/eui"
)

const (
	messageTextTypeSay       = "say"
	messageTextTypeWhisper   = "whisper"
	messageTextTypeYell      = "yell"
	messageTextTypeThink     = "think"
	messageTextTypeAction    = "action"
	messageTextTypePonder    = "ponder"
	messageTextTypeNarration = "narration"
	messageTextTypeMonster   = "monster"
	messageTextTypeSystem    = "system"
)

type messageTextColorOption struct {
	Type       string
	Label      string
	DarkColor  eui.Color
	LightColor eui.Color
}

var messageTextColorOptions = []messageTextColorOption{
	{messageTextTypeSay, "Say", eui.NewColor(255, 145, 145, 255), eui.NewColor(170, 0, 0, 255)},
	{messageTextTypeWhisper, "Whisper", eui.NewColor(180, 220, 255, 255), eui.NewColor(0, 70, 155, 255)},
	{messageTextTypeYell, "Yell", eui.NewColor(255, 220, 120, 255), eui.NewColor(135, 90, 0, 255)},
	{messageTextTypeThink, "Think", eui.NewColor(220, 180, 255, 255), eui.NewColor(100, 35, 145, 255)},
	{messageTextTypeAction, "Action", eui.NewColor(160, 255, 180, 255), eui.NewColor(0, 105, 40, 255)},
	{messageTextTypePonder, "Ponder", eui.NewColor(180, 255, 230, 255), eui.NewColor(0, 105, 90, 255)},
	{messageTextTypeNarration, "Narration", eui.NewColor(255, 190, 220, 255), eui.NewColor(145, 30, 90, 255)},
	{messageTextTypeMonster, "Monster", eui.NewColor(255, 170, 170, 255), eui.NewColor(145, 45, 30, 255)},
	{messageTextTypeSystem, "Default text", eui.NewColor(210, 210, 210, 255), eui.NewColor(25, 25, 25, 255)},
}

func defaultMessageTextColors() map[string]eui.Color {
	return defaultMessageTextColorsForPalette(false)
}

func defaultLightMessageTextColors() map[string]eui.Color {
	return defaultMessageTextColorsForPalette(true)
}

func defaultMessageTextColorsForPalette(light bool) map[string]eui.Color {
	colors := make(map[string]eui.Color, len(messageTextColorOptions))
	for _, option := range messageTextColorOptions {
		color := option.DarkColor
		if light {
			color = option.LightColor
		}
		colors[option.Type] = color
	}
	return colors
}

func legacyMessageTextColors() map[string]eui.Color {
	return map[string]eui.Color{
		messageTextTypeSay:       eui.NewColor(255, 255, 255, 255),
		messageTextTypeWhisper:   eui.NewColor(180, 220, 255, 255),
		messageTextTypeYell:      eui.NewColor(255, 220, 120, 255),
		messageTextTypeThink:     eui.NewColor(220, 180, 255, 255),
		messageTextTypeAction:    eui.NewColor(160, 255, 180, 255),
		messageTextTypePonder:    eui.NewColor(180, 255, 230, 255),
		messageTextTypeNarration: eui.NewColor(255, 190, 220, 255),
		messageTextTypeMonster:   eui.NewColor(255, 170, 170, 255),
		messageTextTypeSystem:    eui.NewColor(210, 210, 210, 255),
	}
}

func messageTextColorsEqual(a, b map[string]eui.Color) bool {
	if len(a) != len(b) {
		return false
	}
	for messageType, color := range a {
		if b[messageType] != color {
			return false
		}
	}
	return true
}

func ensureMessageTextColors() {
	darkDefaults := defaultMessageTextColors()
	lightDefaults := defaultLightMessageTextColors()
	if gs.MessageTextColors == nil {
		gs.MessageTextColors = darkDefaults
	} else if gs.MessageTextColorsLight == nil && messageTextColorsEqual(gs.MessageTextColors, legacyMessageTextColors()) {
		// Upgrade the original single palette, whose Say default was white.
		gs.MessageTextColors = darkDefaults
	}
	if gs.MessageTextColorsLight == nil {
		gs.MessageTextColorsLight = lightDefaults
	}
	for messageType, color := range darkDefaults {
		if _, ok := gs.MessageTextColors[messageType]; !ok {
			gs.MessageTextColors[messageType] = color
		}
	}
	for messageType, color := range lightDefaults {
		if _, ok := gs.MessageTextColorsLight[messageType]; !ok {
			gs.MessageTextColorsLight[messageType] = color
		}
	}
}

func messageTextPalette(light bool) map[string]eui.Color {
	ensureMessageTextColors()
	if light {
		return gs.MessageTextColorsLight
	}
	return gs.MessageTextColors
}

func messageTextColorForPalette(messageType string, light bool) eui.Color {
	colors := messageTextPalette(light)
	if color, ok := colors[messageType]; ok {
		return color
	}
	return colors[messageTextTypeSystem]
}

func messageTextStyle(messageType string) (eui.Color, bool) {
	if messageType == "" {
		messageType = messageTextTypeSystem
	}
	if messageType == messageTextTypeSystem && !gs.OverrideThemeTextColor {
		return eui.TextColor(), false
	}
	return messageTextColorForPalette(messageType, eui.IsLightTheme()), true
}

// classicMessageTextColor mirrors the bundled defaults in the classic
// client's TextStyles.plist. Classic text-window colors distinguish ordinary,
// labeled-friend, self, and private think-to speech rather than bubble verbs.
func classicMessageTextColor(messageType, message string) eui.Color {
	black := eui.NewColor(0, 0, 0, 255)
	if messageType == messageTextTypeThink && strings.Contains(strings.ToLower(message), " thinks to you,") {
		return eui.NewColor(44, 102, 0, 255)
	}
	speaker := chatSpeaker(message)
	if speaker == "" {
		return black
	}
	if isLocalPlayerName(speaker) {
		return eui.NewColor(80, 0, 51, 255)
	}
	playersMu.RLock()
	player := players[speaker]
	friend := player != nil && player.FriendLabel >= 1 && player.FriendLabel <= 5
	playersMu.RUnlock()
	if friend {
		return eui.NewColor(0, 80, 0, 255)
	}
	return black
}

func messageTextStyleForMessage(messageType, message string) (eui.Color, bool) {
	if gs.ClassicMessageColors {
		return classicMessageTextColor(messageType, message), true
	}
	return messageTextStyle(messageType)
}

func messageTextColor(messageType string) eui.Color {
	color, _ := messageTextStyle(messageType)
	return color
}

func messageTextTypeForBubble(bubbleType int) string {
	switch bubbleType {
	case kBubbleNormal:
		return messageTextTypeSay
	case kBubbleWhisper:
		return messageTextTypeWhisper
	case kBubbleYell:
		return messageTextTypeYell
	case kBubbleThought:
		return messageTextTypeThink
	case kBubbleRealAction, kBubblePlayerAction:
		return messageTextTypeAction
	case kBubblePonder:
		return messageTextTypePonder
	case kBubbleNarrate:
		return messageTextTypeNarration
	case kBubbleMonster:
		return messageTextTypeMonster
	default:
		return messageTextTypeSystem
	}
}

func refreshMessageTextWindows() {
	consoleWindowForceFull = true
	chatWindowForceFull = true
	updateConsoleWindow()
	updateChatWindow()
}
