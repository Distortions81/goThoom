package main

import (
	"strings"

	"gothoom/eui"
)

type macroHighlightKind uint8

const (
	macroHighlightPlain macroHighlightKind = iota
	macroHighlightComment
	macroHighlightString
	macroHighlightKeyword
	macroHighlightNumber
	macroHighlightVariable
	macroHighlightBinding
)

func macroHighlightPalette(background eui.Color) [7]eui.Color {
	if 299*int(background.R)+587*int(background.G)+114*int(background.B) >= 128000 {
		return [7]eui.Color{{},
			eui.NewColor(45, 105, 45, 255), eui.NewColor(145, 65, 20, 255),
			eui.NewColor(35, 70, 170, 255), eui.NewColor(115, 55, 145, 255),
			eui.NewColor(0, 100, 115, 255), eui.NewColor(140, 55, 105, 255),
		}
	}
	return [7]eui.Color{{},
		eui.NewColor(145, 200, 145, 255), eui.NewColor(235, 190, 130, 255),
		eui.NewColor(120, 190, 255, 255), eui.NewColor(205, 165, 240, 255),
		eui.NewColor(120, 215, 220, 255), eui.NewColor(235, 160, 200, 255),
	}
}

// Tokenize only the draft: highlighting never reads includes or executes macros.
// The parser's byte-offset mapping preserves block comments inside tokens and
// quotes, including the legacy rule that block comments take precedence there.
func highlightLegacyMacro(value string, background eui.Color) []eui.TextColorSpan {
	kinds := make([]macroHighlightKind, len(value))
	var comments []legacyMacroComment
	lines, diagnostics := legacyMacroSourceLinesWithComments(legacyMacroSource{Text: value}, &comments)
	for _, comment := range comments {
		for i := comment.Start; i < comment.End; i++ {
			kinds[i] = macroHighlightComment
		}
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Message != "unterminated block comment" {
			continue
		}
		// Locations use physical lines and byte columns, including CRLF files.
		line, start := 1, 0
		for start < len(value) && line < diagnostic.Location.Line {
			if value[start] == '\r' {
				line++
				if start+1 < len(value) && value[start+1] == '\n' {
					start++
				}
			} else if value[start] == '\n' {
				line++
			}
			start++
		}
		start += diagnostic.Location.Column - 1
		for i := start; i < len(value); i++ {
			kinds[i] = macroHighlightComment
		}
	}
	for _, line := range lines {
		tokens, diagnostic := tokenizeLegacyMacroLine(line)
		if diagnostic != nil {
			// Keep completed tokens and color the unfinished quote to line end.
			start := diagnostic.Location.Column - 1
			prefix := line
			prefix.Text = line.Text[:start]
			tokens, _ = tokenizeLegacyMacroLine(prefix)
			tokens = append(tokens, legacyMacroToken{Quote: line.Text[start], Column: start + 1, EndColumn: len(line.Text) + 1})
		}
		paint := func(start, end int, kind macroHighlightKind) {
			for i := start; i < end; i++ {
				kinds[line.Offsets[i]] = kind
			}
		}
		tail := 0
		for index, token := range tokens {
			kind := legacyMacroTokenHighlight(token, index)
			if kind != macroHighlightPlain {
				paint(token.Column-1, token.EndColumn-1, kind)
			}
			tail = token.EndColumn - 1
		}
		if comment := strings.Index(line.Text[tail:], "//"); comment >= 0 {
			paint(tail+comment, len(line.Text), macroHighlightComment)
		}
	}
	palette := macroHighlightPalette(background)
	var spans []eui.TextColorSpan
	position := 0
	for offset := range value {
		kind := kinds[offset]
		if kind != macroHighlightPlain {
			color := palette[kind]
			if n := len(spans); n > 0 && spans[n-1].End == position && spans[n-1].Color == color {
				spans[n-1].End++
			} else {
				spans = append(spans, eui.TextColorSpan{Start: position, End: position + 1, Color: color})
			}
		}
		position++
	}
	return spans
}

func legacyMacroTokenHighlight(token legacyMacroToken, index int) macroHighlightKind {
	if token.Quote != 0 {
		return macroHighlightString
	}
	if strings.HasPrefix(token.Text, "@") {
		return macroHighlightVariable
	}
	if legacyMacroAttribute(token) != 0 {
		return macroHighlightKeyword
	}
	switch strings.ToLower(token.Text) {
	case "include", "set", "setglobal", "if", "else", "end", "random", "or", "no-repeat",
		"label", "goto", "call", "pause", "message", "msg", "move", "equip", "unequip",
		"{", "}", "+", "-", "*", "/", "%", "==", "!=", "<", ">", "<=", ">=":
		return macroHighlightKeyword
	case "true", "false":
		return macroHighlightNumber
	}
	digits := token.Text
	if strings.HasPrefix(digits, "+") || strings.HasPrefix(digits, "-") {
		digits = digits[1:]
	}
	if digits != "" && strings.Trim(digits, "0123456789") == "" {
		return macroHighlightNumber
	}
	if index == 0 {
		if _, _, ok := parseLegacyMacroKeyBinding(token.Text); ok {
			return macroHighlightBinding
		}
	}
	return macroHighlightPlain
}
