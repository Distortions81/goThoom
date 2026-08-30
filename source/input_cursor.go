package main

import "unicode"

// wrappedCursorPos converts a plain text cursor position to the equivalent
// index in a wrapped string that may contain newline characters.
func wrappedCursorPos(text string, plain int) int {
	rs := []rune(text)
	plainCount := 0
	for i, r := range rs {
		if r == '\n' {
			continue
		}
		if plainCount == plain {
			return i
		}
		plainCount++
	}
	return len(rs)
}

// plainCursorPos converts a cursor index in a wrapped string back to the
// position in the underlying plain text (with newlines removed).
func plainCursorPos(text string, wrapped int) int {
	rs := []rune(text)
	if wrapped < 0 {
		wrapped = 0
	}
	if wrapped > len(rs) {
		wrapped = len(rs)
	}
	plain := 0
	for i := 0; i < wrapped; i++ {
		if rs[i] != '\n' {
			plain++
		}
	}
	return plain
}

func previousWordBoundary(text []rune, cursor int) int {
	cursor = max(0, min(cursor, len(text)))
	for cursor > 0 && unicode.IsSpace(text[cursor-1]) {
		cursor--
	}
	for cursor > 0 && !unicode.IsSpace(text[cursor-1]) {
		cursor--
	}
	return cursor
}

func nextWordBoundary(text []rune, cursor int) int {
	cursor = max(0, min(cursor, len(text)))
	if cursor < len(text) && unicode.IsSpace(text[cursor]) {
		for cursor < len(text) && unicode.IsSpace(text[cursor]) {
			cursor++
		}
		return cursor
	}
	for cursor < len(text) && !unicode.IsSpace(text[cursor]) {
		cursor++
	}
	for cursor < len(text) && unicode.IsSpace(text[cursor]) {
		cursor++
	}
	return cursor
}
