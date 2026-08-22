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
var spellCache map[string]bool
var spellDirty bool

// commonWords provides a tiny built-in dictionary so the checker works out of
// the box without large data files. A more complete word list can be added
// later by placing a file at spellcheck_words.txt.
var commonWords = []string{
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

func loadSpellcheck() {
	var err error
	sc, err = spellchecker.New("abcdefghijklmnopqrstuvwxyz'", spellchecker.WithMaxErrors(1))
	if err != nil {
		sc = nil
		return
	}
	spellCache = make(map[string]bool)
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
			if correct, ok := spellCache[word]; ok {
				if !correct {
					spans = append(spans, eui.TextSpan{Start: start, End: i})
				}
			} else {
				correct := sc.IsCorrect(word)
				spellCache[word] = correct
				if !correct {
					spans = append(spans, eui.TextSpan{Start: start, End: i})
				}
			}
			start = -1
		}
	}
	if start != -1 {
		word := strings.ToLower(string(rs[start:]))
		if correct, ok := spellCache[word]; ok {
			if !correct {
				spans = append(spans, eui.TextSpan{Start: start, End: len(rs)})
			}
		} else {
			correct := sc.IsCorrect(word)
			spellCache[word] = correct
			if !correct {
				spans = append(spans, eui.TextSpan{Start: start, End: len(rs)})
			}
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
