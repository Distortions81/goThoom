package main

import (
	"go/format"
	"go/scanner"
	"go/token"
	"unicode/utf8"
)

func formatGoScript(value string) (string, error) {
	formatted, err := format.Source([]byte(value))
	return string(formatted), err
}

type goFormatToken struct {
	kind       token.Token
	literal    string
	start, end int
}

func goFormatTokens(value string) []goFormatToken {
	file := token.NewFileSet().AddFile("", -1, len(value))
	var scan scanner.Scanner
	scan.Init(file, []byte(value), nil, scanner.ScanComments)
	var tokens []goFormatToken
	byteOffset, runeOffset := 0, 0
	for {
		pos, kind, literal := scan.Scan()
		if kind == token.EOF {
			return tokens
		}
		// gofmt removes optional semicolons; ignore the scanner's inserted ones
		// too, including those positioned inside multiline comments.
		if kind == token.SEMICOLON {
			continue
		}
		start := file.Offset(pos)
		runeOffset += utf8.RuneCountInString(value[byteOffset:start])
		if literal == "" {
			literal = kind.String()
		}
		tokens = append(tokens, goFormatToken{kind, literal, runeOffset, runeOffset + utf8.RuneCountInString(literal)})
		byteOffset = start
	}
}

// Follow the same occurrence of a token through spacing changes, new lines,
// and sorted imports. Positions in whitespace attach to the following token.
// At a token boundary, trailing keeps the position on the preceding token.
// If gofmt removes an anchor (for example a duplicate import), use the next
// surviving token, or the end of the document.
func remapGoFormattedPosition(before, after string, position int, trailing bool) int {
	position = max(0, min(position, utf8.RuneCountInString(before)))
	if position == 0 {
		return 0
	}
	type key struct {
		kind    token.Token
		literal string
	}
	byToken := map[key][]goFormatToken{}
	for _, tok := range goFormatTokens(after) {
		k := key{tok.kind, tok.literal}
		byToken[k] = append(byToken[k], tok)
	}
	occurrences := map[key]int{}
	for _, tok := range goFormatTokens(before) {
		k := key{tok.kind, tok.literal}
		index := occurrences[k]
		occurrences[k]++
		if position > tok.end || position == tok.end && !trailing || index >= len(byToken[k]) {
			continue
		}
		next := byToken[k][index]
		return next.start + max(0, min(position-tok.start, next.end-next.start))
	}
	return utf8.RuneCountInString(after)
}
