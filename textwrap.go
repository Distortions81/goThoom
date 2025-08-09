package main

import (
	"math"
	"strings"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

// wrapText splits s into lines that do not exceed maxWidth when rendered
// with the provided face. Words are kept intact when possible; if a single
// word exceeds maxWidth it will be broken across lines.
func wrapText(s string, face text.Face, maxWidth float64) (int, []string) {
	var (
		lines         []string
		maxUsed       float64
		runesBuffer   []rune
		spaceWidth, _ = text.Measure(" ", face, 0)
	)
	for _, para := range strings.Split(s, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		wordWidths := make([]float64, len(words))
		for i, w := range words {
			ww, _ := text.Measure(w, face, 0)
			wordWidths[i] = ww
		}

		var builder strings.Builder
		builder.WriteString(words[0])
		curWidth := wordWidths[0]

		for i := 1; i < len(words); i++ {
			w := words[i]
			wWidth := wordWidths[i]
			candWidth := curWidth + spaceWidth + wWidth
			if candWidth <= maxWidth {
				builder.WriteByte(' ')
				builder.WriteString(w)
				curWidth = candWidth
				continue
			}

			if curWidth > maxUsed {
				maxUsed = curWidth
			}
			lines = append(lines, builder.String())

			if wWidth > maxWidth {
				runesBuffer = runesBuffer[:0]
				partWidth := 0.0
				for _, r := range w {
					rw, _ := text.Measure(string(r), face, 0)
					if partWidth+rw > maxWidth && len(runesBuffer) > 0 {
						part := string(runesBuffer)
						if partWidth > maxUsed {
							maxUsed = partWidth
						}
						lines = append(lines, part)
						runesBuffer = runesBuffer[:0]
						partWidth = 0
					}
					runesBuffer = append(runesBuffer, r)
					partWidth += rw
				}
				builder.Reset()
				builder.WriteString(string(runesBuffer))
				curWidth = partWidth
			} else {
				builder.Reset()
				builder.WriteString(w)
				curWidth = wWidth
			}
		}
		if curWidth > maxUsed {
			maxUsed = curWidth
		}
		lines = append(lines, builder.String())
	}
	return int(math.Ceil(maxUsed)), lines
}
