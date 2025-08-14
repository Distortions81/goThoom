package main

import (
	"bytes"
	"strings"
)

type thinkTarget int

const (
	thinkNone thinkTarget = iota
	thinkToYou
	thinkToClan
	thinkToGroup
)

// ThinkUnknownName is used when the sender's name can't be parsed.
const ThinkUnknownName = "someone"

const (
	bubbleVerbVerbatim    = "\x01"
	bubbleVerbParentheses = "\x02"
)

var bubbleLanguageNames = []string{
	"Halfling",
	"Sylvan",
	"People",
	"Thoom",
	"Dwarven",
	"Ghorak Zo",
	"Ancient",
	"Magic",
	"Common",
	"Thieves' Cant",
	"Mystic",
	"Monster",
	"unknown language",
	"Orga",
	"Sirrush",
	"Azcatl",
	"Lepori",
}

var languageWhisperVerb = []string{
	"squeaks softly",      // Halfling
	"chirps softly",       // Sylvan
	"purrs softly",        // People
	"hums softly",         // Thoom
	"mumbles",             // Dwarven
	"murmurs",             // Ghorak Zo
	"chants softly",       // Ancient
	"utters softly",       // Magic
	"whispers something",  // Common
	"gestures discreetly", // Thieves' Cant
	"incants softly",      // Mystic
	"growls softly",       // Monster
	"sounds softly",       // unknown language
	"grunts softly",       // Orga
	"hisses softly",       // Sirrush
	"clacks softly",       // Azcatl
	"nibbles softly",      // Lepori
}

var languageYellVerb = []string{
	"shouts",          // Halfling
	"calls",           // Sylvan
	"roars",           // People
	"trumpets",        // Thoom
	"hollers",         // Dwarven
	"bellows",         // Ghorak Zo
	"chants loudly",   // Ancient
	"utters loudly",   // Magic
	"yells something", // Common
	"gestures wildly", // Thieves' Cant
	"incants loudly",  // Mystic
	"growls loudly",   // Monster
	"shrieks",         // unknown language
	"grunts loudly",   // Orga
	"hisses loudly",   // Sirrush
	"rattles",         // Azcatl
	"yelps",           // Lepori
}

func decodeMacRoman(b []byte) string { return string(b) }

func decodeBEPP(data []byte) string {
	if len(data) < 3 || data[0] != 0xC2 {
		return ""
	}
	prefix := string(data[1:3])
	textBytes := data[3:]
	if i := bytes.IndexByte(textBytes, 0); i >= 0 {
		textBytes = textBytes[:i]
	}
	textBytes = stripBEPPTags(textBytes)
	text := strings.TrimSpace(decodeMacRoman(textBytes))
	if text == "" {
		return ""
	}

	switch prefix {
	case "th":
		if text != "" {
			return "think: " + text
		}
	case "in":
		if text != "" {
			return "info: " + text
		}
	case "sh":
		if text != "" {
			return "share: " + text
		}
	case "be":
		parseBackend(textBytes)
	case "yk":
		if text != "" {
			return text
		}
	case "iv":
		if text != "" {
			return text
		}
	case "hp":
		if text != "" {
			return text
		}
	}
	return ""
}

func stripBEPPTags(b []byte) []byte {
	out := b[:0]
	for i := 0; i < len(b); {
		c := b[i]
		if c == 0xC2 {
			if i+4 < len(b) && b[i+1] == 't' && b[i+2] == '_' && b[i+3] == 't' {
				switch b[i+4] {
				case 'h', 't', 'c', 'g':
					i += 5
					continue
				}
			}
			if i+2 < len(b) {
				i += 3
				continue
			}
			break
		}
		if c >= 0x80 || c < 0x20 {
			i++
			continue
		}
		out = append(out, c)
		i++
	}
	return out
}

func parseThinkText(raw []byte, text string) (name string, target thinkTarget, msg string) {
	idx := strings.IndexByte(text, ':')
	if idx >= 0 {
		name = strings.TrimSpace(text[:idx])
		msg = strings.TrimSpace(text[idx+1:])
	} else {
		name = ThinkUnknownName
		msg = strings.TrimSpace(text)
	}

	if i := bytes.Index(raw, []byte{0xC2, 't', '_', 't'}); i >= 0 && i+4 < len(raw) {
		switch raw[i+4] {
		case 't':
			target = thinkToYou
		case 'c':
			target = thinkToClan
		case 'g':
			target = thinkToGroup
		}
	}

	if target == thinkNone && name != "" && name != ThinkUnknownName {
		switch {
		case strings.HasSuffix(name, " to you"):
			target = thinkToYou
			name = strings.TrimSuffix(name, " to you")
		case strings.HasSuffix(name, " to your clan"):
			target = thinkToClan
			name = strings.TrimSuffix(name, " to your clan")
		case strings.HasSuffix(name, " to a group"):
			target = thinkToGroup
			name = strings.TrimSuffix(name, " to a group")
		}
		name = strings.TrimSpace(name)
	}
	return
}

