package main

import (
	"bufio"
	_ "embed"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Unicode Emoji 17.0, https://www.unicode.org/Public/17.0.0/emoji/emoji-test.txt
// See licenses/Unicode.txt.
//
//go:embed data/emoji/emoji-test.txt
var unicodeEmojiData string

type emojiEntry struct{ Emoji, Name, Group, Subgroup string }

var emojiCatalog = func() []emojiEntry {
	entries, err := parseEmojiCatalog(unicodeEmojiData)
	if err != nil {
		panic(err)
	}
	return entries
}()

func parseEmojiCatalog(data string) ([]emojiEntry, error) {
	var entries []emojiEntry
	var group, subgroup string
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if value, ok := strings.CutPrefix(line, "# group: "); ok {
			group = value
			continue
		}
		if value, ok := strings.CutPrefix(line, "# subgroup: "); ok {
			subgroup = value
			continue
		}
		code, tail, ok := strings.Cut(line, ";")
		if !ok {
			continue
		}
		status, comment, ok := strings.Cut(tail, "#")
		if !ok || strings.TrimSpace(status) != "fully-qualified" {
			continue
		}
		fields := strings.Fields(comment)
		if group == "" || subgroup == "" || len(fields) < 3 {
			return nil, fmt.Errorf("invalid emoji entry: %s", line)
		}
		var emoji strings.Builder
		for _, point := range strings.Fields(code) {
			r, err := strconv.ParseUint(point, 16, 32)
			if err != nil || r > unicode.MaxRune || r >= 0xd800 && r <= 0xdfff {
				return nil, fmt.Errorf("invalid emoji code point: %s", point)
			}
			emoji.WriteRune(rune(r))
		}
		entries = append(entries, emojiEntry{emoji.String(), strings.Join(fields[2:], " "), group, subgroup})
	}
	return entries, scanner.Err()
}

// Unicode names become readable, ASCII shortcode aliases. Existing aliases,
// such as smile and thumbs_up, remain available.
func unicodeEmojiAlias(name string) string {
	var b strings.Builder
	space := false
	for _, r := range norm.NFD.String(strings.ToLower(name)) {
		if unicode.Is(unicode.Mn, r) || r == '\'' || r == '’' {
			continue
		}
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteRune(r)
			space = false
		} else {
			space = true
		}
	}
	return b.String()
}

func addUnicodeEmojiNames(names map[string]string) map[string]string {
	for _, entry := range emojiCatalog {
		alias := unicodeEmojiAlias(entry.Name)
		if _, exists := names[alias]; !exists {
			names[alias] = entry.Emoji
		}
	}
	return names
}
