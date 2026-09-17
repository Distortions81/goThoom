package main

import "gothoom/eui"

type sourceHighlightKind uint8

const (
	sourceHighlightPlain sourceHighlightKind = iota
	sourceHighlightComment
	sourceHighlightString
	sourceHighlightKeyword
	sourceHighlightNumber
	sourceHighlightVariable
	sourceHighlightBinding
)

func sourceHighlightPalette(background eui.Color) [7]eui.Color {
	if 299*int(background.R)+587*int(background.G)+114*int(background.B) >= 128000 {
		return [7]eui.Color{{},
			eui.NewColor(45, 105, 45, 255), eui.NewColor(145, 65, 20, 255),
			eui.NewColor(35, 70, 170, 255), eui.NewColor(115, 55, 145, 255),
			eui.NewColor(0, 100, 115, 255), eui.NewColor(140, 55, 105, 255),
		}
	}
	return [7]eui.Color{{},
		eui.NewColor(145, 200, 145, 255), eui.NewColor(235, 190, 130, 255),
		eui.NewColor(120, 190, 255, 255), eui.NewColor(205, 165, 240, 255),
		eui.NewColor(120, 215, 220, 255), eui.NewColor(235, 160, 200, 255),
	}
}
