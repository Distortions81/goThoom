package main

import (
	"go/scanner"
	"go/token"
	"strings"
	"unicode/utf8"

	"gothoom/eui"
)

// Scan the draft without loading imports or executing any script code. Scanner
// recovery keeps highlighting available while a statement or literal is unfinished.
func highlightGoScript(value string, background eui.Color) []eui.TextColorSpan {
	file := token.NewFileSet().AddFile("", -1, len(value))
	var scan scanner.Scanner
	scan.Init(file, []byte(value), nil, scanner.ScanComments)
	palette := sourceHighlightPalette(background)
	var spans []eui.TextColorSpan
	byteOffset, runeOffset := 0, 0
	for {
		pos, tok, literal := scan.Scan()
		if tok == token.EOF {
			return spans
		}
		kind := sourceHighlightPlain
		switch {
		case tok == token.COMMENT:
			kind = sourceHighlightComment
		case tok == token.STRING || tok == token.CHAR:
			kind = sourceHighlightString
		case tok.IsKeyword():
			kind = sourceHighlightKeyword
		case tok == token.INT || tok == token.FLOAT || tok == token.IMAG:
			kind = sourceHighlightNumber
		}
		if kind == sourceHighlightPlain {
			continue
		}
		start := file.Offset(pos)
		end := start + len(literal)
		// Go's scanner removes carriage returns from comments and raw strings.
		// Recover their original extent so CRLF and Unicode keep exact positions.
		if tok == token.COMMENT || tok == token.STRING && value[start] == '`' {
			end = len(value)
			switch {
			case strings.HasPrefix(value[start:], "//"):
				if n := strings.IndexByte(value[start:], '\n'); n >= 0 {
					end = start + n
				}
			case tok == token.COMMENT:
				if n := strings.Index(value[start+2:], "*/"); n >= 0 {
					end = start + 2 + n + 2
				}
			default:
				if n := strings.IndexByte(value[start+1:], '`'); n >= 0 {
					end = start + 1 + n + 1
				}
			}
		}
		// Only advance through the source once when converting byte positions to
		// the rune positions used by the editor (including tabs and newlines).
		runeOffset += utf8.RuneCountInString(value[byteOffset:start])
		length := utf8.RuneCountInString(value[start:end])
		spans = append(spans, eui.TextColorSpan{Start: runeOffset, End: runeOffset + length, Color: palette[kind]})
		byteOffset, runeOffset = end, runeOffset+length
	}
}
