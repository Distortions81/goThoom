package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"gothoom/eui"
)

const bardMaxFileSize = 256 * 1024

type bardTune struct {
	Path, Name string
	Instrument int
	Score      bardScore
	Err        error
}

func bardTunesDir() string { return filepath.Join(dataDirPath, "Tunes") }

func listBardTunes() ([]bardTune, error) {
	if err := os.MkdirAll(bardTunesDir(), 0755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(bardTunesDir())
	if err != nil {
		return nil, err
	}
	var tunes []bardTune
	for _, entry := range entries {
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if !entry.Type().IsRegular() || (ext != ".tune" && ext != ".txt") {
			continue
		}
		tune := bardTune{Path: filepath.Join(bardTunesDir(), entry.Name()), Name: strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())), Instrument: defaultInstrument}
		tune.Instrument = bardLegacyInstrument(tune.Path)
		value, readErr := readBardTune(tune.Path)
		tune.Err = readErr
		if readErr == nil {
			tune.Score, tune.Err = parseBardScore(value, tune.Instrument)
			if tune.Score.Title != "" {
				tune.Name = tune.Score.Title
			}
			if len(tune.Score.Parts) > 0 {
				tune.Instrument = tune.Score.Parts[0].Instrument
			}
		}
		tunes = append(tunes, tune)
	}
	sort.Slice(tunes, func(i, j int) bool { return strings.ToLower(tunes[i].Name) < strings.ToLower(tunes[j].Name) })
	return tunes, nil
}
func bardSavedInstrument(path string) int {
	index := bardLegacyInstrument(path)
	if value, err := readBardTune(path); err == nil {
		if score, err := parseBardScore(value, index); err == nil {
			return score.Parts[0].Instrument
		}
	}
	return index
}
func bardLegacyInstrument(path string) int {
	data, err := os.ReadFile(path + ".json")
	if err == nil {
		var meta struct{ Instrument int }
		if json.Unmarshal(data, &meta) == nil && meta.Instrument >= 0 && meta.Instrument < len(instruments) {
			return meta.Instrument
		}
	}
	return defaultInstrument
}
func saveBardInstrument(tune bardTune, index int) error {
	return saveBardPartInstrument(tune, 0, index)
}
func saveBardPartInstrument(tune bardTune, part, index int) error {
	doc, err := loadSourceDocument(tune.Path, false)
	if err != nil {
		return err
	}
	ed := sourceEditors[doc.path]
	if ed != nil && ed.dirty() {
		return fmt.Errorf("Save or close this tune's draft before changing its instrument.")
	}
	if part >= 0 && part < len(tune.Score.Parts) {
		current, err := parseBardScore(doc.savedText, bardLegacyInstrument(tune.Path))
		if err != nil {
			return err
		}
		part, err = bardSelectedPart(current, tune.Score.Parts[part].Name)
		if err != nil {
			return err
		}
	}
	value, err := bardScoreInstrumentText(doc.savedText, bardLegacyInstrument(tune.Path), part, index)
	if err != nil {
		return err
	}
	// A clean open editor must still detect an external edit before saving.
	if ed != nil {
		doc = ed.doc
	}
	if err := doc.save(value); err != nil {
		return err
	}
	if ed != nil {
		ed.input.ReplaceText(value)
		ed.setStatus("")
	}
	return nil
}
func createBardTune(name, value string) (bardTune, error) {
	name = strings.TrimSpace(name)
	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".tune" && ext != ".txt" {
		name += ".tune"
	}
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	if stem == "" || strings.HasPrefix(name, ".") || len(name) > 180 || strings.HasSuffix(stem, ".") {
		return bardTune{}, fmt.Errorf("Enter a tune name without leading or trailing dots.")
	}
	for _, r := range name {
		if unicode.IsControl(r) || strings.ContainsRune(`/\:*?"<>|`, r) {
			return bardTune{}, fmt.Errorf("Use a tune name without slashes or special characters.")
		}
	}
	reserved := strings.ToUpper(strings.Split(stem, ".")[0])
	if reserved == "CON" || reserved == "PRN" || reserved == "AUX" || reserved == "NUL" || (len(reserved) == 4 && (strings.HasPrefix(reserved, "COM") || strings.HasPrefix(reserved, "LPT")) && reserved[3] >= '1' && reserved[3] <= '9') {
		return bardTune{}, fmt.Errorf("Choose a different tune name.")
	}
	if !utf8.ValidString(value) || len(value) > bardMaxFileSize {
		return bardTune{}, fmt.Errorf("Tunes must be UTF-8 text, up to 256 KiB.")
	}
	if err := os.MkdirAll(bardTunesDir(), 0755); err != nil {
		return bardTune{}, err
	}
	// Match filenames case-insensitively so a library remains portable.
	entries, err := os.ReadDir(bardTunesDir())
	if err != nil {
		return bardTune{}, err
	}
	for _, e := range entries {
		if strings.EqualFold(e.Name(), name) {
			return bardTune{}, fmt.Errorf("A tune named %s already exists.", name)
		}
	}
	path := filepath.Join(bardTunesDir(), name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return bardTune{}, err
	}
	_, err = f.WriteString(value)
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(path)
		return bardTune{}, err
	}
	return bardTune{Path: path, Name: stem}, nil
}
func readBardTune(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() > bardMaxFileSize {
		return "", fmt.Errorf("Choose a text tune file up to 256 KiB.")
	}
	doc, err := loadSourceDocument(path, false)
	if err != nil {
		return "", err
	}
	return doc.savedText, nil
}

