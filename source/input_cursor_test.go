package main

import "testing"

func TestWordBoundaries(t *testing.T) {
	text := []rune("one  two three")
	for _, test := range []struct {
		name   string
		cursor int
		left   int
		right  int
	}{
		{name: "start", cursor: 0, left: 0, right: 5},
		{name: "inside first", cursor: 2, left: 0, right: 5},
		{name: "between words", cursor: 4, left: 0, right: 5},
		{name: "start second", cursor: 5, left: 0, right: 9},
		{name: "inside second", cursor: 7, left: 5, right: 9},
		{name: "end", cursor: len(text), left: 9, right: len(text)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := previousWordBoundary(text, test.cursor); got != test.left {
				t.Errorf("previous boundary = %d, want %d", got, test.left)
			}
			if got := nextWordBoundary(text, test.cursor); got != test.right {
				t.Errorf("next boundary = %d, want %d", got, test.right)
			}
		})
	}
}

func TestPreviousWordBoundaryHandlesUnicodeWhitespace(t *testing.T) {
	text := []rune("hello\t世界")
	if got := previousWordBoundary(text, len(text)); got != 6 {
		t.Fatalf("previous boundary = %d, want 6", got)
	}
	if got := previousWordBoundary(text, 6); got != 0 {
		t.Fatalf("boundary before whitespace = %d, want 0", got)
	}
}
