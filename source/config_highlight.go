package main

import (
	"strings"
	"unicode"

	"gothoom/eui"
)

func highlightJSONSource(value string, colors eui.SyntaxColors) []eui.TextColorSpan {
	text := []rune(value)
	var spans []eui.TextColorSpan
	for i := 0; i < len(text); {
		start := i
		var color eui.Color
		switch {
		case text[i] == '"':
			i++
			for i < len(text) && text[i] != '\n' {
				r := text[i]
				i++
				if r == '\\' && i < len(text) && text[i] != '\n' {
					i++
					continue
				}
				if r == '"' {
					break
				}
			}
			color = colors.Strings
			next := i
			for next < len(text) && unicode.IsSpace(text[next]) {
				next++
			}
			if next < len(text) && text[next] == ':' {
				color = colors.Variables
			}
		case text[i] == '-' || text[i] >= '0' && text[i] <= '9':
			i++
			for i < len(text) && strings.ContainsRune("0123456789.eE+-", text[i]) {
				i++
			}
			color = colors.Numbers
		case unicode.IsLetter(text[i]):
			i++
			for i < len(text) && (unicode.IsLetter(text[i]) || unicode.IsDigit(text[i])) {
				i++
			}
			switch string(text[start:i]) {
			case "true", "false", "null":
				color = colors.Keywords
			default:
				continue
			}
		default:
			i++
			continue
		}
		spans = append(spans, eui.TextColorSpan{Start: start, End: i, Color: color})
	}
	return spans
}

func highlightTTSSubstitutions(value string, colors eui.SyntaxColors) []eui.TextColorSpan {
	var spans []eui.TextColorSpan
	offset := 0
	for _, line := range strings.Split(value, "\n") {
		text := []rune(line)
		start := 0
		for start < len(text) && unicode.IsSpace(text[start]) {
			start++
		}
		if start < len(text) && text[start] == '#' {
			spans = append(spans, eui.TextColorSpan{Start: offset + start, End: offset + len(text), Color: colors.Comments})
		} else {
			for i := start; i < len(text); i++ {
				if text[i] != '=' {
					continue
				}
				if i > start {
					spans = append(spans, eui.TextColorSpan{Start: offset + start, End: offset + i, Color: colors.Variables})
				}
				spans = append(spans, eui.TextColorSpan{Start: offset + i, End: offset + i + 1, Color: colors.Keywords})
				if i+1 < len(text) {
					spans = append(spans, eui.TextColorSpan{Start: offset + i + 1, End: offset + len(text), Color: colors.Strings})
				}
				break
			}
		}
		offset += len(text) + 1
	}
	return spans
}
