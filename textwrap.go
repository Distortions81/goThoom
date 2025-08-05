package main

import (
	"strings"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

// wrapText splits s into lines that do not exceed maxWidth when rendered
// with the provided face. Words are kept intact when possible; if a single
// word exceeds maxWidth it will be broken across lines.
func wrapText(s string, face text.Face, maxWidth float64) (int, []string) {

	var lines []string
	for _, para := range strings.Split(s, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		cur := words[0]
		for _, w := range words[1:] {
			cand := cur + " " + w
			width, _ := text.Measure(cand, face, 0)
			if width <= maxWidth {
				cur = cand
				continue
			}

			lines = append(lines, cur)
			// if the single word is too wide, break it into pieces
			if ww, _ := text.Measure(w, face, 0); ww > maxWidth {
				var runes []rune
				for _, r := range w {
					runes = append(runes, r)
					if wpart, _ := text.Measure(string(runes), face, 0); wpart > maxWidth && len(runes) > 1 {
						lines = append(lines, string(runes[:len(runes)-1]))
						runes = runes[len(runes)-1:]
					}
				}
				cur = string(runes)
			} else {
				cur = w
			}
		}
		lines = append(lines, cur)
	}
	return int(maxWidth), lines
}
