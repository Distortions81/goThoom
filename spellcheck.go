package main

//go:generate build-scripts/download_spellcheck_dict.sh

import (
	"bytes"
	_ "embed"
	"strings"
	"unicode"

	"github.com/f1monkey/spellchecker"

	"gothoom/eui"
)

//go:embed spellcheck_words.txt
var embeddedDict []byte

var sc *spellchecker.Spellchecker

// commonWords provides a tiny built-in dictionary so the checker works out of
// the box without large data files. A more complete word list can be added
// later by placing a file at spellcheck_words.txt.
var commonWords = []string{
	"the", "be", "to", "of", "and", "a", "in", "that", "have", "i", "it", "for", "not", "on", "with", "he",
	"as", "you", "do", "at", "this", "but", "his", "by", "from", "they", "we", "say", "her", "she",
	"or", "an", "will", "my", "one", "all", "would", "there", "their",
	// Clan Lord terms and notable NPCs
	"puddleby", "thoom", "sylvan", "halfling", "dwarf", "fen",
	"healer", "fighter", "mystic", "ranger", "champion", "bloodmage", "bard",
	"clan", "clans", "clanning", "exile", "exiles",
	"moonstone", "sunstone", "fellblade", "greataxe", "gossamer", "kudzu",
	"darshak", "orga", "wendecka", "noth", "undine", "arachne", "lyfelidae", "yorilla", "t'rool", "scarmis",
	"lok'groton", "anura", "meshra",
	"warlock", "berserker", "slasher", "hatchet", "shaman", "scout",
	"greymyr", "worg", "wraith", "drake", "wyrm", "panther", "cougar", "ferret",
	"rat", "ratling", "vermine", "noid", "noids", "goblin", "ogre", "orc",
	"feral", "tenebrion", "melabrion", "qual", "kizmia",
}

func init() {
	var err error
	sc, err = spellchecker.New("abcdefghijklmnopqrstuvwxyz'", spellchecker.WithMaxErrors(1))
	if err != nil {
		sc = nil
		return
	}
	if len(embeddedDict) > 0 {
		if err := sc.AddFrom(bytes.NewReader(embeddedDict)); err != nil {
			// ignore errors reading embedded dictionary
			sc.Add(commonWords...)
		}
	} else {
		// Fallback to a minimal set of common words.
		sc.Add(commonWords...)
	}
}

func findMisspellings(s string) []eui.TextSpan {
	if sc == nil {
		return nil
	}
	rs := []rune(s)
	spans := []eui.TextSpan{}
	start := -1
	for i, r := range rs {
		if unicode.IsLetter(r) || r == '\'' {
			if start == -1 {
				start = i
			}
			continue
		}
		if start != -1 {
			word := strings.ToLower(string(rs[start:i]))
			if !sc.IsCorrect(word) {
				spans = append(spans, eui.TextSpan{Start: start, End: i})
			}
			start = -1
		}
	}
	if start != -1 {
		word := strings.ToLower(string(rs[start:]))
		if !sc.IsCorrect(word) {
			spans = append(spans, eui.TextSpan{Start: start, End: len(rs)})
		}
	}
	return spans
}
func suggestCorrections(word string, n int) []string {
	if sc == nil {
		return nil
	}
	suggestions, err := sc.Suggest(word, n)
	if err != nil {
		return nil
	}
	return suggestions
}