// Tokens remain intact across commands: the receiving client may insert a
// space between parts. Comments are local, so Unicode prose never enters a
// legacy server command. Whitespace between musical tokens is preserved.
func bardTuneTokens(value string) ([]string, error) {
	score, err := parseBardScore(value, defaultInstrument)
	if err != nil {
		return nil, err
	}
	if len(score.Parts) != 1 {
		return nil, fmt.Errorf("Choose one part to perform in-game.")
	}
	return bardNoteTokens(score.Parts[0].Text)
}

func bardNoteTokens(value string) ([]string, error) {
	if !utf8.ValidString(value) || len(value) > bardMaxFileSize {
		return nil, fmt.Errorf("Tunes must be UTF-8 text, up to 256 KiB.")
	}
	// Strip comments before tokenizing, including comments inside a token.
	depth := 0
	for _, c := range value {
		if c == '<' {
			depth++
		}
		if c == '>' {
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("Unmatched comment ending.")
			}
		}
	}
	if depth != 0 {
		return nil, fmt.Errorf("Unclosed comment.")
	}
	value = stripComments(value)
	var tokens []string
	var stack []byte
	costs := []int{0}
	for i := 0; i < len(value); {
		start := i
		c := value[i]
		if strings.ContainsRune(" \t\r\n", rune(c)) {
			tokens = append(tokens, " ")
			costs[len(costs)-1]++
			if costs[len(costs)-1] > 100000 {
				return nil, fmt.Errorf("The expanded tune is too large.")
			}
			i++
			continue
		}
		i++
		switch {
		case isNoteLetter(c):
			for i < len(value) && strings.ContainsRune("#._", rune(value[i])) {
				i++
			}
			if i < len(value) && value[i] >= '1' && value[i] <= '9' {
				i++
			}
		case c == 'p' || c == '%' || c == '{' || c == '}':
			if i < len(value) && value[i] >= '1' && value[i] <= '9' {
				i++
			}
		case c == '@':
			if i < len(value) && strings.ContainsRune("+-=", rune(value[i])) {
				i++
			}
			digits := i
			for i < len(value) && value[i] >= '0' && value[i] <= '9' {
				i++
			}
			if i-digits > 3 {
				return nil, fmt.Errorf("Tempo is too large at byte %d.", start+1)
			}
		case c == '(' || c == '[':
			if len(stack) > 8 {
				return nil, fmt.Errorf("Too many nested loops or chords.")
			}
			if len(stack) > 0 && stack[len(stack)-1] == '[' {
				return nil, fmt.Errorf("A chord cannot contain a loop or another chord.")
			}
			stack = append(stack, c)
			costs = append(costs, 0)
		case c == ')' || c == ']':
			want := byte('(')
			if c == ']' {
				want = '['
			}
			if len(stack) == 0 || stack[len(stack)-1] != want {
				return nil, fmt.Errorf("Unmatched %c at byte %d.", c, start+1)
			}
			stack = stack[:len(stack)-1]
			count := 1
			if i < len(value) && value[i] >= '1' && value[i] <= '9' {
				if c == ')' {
					count = int(value[i] - '0')
				}
				i++
			}
			if c == ']' && i < len(value) && value[i] == '$' {
				i++
			}
			cost := costs[len(costs)-1] * count
			costs = costs[:len(costs)-1]
			costs[len(costs)-1] += cost
		case c == '|' || c == '!':
			if len(stack) == 0 || stack[len(stack)-1] != '(' {
				return nil, fmt.Errorf("Loop ending outside a loop at byte %d.", start+1)
			}
			if c == '|' {
				if i >= len(value) || value[i] < '1' || value[i] > '9' {
					return nil, fmt.Errorf("Use |1 through |9 for a loop ending.")
				}
				i++
			}
		case strings.ContainsRune("+-=/\\", rune(c)):
		default:
			return nil, fmt.Errorf("Unexpected character at byte %d. Use CL tune notation; put text inside <comments>.", start+1)
		}
		costs[len(costs)-1] += i - start
		if costs[len(costs)-1] > 100000 {
			return nil, fmt.Errorf("The expanded tune is too large.")
		}
		tokens = append(tokens, value[start:i])
	}
	if len(stack) > 0 {
		return nil, fmt.Errorf("Unclosed loop or chord.")
	}
	return tokens, nil
}
func validateBardTune(value string, inst int) ([]Note, error) {
	score, err := parseBardScore(value, inst)
	if err != nil {
		return nil, err
	}
	if len(score.Parts) != 1 {
		return nil, fmt.Errorf("Choose one part to perform in-game.")
	}
	inst = score.Parts[0].Instrument
	tokens, err := bardNoteTokens(score.Parts[0].Text)
	if err != nil {
		return nil, err
	}
	if inst < 0 || inst >= len(instruments) {
		return nil, fmt.Errorf("Choose an instrument.")
	}
	notes, parseErr := parseClassicTune(strings.Join(tokens, ""), instruments[inst], 120, 100)
	if parseErr != nil {
		return nil, parseErr
	}
	if len(notes) == 0 {
		return nil, fmt.Errorf("This tune has no playable notes for %s.", classicInstrumentNames[inst])
	}
	return notes, nil
}
func bardTuneCommands(value string) ([]string, error) {
	return bardEnsembleCommands(value, nil)
}
func bardEnsembleCommands(value string, partners []string) ([]string, error) {
	tokens, err := bardTuneTokens(value)
	if err != nil {
		return nil, err
	}
	with, err := bardWithOptions(partners)
	if err != nil {
		return nil, err
	}
	// Reserve room for every /with option even on buffered /part commands.
	limit := 511 - len("/use /part ") - len(with)
	var parts []string
	part := ""
	for _, token := range tokens {
		if len(token) > limit {
			return nil, fmt.Errorf("A tune token is too long for a game command.")
		}
		if len(part)+len(token) > limit {
			if strings.TrimSpace(part) != "" {
				parts = append(parts, strings.TrimSpace(part))
			}
			part = ""
		}
		part += token
	}
	if strings.TrimSpace(part) != "" {
		parts = append(parts, strings.TrimSpace(part))
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("Enter a tune first.")
	}
	if len(parts) > 5 {
		return nil, fmt.Errorf("This music needs %d command segments; Clan Lord supports up to five. Shorten it with loops.", len(parts))
	}
	for i := range parts {
		prefix := "/use /part "
		if i == len(parts)-1 {
			prefix = "/use "
		}
		parts[i] = prefix + with + parts[i]
	}
	return parts, nil
}
func highlightBardTune(value string, colors eui.SyntaxColors) []eui.TextColorSpan {
	text := []rune(value)
	var spans []eui.TextColorSpan
	for i := 0; i < len(text); i++ {
		start := i
		c := text[i]
		color := colors.Keywords
		switch {
		case c == ';':
			for i+1 < len(text) && text[i+1] != '\n' && text[i+1] != '\r' {
				i++
			}
			color = colors.Comments
		case c == '<':
			depth := 1
			for i+1 < len(text) && depth > 0 {
				i++
				if text[i] == '<' {
					depth++
				}
				if text[i] == '>' {
					depth--
				}
			}
			color = colors.Comments
		case c >= '0' && c <= '9':
			color = colors.Numbers
		case c == 'p':
			color = colors.Variables
		case c < 128 && isNoteLetter(byte(c)):
			color = colors.Strings
		case unicode.IsSpace(c):
			continue
		}
		spans = append(spans, eui.TextColorSpan{Start: start, End: i + 1, Color: color})
	}
	return spans
}
