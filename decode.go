package main

import (
	"bytes"
	"log"
	"strings"

	"golang.org/x/text/encoding/charmap"
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

func decodeMacRoman(b []byte) string {
	s, err := charmap.Macintosh.NewDecoder().Bytes(b)
	if err != nil {
		return string(b)
	}
	return string(s)
}

/*
BEPP tag reference (two-letter codes following the 0xC2 prefix):

| Tag | Meaning             |
|-----|---------------------|
| ba  | bard message        |
| be  | back-end command    |
| cn  | clan name           |
| cf  | config              |
| dd  | do not display      |
| de  | demo notice         |
| dp  | depart              |
| dl  | download            |
| er  | error message       |
| gm  | game master         |
| hf  | has fallen          |
| nf  | no longer fallen    |
| hp  | help                |
| in  | info                |
| iv  | inventory           |
| ka  | karma               |
| kr  | karma received      |
| lf  | log off             |
| lg  | log on              |
| lo  | location            |
| ml  | multilingual        |
| mn  | monster name        |
| mu  | music               |
| nw  | news                |
| pn  | player name         |
| sh  | share               |
| su  | unshare             |
| tl  | text log only       |
| th  | think               |
| tt  | monospaced style    |
| wh  | who list            |
| yk  | you killed          |
*/
func decodeBEPP(data []byte) string {
	if len(data) < 3 || data[0] != 0xC2 {
		return ""
	}
	prefix := string(data[1:3])
	// Keep a raw copy (without NUL terminator) for backend parsing.
	raw := data[3:]
	if i := bytes.IndexByte(raw, 0); i >= 0 {
		raw = raw[:i]
	}
	// For displayable text, strip BEPP tags and non-printables.
	cleaned := stripBEPPTags(append([]byte(nil), raw...))
	text := strings.TrimSpace(decodeMacRoman(cleaned))

	if dumpBEPPTags {
		// Log the tag and a truncated form of the text for empirical analysis.
		t := text
		if len(t) > 120 {
			t = t[:120] + "..."
		}
		log.Printf("BEPP %s: %q", prefix, t)
	}
	if text == "" && prefix != "be" && prefix != "mu" { // backend commands or music may have no printable text
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
	case "sh", "su":
		parseShareText(raw, text)
		if text != "" {
			return text
		}
	case "hf", "nf":
		// Fallen or not-fallen notices
		parseFallenText(raw, text)
		if text != "" {
			return text
		}
	case "ba", "mu":
		// Bard guild messages or tunes
		handled := parseBardText(raw, text)
		if !handled && text != "" {
			return text
		}
	case "lg", "lf", "er":
		// Login/logout presence notices and error messages like
		// "<name> is not in the lands." which imply logoff
		parsePresenceText(raw, text)
		if text != "" {
			return text
		}
	case "be":
		// Back-end command: handle internally using raw (unstripped) data.
		parseBackend(raw)
		return ""
	case "kr":
		// Karma received: suppress notifications from blocked or ignored players.
		name := utfFold(firstTagContent(raw, 'p', 'n'))
		if name != "" {
			playersMu.RLock()
			p, ok := players[name]
			blocked := ok && (p.Blocked || p.Ignored)
			playersMu.RUnlock()
			if blocked {
				return ""
			}
		}
		if text != "" {
			return text
		}
	case "yk", "iv", "hp", "cf", "pn", "ka", "tl":
		// Known simple pass-through prefixes (e.g., iv: item/verb,
		// ka: karma, tl: text log only)
		if text != "" {
			return text
		}
	}
	if text != "" {
		logDebug("unknown BEPP prefix %q: %q", prefix, text)
		return text
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
		// Preserve MacRoman high-bit printable characters so decodeMacRoman
		// can convert them (e.g., curly apostrophes). Only drop ASCII control
		// characters here.
		if c < 0x20 {
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

func decodeBubble(data []byte) (verb, text, name, lang string, code uint8, bubbleType int, target thinkTarget) {
	if len(data) < 2 {
		return "", "", "", "", kBubbleCodeKnown, kBubbleNormal, thinkNone
	}
	typ := int(data[1])
	bubbleType = typ & kBubbleTypeMask
	p := 2
	code = kBubbleCodeKnown
	langIdx := -1
	if typ&kBubbleNotCommon != 0 {
		if len(data) < p+1 {
			return "", "", "", "", kBubbleCodeKnown, bubbleType, thinkNone
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
			return "", "", "", lang, code, bubbleType, thinkNone
		}
		p += 4
	}
	if len(data) <= p {
		return "", "", "", lang, code, bubbleType, thinkNone
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
        if parseInterruptCommand(s) {
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
		return "", "", "", lang, code, bubbleType, thinkNone
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
	return verb, text, name, lang, code, bubbleType, target
}

// decodeMessage extracts printable text from a raw server message. It operates
// directly on m[16:], which may be modified during decoding (e.g., when
// decrypting).
func decodeMessage(m []byte) string {
	if len(m) <= 16 {
		return ""
	}
	data := m[16:]
	for attempt := 0; attempt < 2; attempt++ {
		if len(data) > 0 && data[0] == 0xC2 {
			if s := decodeBEPP(data); s != "" {
				return s
			}
			return ""
		}
		if _, s, _, _, _, _, _ := decodeBubble(data); s != "" {
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
		if attempt == 0 {
			simpleEncrypt(data)
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
		if _, txt, _, _, _, bubbleType, _ := decodeBubble(line); txt != "" {
			if isChatBubble(bubbleType) {
				if gs.MessagesToConsole {
					consoleMessage(txt)
				} else {
					chatMessage(txt)
				}
			} else {
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
        if parseInterruptCommand(s) {
            continue
        }
        // Empirical: classic client handles server-sent info-text music commands.
        // Be permissive here as servers can vary:
        // - Accept explicit "/music/..." payloads anywhere in the line
        // - Accept leading "play ..." or "play/..." forms
        if strings.Contains(s, "/music/") || strings.HasPrefix(s, "play ") || strings.HasPrefix(s, "play/") {
            if parseMusicCommand(s, line) {
                continue
            }
        }
		// Ignore other command-like lines.
		if strings.HasPrefix(s, "/") {
			continue
		}
		consoleMessage(s)
	}
}
