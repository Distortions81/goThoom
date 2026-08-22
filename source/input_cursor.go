package main

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
