package main

import (
	"math"
	"strings"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

// measureWidth safely measures the width of s using face. If the face lacks a
// backing font source (e.g., *text.GoTextFace with nil Source), fall back to a
// simple approximation that avoids panics during tests.
func measureWidth(s string, face text.Face) float64 {
	if gf, ok := face.(*text.GoTextFace); ok {
		if gf.Source == nil {
			// Approximate average advance as ~0.6x size per rune.
			// This is only used in tests that don't verify exact widths.
			return float64(len(s)) * (gf.Size * 0.6)
		}
	}
	w, _ := text.Measure(s, face, 0)
	return w
}

// wrapText splits s into lines that do not exceed maxWidth when rendered
// with the provided face. Words are kept intact when possible; if a single
// word exceeds maxWidth it will be broken across lines. Unlike strings.Fields,
// it preserves runs of spaces so user input doesn't lose spacing.
func wrapText(s string, face text.Face, maxWidth float64) (int, []string) {
	var (
		lines   []string
		maxUsed float64
	)
	for _, para := range strings.Split(s, "\n") {
		tokens := strings.SplitAfter(para, " ")
		var builder strings.Builder
		curWidth := 0.0
		for _, tok := range tokens {
			if tok == "" {
				continue
			}
			w := measureWidth(tok, face)
			if curWidth+w <= maxWidth {
				builder.WriteString(tok)
				curWidth += w
				continue
			}
			if builder.Len() > 0 {
				if curWidth > maxUsed {
					maxUsed = curWidth
				}
				lines = append(lines, builder.String())
				builder.Reset()
				curWidth = 0
			}
			if w <= maxWidth {
				builder.WriteString(tok)
				curWidth = w
				continue
			}
			for _, r := range tok {
				rw := measureWidth(string(r), face)
				if curWidth+rw > maxWidth && builder.Len() > 0 {
					if curWidth > maxUsed {
						maxUsed = curWidth
					}
					lines = append(lines, builder.String())
					builder.Reset()
					curWidth = 0
				}
				builder.WriteRune(r)
				curWidth += rw
			}
		}
		if curWidth > maxUsed {
			maxUsed = curWidth
		}
		lines = append(lines, builder.String())
	}
	return int(math.Ceil(maxUsed)), lines
}