func decodeBubble(data []byte) (verb, text, name, lang string, code uint8, target thinkTarget) {
	if len(data) < 2 {
		return "", "", "", "", kBubbleCodeKnown, thinkNone
	}
	typ := int(data[1])
	bubbleType := typ & kBubbleTypeMask
	p := 2
	code = kBubbleCodeKnown
	langIdx := -1
	if typ&kBubbleNotCommon != 0 {
		if len(data) < p+1 {
			return "", "", "", "", kBubbleCodeKnown, thinkNone
		}
		b := data[p]
		langIdx = int(b & kBubbleLanguageMask)
		if langIdx >= 0 && langIdx < len(bubbleLanguageNames) {
			lang = bubbleLanguageNames[langIdx]
		}
		code = b & kBubbleCodeMask
		p++
	}
	if typ&kBubbleFar != 0 {
		if len(data) < p+4 {
			return "", "", "", lang, code, thinkNone
		}
		p += 4
	}
	if len(data) <= p {
		return "", "", "", lang, code, thinkNone
	}
	raw := data[p:]
	msgData := stripBEPPTags(raw)
	if i := bytes.IndexByte(msgData, 0); i >= 0 {
		msgData = msgData[:i]
	}
	lines := bytes.Split(msgData, []byte{'\r'})
	for _, ln := range lines {
		if len(ln) == 0 {
			continue
		}
		s := strings.TrimSpace(decodeMacRoman(ln))
		if s == "" {
			continue
		}
		if parseNightCommand(s) {
			continue
		}
		if text == "" {
			text = s
		} else {
			text += " " + s
		}
	}
	if code != kBubbleCodeKnown && bubbleType != kBubbleYell {
		text = ""
	}
	if text == "" && code == kBubbleCodeKnown {
		return "", "", "", lang, code, thinkNone
	}
	switch bubbleType {
	case kBubbleNormal:
		verb = "says"
	case kBubbleWhisper:
		verb = "whispers"
		if typ&kBubbleNotCommon != 0 && langIdx >= 0 && langIdx < len(languageWhisperVerb) && langIdx != kBubbleCommon {
			verb = languageWhisperVerb[langIdx]
		}
	case kBubbleYell:
		verb = "yells"
		if typ&kBubbleNotCommon != 0 && langIdx >= 0 && langIdx < len(languageYellVerb) && langIdx != kBubbleCommon {
			verb = languageYellVerb[langIdx]
		}
	case kBubbleThought:
		verb = "thinks"
		name, target, text = parseThinkText(raw, text)
	case kBubbleRealAction:
		verb = bubbleVerbVerbatim
	case kBubbleMonster:
		verb = "growls"
	case kBubblePlayerAction:
		verb = bubbleVerbParentheses
	case kBubblePonder:
		verb = "ponders"
	case kBubbleNarrate:
		// narrate bubbles have no verb
	default:
		// unknown bubble types return no verb
	}
	return verb, text, name, lang, code, target
}

func decodeMessage(m []byte) string {
	if len(m) <= 16 {
		return ""
	}
	data := append([]byte(nil), m[16:]...)
	if len(data) > 0 && data[0] == 0xC2 {
		if s := decodeBEPP(data); s != "" {
			return s
		}
		return ""
	}
	if _, s, _, _, _, _ := decodeBubble(data); s != "" {
		return s
	}
	if i := bytes.IndexByte(data, 0); i >= 0 {
		data = data[:i]
	}
	if len(data) > 0 {
		txt := decodeMacRoman(data)
		if len([]rune(strings.TrimSpace(txt))) >= 4 {
			return txt
		}
	}

	simpleEncrypt(data)
	if len(data) > 0 && data[0] == 0xC2 {
		if s := decodeBEPP(data); s != "" {
			return s
		}
		return ""
	}
	if _, s, _, _, _, _ := decodeBubble(data); s != "" {
		return s
	}
	if i := bytes.IndexByte(data, 0); i >= 0 {
		data = data[:i]
	}
	if len(data) > 0 {
		txt := decodeMacRoman(data)
		if len([]rune(strings.TrimSpace(txt))) >= 4 {
			return txt
		}
	}
	return ""
}

func handleInfoText(data []byte) {
	for _, line := range bytes.Split(data, []byte{'\r'}) {
		if len(line) == 0 {
			continue
		}
		if line[0] == 0xC2 {
			if txt := decodeBEPP(line); txt != "" {
				consoleMessage(txt)
			}
			continue
		}
		if _, txt, _, _, _, _ := decodeBubble(line); txt != "" {
			chatMessage(txt)
			if gs.MessagesToConsole {
				consoleMessage(txt)
			}
			continue
		}
		s := strings.TrimSpace(decodeMacRoman(stripBEPPTags(line)))
		if s == "" {
			continue
		}
		if parseNightCommand(s) {
			continue
		}
		if strings.HasPrefix(s, "/") {
			continue
		}
		consoleMessage(s)
	}
}
