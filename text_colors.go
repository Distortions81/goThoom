package main

import "gothoom/eui"

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
	Type  string
	Label string
	Color eui.Color
}

var messageTextColorOptions = []messageTextColorOption{
	{messageTextTypeSay, "Say", eui.NewColor(255, 255, 255, 255)},
	{messageTextTypeWhisper, "Whisper", eui.NewColor(180, 220, 255, 255)},
	{messageTextTypeYell, "Yell", eui.NewColor(255, 220, 120, 255)},
	{messageTextTypeThink, "Think", eui.NewColor(220, 180, 255, 255)},
	{messageTextTypeAction, "Action", eui.NewColor(160, 255, 180, 255)},
	{messageTextTypePonder, "Ponder", eui.NewColor(180, 255, 230, 255)},
	{messageTextTypeNarration, "Narration", eui.NewColor(255, 190, 220, 255)},
	{messageTextTypeMonster, "Monster", eui.NewColor(255, 170, 170, 255)},
	{messageTextTypeSystem, "System", eui.NewColor(210, 210, 210, 255)},
}

func defaultMessageTextColors() map[string]eui.Color {
	colors := make(map[string]eui.Color, len(messageTextColorOptions))
	for _, option := range messageTextColorOptions {
		colors[option.Type] = option.Color
	}
	return colors
}

func ensureMessageTextColors() {
	defaults := defaultMessageTextColors()
	if gs.MessageTextColors == nil {
		gs.MessageTextColors = defaults
		return
	}
	for messageType, color := range defaults {
		if _, ok := gs.MessageTextColors[messageType]; !ok {
			gs.MessageTextColors[messageType] = color
		}
	}
}

func messageTextColor(messageType string) eui.Color {
	ensureMessageTextColors()
	if color, ok := gs.MessageTextColors[messageType]; ok {
		return color
	}
	return gs.MessageTextColors[messageTextTypeSystem]
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
	updateConsoleWindow()
	updateChatWindow()
}
