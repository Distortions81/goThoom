package main

import (
	"sort"
	"strings"
	"unicode/utf8"
)

var emojiShortcodeReplacer = newEmojiShortcodeReplacer()

var emojiStarts = func() map[rune]bool {
	starts := make(map[rune]bool)
	for _, emoji := range emojiByName {
		r, _ := utf8.DecodeRuneInString(emoji)
		starts[r] = true
	}
	return starts
}()

func containsEmoji(input string) bool {
	for _, r := range input {
		if r >= 0x80 && emojiStarts[r] {
			return true
		}
	}
	return false
}

func newEmojiShortcodeReplacer() *strings.Replacer {
	// Pick a stable, short alias when several names share the same emoji.
	names := make(map[string]string)
	for name, emoji := range emojiByName {
		name = strings.ReplaceAll(name, " ", "_")
		previous, exists := names[emoji]
		if !exists || len(name) < len(previous) || len(name) == len(previous) && name < previous {
			names[emoji] = name
		}
	}
	emojis := make([]string, 0, len(names))
	for emoji := range names {
		emojis = append(emojis, emoji)
	}
	// Match complete sequences before their component emoji (e.g. a profession
	// before its person glyph, or a heart with its presentation selector).
	sort.Slice(emojis, func(i, j int) bool {
		if len(emojis[i]) != len(emojis[j]) {
			return len(emojis[i]) > len(emojis[j])
		}
		return emojis[i] < emojis[j]
	})
	pairs := make([]string, 0, len(emojis)*2)
	for _, emoji := range emojis {
		pairs = append(pairs, emoji, ":"+names[emoji]+":")
	}
	return strings.NewReplacer(pairs...)
}

// encodeEmojiShortcodes preserves typed names and converts pasted emoji into
// readable names for clients that do not display Unicode emoji.
func encodeEmojiShortcodes(input string) string {
	return emojiShortcodeReplacer.Replace(input)
}

func displayEmojiText(input string) string {
	if !gs.ExpandEmojiNames {
		return input
	}
	return expandEmojiShortcodes(input)
}

// expandEmojiShortcodes accepts GoMUD2's space-separated names and the usual
// underscore spelling. Only complete, recognized :name: tokens are replaced.
func expandEmojiShortcodes(input string) string {
	var output strings.Builder
	written := 0
	for start := 0; start < len(input); {
		i := strings.IndexByte(input[start:], ':')
		if i < 0 {
			break
		}
		start += i
		end := strings.IndexByte(input[start+1:], ':')
		if end < 0 {
			break
		}
		end += start + 1
		name := strings.ToLower(strings.ReplaceAll(input[start+1:end], "_", " "))
		if emoji, ok := emojiByName[name]; ok {
			output.WriteString(input[written:start])
			output.WriteString(emoji)
			written = end + 1
			start = written
		} else {
			// This colon may also open the next token, e.g. :unknown::smile:.
			start = end
		}
	}
	if written == 0 {
		return input
	}
	output.WriteString(input[written:])
	return output.String()
}
