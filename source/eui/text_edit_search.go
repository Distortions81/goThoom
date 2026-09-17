package eui

import (
	"slices"
	"unicode"
)

// FindText selects a case-insensitive literal match at or after start (a rune
// offset), wrapping at the document boundary. Backward searches include start
// and wrap from a negative offset to the end. It reveals the match without
// taking keyboard focus from a search field or changing the document.
func (item *itemData) FindText(query string, start int, backward bool) bool {
	if !itemHandlesTextEditing(item) || item.HideText || query == "" {
		return false
	}
	value, needle := []rune(item.editText()), []rune(query)
	for i := range value {
		value[i] = unicode.ToLower(value[i])
	}
	for i := range needle {
		needle[i] = unicode.ToLower(needle[i])
	}
	n := len(value)
	start = (start%(n+1) + n + 1) % (n + 1)
	for i := range n + 1 {
		pos := (start + i) % (n + 1)
		if backward {
			pos = (start - i + n + 1) % (n + 1)
		}
		if pos+len(needle) <= n && slices.Equal(value[pos:pos+len(needle)], needle) {
			item.editSelect(pos, pos+len(needle))
			return true
		}
	}
	return false
}
