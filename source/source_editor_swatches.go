package main

import (
	"fmt"
	"strconv"
	"unicode"

	"gothoom/eui"
)

// Only whole, unescaped JSON string values are color candidates. Keys and text
// containing a hex-looking fragment retain their ordinary text presentation.
func jsonColorSwatches(value string) []eui.TextColorSpan {
	runes := []rune(value)
	var spans []eui.TextColorSpan
	for i := 0; i < len(runes); i++ {
		if runes[i] != '"' {
			continue
		}
		start := i + 1
		i++
		for i < len(runes) && runes[i] != '"' && runes[i] != '\n' {
			if runes[i] == '\\' && i+1 < len(runes) {
				i++
			}
			i++
		}
		if i >= len(runes) || runes[i] != '"' || (i-start != 7 && i-start != 9) || runes[start] != '#' {
			continue
		}
		next := i + 1
		for next < len(runes) && unicode.IsSpace(runes[next]) {
			next++
		}
		if next < len(runes) && runes[next] == ':' {
			continue
		}
		color, err := strconv.ParseUint(string(runes[start+1:i]), 16, 32)
		if err != nil {
			continue
		}
		if i-start == 7 {
			color = color<<8 | 255
		}
		spans = append(spans, eui.TextColorSpan{Start: start, End: i, Color: eui.NewColor(uint8(color>>24), uint8(color>>16), uint8(color>>8), uint8(color))})
	}
	return spans
}

func (ed *sourceEditor) editColorSwatch(span eui.TextColorSpan) {
	before := ed.input.Text
	runes := []rune(before)
	if span.Start < 0 || span.End > len(runes) || span.Start >= span.End {
		return
	}
	value := string(runes[span.Start:span.End])
	openColorPicker("Edit Color — "+value, span.Color, func(color eui.Color) {
		ed.applyColorSwatch(before, span, color)
	})
}

func (ed *sourceEditor) applyColorSwatch(before string, span eui.TextColorSpan, color eui.Color) {
	if sourceEditors[ed.doc.path] != ed {
		return
	}
	if ed.input.Text != before {
		ed.setStatus("Color not changed: the draft changed while the picker was open. Select the color again.")
		return
	}
	if color == span.Color {
		return
	}
	runes := []rune(before)
	value := fmt.Sprintf("#%02x%02x%02x%02x", color.R, color.G, color.B, color.A)
	if span.End-span.Start == 7 && color.A == 255 {
		value = value[:7]
	}
	ed.input.ReplaceText(string(runes[:span.Start]) + value + string(runes[span.End:]))
	ed.input.CursorPos, ed.input.SelectStart, ed.input.SelectEnd = span.Start-1, span.Start-1, span.Start-1
	ed.focus()
	ed.setStatus("")
}
