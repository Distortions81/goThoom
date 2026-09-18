package main

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// A score contains simultaneous performers, not consecutive song sections.
// Source line numbers let instrument edits preserve comments and notation.
type bardPart struct {
	Name           string
	Instrument     int
	Text           string
	startLine      int
	instrumentLine int
}

type bardScore struct {
	Title, Composer string
	Tags            []string
	Parts           []bardPart
}

func bardSelectedPart(score bardScore, name string) (int, error) {
	for i, part := range score.Parts {
		if name == "" || part.Name == name {
			return i, nil
		}
	}
	return 0, fmt.Errorf("The selected part has changed. Refresh the library and choose a part.")
}

func parseBardScore(value string, fallback int) (bardScore, error) {
	var score bardScore
	if !utf8.ValidString(value) || len(value) > bardMaxFileSize {
		return score, fmt.Errorf("Tunes must be UTF-8 text, up to 256 KiB.")
	}
	if fallback < 0 || fallback >= len(instruments) {
		return score, fmt.Errorf("Choose an instrument.")
	}
	part := bardPart{Name: "Solo", Instrument: fallback, startLine: -1, instrumentLine: -1}
	var notes strings.Builder
	depth := 0
	explicit := false
	seen := make(map[string]bool)
	names := make(map[string]bool)
	lines := strings.Split(normalizeSourceEditorText(value), "\n")
	for lineNumber, line := range lines {
		trimmed := strings.TrimSpace(line)
		if depth == 0 && strings.HasPrefix(trimmed, ";@") {
			key, data, ok := strings.Cut(strings.TrimPrefix(trimmed, ";@"), ":")
			key, data = strings.ToLower(strings.TrimSpace(key)), strings.TrimSpace(data)
			fail := func(message string) (bardScore, error) {
				return score, fmt.Errorf("Line %d: %s", lineNumber+1, message)
			}
			if !ok {
				return fail("Use ;@name: value for tune metadata.")
			}
			switch key {
			case "title", "composer", "tags":
				if explicit {
					return fail("Put song details before the first ;@part: line.")
				}
				if seen[key] {
					return fail("Duplicate " + key + " metadata.")
				}
				seen[key] = true
				switch key {
				case "title":
					score.Title = data
				case "composer":
					score.Composer = data
				case "tags":
					tags := make(map[string]bool)
					for _, tag := range strings.Split(data, ",") {
						tag = strings.TrimSpace(tag)
						key := strings.ToLower(tag)
						if tag != "" && !tags[key] {
							score.Tags = append(score.Tags, tag)
							tags[key] = true
						}
					}
				}
			case "part":
				if data == "" || names[strings.ToLower(data)] {
					return fail("Give each part a unique, nonempty name.")
				}
				if explicit {
					part.Text = notes.String()
					score.Parts = append(score.Parts, part)
				} else if strings.TrimSpace(stripComments(notes.String())) != "" {
					return fail("Put the first ;@part: line before the music.")
				}
				names[strings.ToLower(data)] = true
				explicit = true
				part = bardPart{Name: data, Instrument: fallback, startLine: lineNumber, instrumentLine: -1}
				notes.Reset()
			case "instrument":
				if part.instrumentLine >= 0 {
					return fail("Duplicate instrument for part " + part.Name + ".")
				}
				index := bardInstrumentIndex(data)
				if index < 0 {
					return fail(fmt.Sprintf("Unknown instrument %q for part %s.", data, part.Name))
				}
				part.Instrument, part.instrumentLine = index, lineNumber
				if !explicit {
					fallback = index
				}
			default:
				return fail(fmt.Sprintf("Unknown metadata %q. Use ; for ordinary comments.", key))
			}
			continue
		}
		// Semicolons inside legacy <comments> remain ordinary comment text.
		for i, c := range line {
			if c == ';' && depth == 0 {
				line = line[:i]
				break
			}
			if c == '<' {
				depth++
			} else if c == '>' {
				depth--
				if depth < 0 {
					return score, fmt.Errorf("Line %d: unmatched comment ending.", lineNumber+1)
				}
			}
		}
		notes.WriteString(line)
		if lineNumber+1 < len(lines) {
			notes.WriteByte('\n')
		}
	}
	if depth != 0 {
		return score, fmt.Errorf("Unclosed comment in part %s.", part.Name)
	}
	part.Text = notes.String()
	score.Parts = append(score.Parts, part)
	return score, nil
}

func validateBardScore(score bardScore) ([]musicPart, error) {
	parts := make([]musicPart, 0, len(score.Parts))
	for _, part := range score.Parts {
		notes, err := validateBardTune(part.Text, part.Instrument)
		if err != nil {
			return nil, fmt.Errorf("Part %s: %w", part.Name, err)
		}
		if _, err := bardTuneCommands(part.Text); err != nil {
			return nil, fmt.Errorf("Part %s: %w", part.Name, err)
		}
		parts = append(parts, musicPart{program: instruments[part.Instrument].program, notes: notes})
	}
	return parts, nil
}

func bardScoreInstrumentText(value string, fallback, selected, index int) (string, error) {
	if index < 0 || index >= len(instruments) {
		return "", fmt.Errorf("Choose an instrument.")
	}
	score, err := parseBardScore(value, fallback)
	if err != nil {
		return "", err
	}
	if selected < 0 || selected >= len(score.Parts) {
		return "", fmt.Errorf("Choose a part.")
	}
	part := score.Parts[selected]
	lines := strings.Split(normalizeSourceEditorText(value), "\n")
	metadata := ";@instrument: " + classicInstrumentNames[index]
	if part.instrumentLine >= 0 {
		lines[part.instrumentLine] = metadata
	} else {
		at := part.startLine + 1
		lines = append(lines[:at], append([]string{metadata}, lines[at:]...)...)
	}
	result := strings.Join(lines, "\n")
	if len(result) > bardMaxFileSize {
		return "", fmt.Errorf("Tunes must be UTF-8 text, up to 256 KiB.")
	}
	return result, nil
}
